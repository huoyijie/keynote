package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	cp "github.com/otiai10/copy"
	"gopkg.in/yaml.v3"
)

type Site struct {
	Logo, Link, Icon,
	Name, Title, Author,
	Desc, Summry, Copyright,
	Beian, BeianLink string
	StaticPath, StaticFile []string
}

func loadSite(conf string) (site *Site) {
	data, err := os.ReadFile(conf)
	fatalErr(err)

	site = &Site{}

	fatalErr(yaml.Unmarshal(data, site))
	return
}

type FileKind string

func (kind FileKind) IsGitbook() bool {
	return kind == GITBOOK
}

func (kind FileKind) IsKeynote() bool {
	return kind == KEYNOTE
}

func (kind FileKind) IsDocsify() bool {
	return kind == DOCSIFY
}

const (
	KEYNOTE FileKind = "keynote"
	DOCSIFY FileKind = "docsify"
	GITBOOK FileKind = "gitbook"
)

func FileKinds() []FileKind {
	return []FileKind{KEYNOTE, DOCSIFY, GITBOOK}
}

type FolderProps struct {
	Keynote, Docsify, Gitbook, Ignore []string
}

func (props *FolderProps) getFileKind(file string) (fileKind FileKind, found bool) {
	for _, f := range props.Docsify {
		if f == file {
			return DOCSIFY, true
		}
	}

	for _, f := range props.Gitbook {
		if f == file {
			return GITBOOK, true
		}
	}

	for _, f := range props.Keynote {
		if f == file {
			return KEYNOTE, true
		}
	}

	return "", false
}

func loadFolderProps(conf string) (props *FolderProps) {
	data, err := os.ReadFile(conf)
	fatalErr(err)

	props = &FolderProps{}

	fatalErr(yaml.Unmarshal(data, props))
	return
}

type File struct {
	Name, Title string
	Kind        FileKind
	Ctime       time.Time
}

type Folder struct {
	// private fields
	path string

	// public fields
	Name, Title string
	Breadcrumb  []string
	SubFolders  []*Folder
	Files       []*File
}

// Only support files with `.md` suffix or gitbook directories.
func loadKeynotes(keynotesDir, folderName string, breadcrumb []string) (folder *Folder) {
	folder = &Folder{
		path:       keynotesDir,
		Name:       folderName,
		Title:      strings.ReplaceAll(folderName, "-", " "),
		Breadcrumb: breadcrumb,
	}

	folderProps := loadFolderProps(filepath.Join(keynotesDir, ".folder.yaml"))
	entries, _ := os.ReadDir(keynotesDir)
outer:
	for _, v := range entries {
		// check ignore list
		for _, ignore := range folderProps.Ignore {
			if v.Name() == ignore {
				continue outer
			}
		}

		// ignore hidden file or directory
		if strings.HasPrefix(v.Name(), ".") {
			continue
		}

		fileKind, ok := folderProps.getFileKind(v.Name())

		if !v.IsDir() {
			// The file without `.md` suffix is ignored.
			if isMarkdown := strings.HasSuffix(v.Name(), ".md"); !isMarkdown {
				log.Println(v.Name(), "file ignored because not supported")
				continue
			}

			// The file is ignored, if it is:
			// 1) absent from `.folder.yaml`.
			// 2) neither a keynote nor a docsify.
			if !(ok && (fileKind.IsKeynote() || fileKind.IsDocsify())) {
				log.Println(v.Name(), "file ignored because of invalid config")
				continue
			}
		} else {
			// The directory is ignored, if it appeared
			// in `.folder.yaml`, but isn't a gitbook.
			// In other words, the directory must be a `folder` or a gitbook.
			if ok && !fileKind.IsGitbook() {
				log.Println(v.Name(), "dir ignored because of invalid config")
				continue
			}
		}

		// If the directory isn't a gitbook, then it's a sub-folder.
		if v.IsDir() && !fileKind.IsGitbook() {
			subBreadcrumb := make([]string, len(folder.Breadcrumb)+1)
			copy(subBreadcrumb, folder.Breadcrumb)
			subBreadcrumb[len(subBreadcrumb)-1] = v.Name()

			// Recursively load sub-folder
			subFolder := loadKeynotes(filepath.Join(keynotesDir, v.Name()), v.Name(), subBreadcrumb)

			// Add a sub-folder
			folder.SubFolders = append(folder.SubFolders, subFolder)
			continue
		}

		// Ignored if error occurs
		info, err := v.Info()
		if err != nil {
			continue
		}

		// Get time of the last change
		stat := info.Sys().(*syscall.Stat_t)
		ctime := time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))

		// Remove suffix '.md' of keynote or docsify
		name := v.Name()
		if fileKind.IsKeynote() || fileKind.IsDocsify() {
			name = name[:len(name)-3]
		}

		// Make sure gitbook's name is different from the name of keynote or docsify (with suffix `.md` removed).
		for _, file := range folder.Files {
			if file.Name == name {
				log.Println(v.Name(), "file ignored because of duplicated name")
				continue
			}
		}

		// Add a file
		folder.Files = append(folder.Files, &File{
			Name:  name,
			Title: strings.ReplaceAll(name, "-", " "),
			Kind:  fileKind,
			Ctime: ctime,
		})
	}

	// Sort files by time of the last change in descending order
	if len(folder.Files) > 0 {
		sort.Slice(folder.Files, func(i, j int) bool {
			return folder.Files[i].Ctime.After(folder.Files[j].Ctime)
		})
	}
	return
}

//go:embed templates/* templates/blocks/*
var tmplFS embed.FS

func homeRender(ch chan<- chan<- []any) func(*gin.Context) {
	return func(c *gin.Context) {
		site, _ := getData(ch)
		c.HTML(http.StatusOK, "index.htm", gin.H{"Site": site, "Year": time.Now().Year()})
	}
}

func foldersApi(ch chan<- chan<- []any) func(*gin.Context) {
	return func(c *gin.Context) {
		_, rootFolder := getData(ch)
		c.JSON(http.StatusOK, gin.H{
			"RootFolder": rootFolder,
		})
	}
}

func keynoteRender(c *gin.Context, tmplHtm string, site *Site, keynoteDir, keynoteName, keynoteTitle string) {
	c.HTML(http.StatusOK, tmplHtm, gin.H{
		"KeynoteDir":   keynoteDir,
		"KeynoteName":  keynoteName,
		"KeynoteTitle": keynoteTitle,
		"Site":         site,
	})
}

func noRouteHandler(ch chan<- chan<- []any) func(*gin.Context) {
	return func(c *gin.Context) {
		site, rootFolder := getData(ch)
		path := c.Request.URL.Path
		kinds := FileKinds()
		for _, kind := range kinds {
			if keynoteName, found := strings.CutPrefix(path, fmt.Sprintf("/%ss/", kind)); found {
				breadcrumb := strings.Split(keynoteName, "/")
				if len(breadcrumb) == 1 {
					for _, kn := range rootFolder.Files {
						if kn.Name == breadcrumb[0] {
							keynoteRender(c, fmt.Sprintf("%s.htm", kind), &site, fmt.Sprintf("%ss", kind), keynoteName, kn.Title)
							return
						}
					}
				} else {
					p := rootFolder.SubFolders
					for i := 0; i < len(breadcrumb)-1; i++ {
						for _, f := range p {
							if f.Name == breadcrumb[i] {
								if i == len(breadcrumb)-2 {
									for _, kn := range f.Files {
										if kn.Name == breadcrumb[len(breadcrumb)-1] {
											keynoteRender(c, fmt.Sprintf("%s.htm", kind), &site, fmt.Sprintf("%ss", kind), keynoteName, kn.Title)
											return
										}
									}
								} else {
									p = f.SubFolders
									break
								}
							}
						}
					}
				}
			}
		}
		c.AbortWithStatus(http.StatusNotFound)
	}
}

func fatalErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func newTemplate() (tmpl *template.Template) {
	tmpl = template.Must(template.New("").ParseFS(tmplFS, "templates/*.htm", "templates/blocks/*.htm"))
	return
}

func genKeynoteHtml(kind FileKind, tmpl *template.Template, site *Site, folder *Folder, path, basePath string) {
	os.Mkdir(path, os.ModePerm)

	for _, kn := range folder.Files {
		// Process one kind of file at a time.
		if kn.Kind != kind {
			continue
		}
		if kind.IsKeynote() || kind.IsDocsify() {
			// copy original `.md` files for keynote and docsify
			mdFile := kn.Name + ".md"
			data, _ := os.ReadFile(filepath.Join(folder.path, mdFile))
			mdPath := filepath.Join(path, mdFile)
			os.WriteFile(mdPath, data, os.ModePerm)

			// generate `.html` file
			knHtmlPath := filepath.Join(path, kn.Name+".html")
			knHtml, _ := os.Create(knHtmlPath)

			urlPath := strings.Join(folder.Breadcrumb[1:], "/")
			keynoteName, _ := url.JoinPath(urlPath, kn.Name)
			tmpl.ExecuteTemplate(knHtml, fmt.Sprintf("%s.htm", kind), gin.H{
				"KeynoteDir":   filepath.Join(basePath, fmt.Sprintf("%ss", kind))[1:],
				"KeynoteName":  keynoteName,
				"KeynoteTitle": kn.Title,
				"Site":         site,
			})
		} else if kind.IsGitbook() {
			// copy latest dir for gitbook
			latestDir := filepath.Join(folder.path, kn.Name, "latest")
			gitbookDir := filepath.Join(path, kn.Name, "latest")
			os.MkdirAll(gitbookDir, os.ModePerm)
			fatalErr(cp.Copy(latestDir, gitbookDir))
		}
	}

	for _, f := range folder.SubFolders {
		genKeynoteHtml(kind, tmpl, site, f, filepath.Join(path, f.Name), basePath)
	}
}

func genStaticSite(conf, keynotesDir, outputDir, basePath string) {
	if _, err := os.Stat(keynotesDir); os.IsNotExist(err) {
		fatalErr(err)
	}

	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		fatalErr(err)
	}

	tmpl := newTemplate()
	// load data
	site := loadSite(conf)
	rootFolder := loadKeynotes(keynotesDir, "/", []string{"/"})

	// clear old index.html
	indexPath := filepath.Join(outputDir, "index.html")
	os.Remove(indexPath)
	// re-generate index.html
	indexHtml, _ := os.Create(indexPath)
	tmpl.ExecuteTemplate(indexHtml, "index.htm", gin.H{"Site": site, "Year": time.Now().Year()})

	// clear old folders.json
	foldersJsonPath := filepath.Join(outputDir, "folders.json")
	os.Remove(foldersJsonPath)
	// re-generate folders.json
	if data, err := json.Marshal(gin.H{
		"RootFolder": rootFolder,
	}); err != nil {
		fatalErr(err)
	} else {
		fatalErr(os.WriteFile(foldersJsonPath, data, os.ModePerm))
	}

	// generate keynotes
	for _, kind := range FileKinds() {
		// clear old keynotes
		keynotesPath := filepath.Join(outputDir, fmt.Sprintf("%ss", kind))
		os.RemoveAll(keynotesPath)
		// re-generate keynotes
		genKeynoteHtml(kind, tmpl, site, rootFolder, keynotesPath, basePath)
	}
}

func startServer(port int, host, conf, keynotesDir string) {
	ch := make(chan chan<- []any, 1024)
	go loadData(conf, keynotesDir, ch)

	router := gin.Default()

	router.SetHTMLTemplate(newTemplate())

	for _, kind := range FileKinds() {
		router.StaticFS(fmt.Sprintf("%ss", kind), gin.Dir(keynotesDir, false))
	}

	site, _ := getData(ch)
	// Site.StaticPath is a server mode parameter, please restart server after modification.
	for _, path := range site.StaticPath {
		router.StaticFS(path, gin.Dir(keynotesDir, false))
	}
	// Site.StaticFile is a server mode parameter, please restart server after modification.
	for _, file := range site.StaticFile {
		router.StaticFile(file, file)
	}

	router.GET("/", homeRender(ch))
	router.GET("/folders.json", foldersApi(ch))
	router.NoRoute(noRouteHandler(ch))

	router.SetTrustedProxies(nil)
	router.Run(fmt.Sprintf("%s:%d", host, port))
}

func getData(ch chan<- chan<- []any) (site Site, rootFolder Folder) {
	recv := make(chan []any, 1)
	ch <- recv
	arr := <-recv
	site = arr[0].(Site)
	rootFolder = arr[1].(Folder)
	return
}

func loadData(conf, keynotesDir string, ch <-chan chan<- []any) {
	ticker := time.NewTicker(3 * time.Second)

	if production {
		ticker.Stop()
	} else {
		defer ticker.Stop()
	}

	var (
		site       *Site
		rootFolder *Folder
	)

	load := func() {
		site = loadSite(conf)
		rootFolder = loadKeynotes(keynotesDir, "/", []string{"/"})
	}

	load()

	for {
		select {
		case res := <-ch:
			res <- []any{*site, *rootFolder}
		case <-ticker.C:
			load()
		}
	}
}

var production bool

func main() {
	var (
		port                    int
		host, conf, keynotesDir string
		gen                     bool
		outputDir, basePath     string
	)
	flag.IntVar(&port, "port", 8000, "the port that server listen on")
	flag.StringVar(&host, "host", "0.0.0.0", "the host that server listen on")
	flag.StringVar(&conf, "conf", "keynote.yaml", "the config of the site")
	flag.StringVar(&keynotesDir, "src", "src", "where the keynote sources store")
	flag.BoolVar(&production, "pro", false, "production mode (without auto reload)")
	flag.BoolVar(&gen, "gen", false, "generate static site")
	flag.StringVar(&outputDir, "output", ".", "where the generated files store")
	flag.StringVar(&basePath, "base", "/", "base path of the static site")
	flag.Parse()

	if gen {
		genStaticSite(conf, keynotesDir, outputDir, basePath)
	} else {
		startServer(port, host, conf, keynotesDir)
	}
}
