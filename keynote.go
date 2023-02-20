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
	"gopkg.in/yaml.v3"
)

type Site struct {
	Logo, Link, Icon, Name, Title, Author, Desc, Summry, Copyright string
}

func loadSite(conf string) (site *Site) {
	data, err := os.ReadFile(conf)
	fatalErr(err)

	site = &Site{}

	err = yaml.Unmarshal(data, site)
	fatalErr(err)
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
	Keynote, Docsify, Gitbook []string
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

	err = yaml.Unmarshal(data, props)
	fatalErr(err)
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
	for _, v := range entries {
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

func homeRender(site *Site) func(*gin.Context) {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.htm", gin.H{"Site": site, "Year": time.Now().Year()})
	}
}

func foldersApi(rootFolder *Folder) func(*gin.Context) {
	return func(c *gin.Context) {
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

func noRouteHandler(rootFolder *Folder, site *Site) func(*gin.Context) {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		kinds := FileKinds()
		for _, kind := range kinds {
			if keynoteName, found := strings.CutPrefix(path, fmt.Sprintf("/%ss/", kind)); found {
				breadcrumb := strings.Split(keynoteName, "/")
				if len(breadcrumb) == 1 {
					for _, kn := range rootFolder.Files {
						if kn.Name == breadcrumb[0] {
							keynoteRender(c, fmt.Sprintf("%s.htm", kind), site, fmt.Sprintf("%ss", kind), keynoteName, kn.Title)
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
											keynoteRender(c, fmt.Sprintf("%s.htm", kind), site, fmt.Sprintf("%ss", kind), keynoteName, kn.Title)
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

func genKeynoteHtml(tmpl *template.Template, site *Site, folder *Folder, path, basePath string) {
	_ = os.Mkdir(path, os.ModePerm)

	for _, kn := range folder.Files {
		mdFile := kn.Name + ".md"
		data, _ := os.ReadFile(filepath.Join(folder.path, mdFile))
		mdPath := filepath.Join(path, mdFile)
		os.WriteFile(mdPath, data, os.ModePerm)

		knHtmlPath := filepath.Join(path, kn.Name+".html")
		knHtml, _ := os.Create(knHtmlPath)

		urlPath := strings.Join(folder.Breadcrumb[1:], "/")
		keynoteName, _ := url.JoinPath(urlPath, kn.Name)
		tmpl.ExecuteTemplate(knHtml, "keynote.htm", gin.H{
			"KeynoteDir":  filepath.Join(basePath, "keynotes")[1:],
			"KeynoteName": keynoteName,
			"Site":        site,
		})
	}

	for _, f := range folder.SubFolders {
		genKeynoteHtml(tmpl, site, f, filepath.Join(path, f.Name), basePath)
	}
}

func genStaticSite(site *Site, keynotesDir, outputDir, basePath string) {
	if _, err := os.Stat(keynotesDir); os.IsNotExist(err) {
		fatalErr(err)
	}

	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		fatalErr(err)
	}

	indexPath := filepath.Join(outputDir, "index.html")
	os.Remove(indexPath)

	foldersJsonPath := filepath.Join(outputDir, "folders.json")
	os.Remove(foldersJsonPath)

	keynotesPath := filepath.Join(outputDir, "keynotes")
	os.RemoveAll(keynotesPath)

	tmpl := newTemplate()

	indexHtml, _ := os.Create(indexPath)
	tmpl.ExecuteTemplate(indexHtml, "index.htm", gin.H{"Site": site, "Year": time.Now().Year()})

	rootFolder := loadKeynotes(keynotesDir, "/", []string{"/"})

	if data, err := json.Marshal(gin.H{
		"RootFolder": rootFolder,
	}); err != nil {
		fatalErr(err)
	} else {
		err = os.WriteFile(foldersJsonPath, data, os.ModePerm)
		fatalErr(err)
	}

	genKeynoteHtml(tmpl, site, rootFolder, keynotesPath, basePath)
}

func startServer(port int, host, keynotesDir string, site *Site) {
	router := gin.Default()

	router.SetHTMLTemplate(newTemplate())

	for _, kind := range FileKinds() {
		router.StaticFS(fmt.Sprintf("%ss", kind), http.Dir(keynotesDir))
	}

	rootFolder := loadKeynotes(keynotesDir, "/", []string{"/"})
	router.GET("/", homeRender(site))
	router.GET("/folders.json", foldersApi(rootFolder))
	router.NoRoute(noRouteHandler(rootFolder, site))

	router.SetTrustedProxies(nil)
	router.Run(fmt.Sprintf("%s:%d", host, port))
}

func main() {
	var (
		port                    int
		host, conf, keynotesDir string
		g                       bool
		outputDir, basePath     string
	)
	flag.IntVar(&port, "p", 8000, "the port that server listen on")
	flag.StringVar(&host, "H", "0.0.0.0", "the host that server listen on")
	flag.StringVar(&conf, "c", "site.yaml", "the config of the site")
	flag.StringVar(&keynotesDir, "d", "src", "where the keynote sources store")
	flag.BoolVar(&g, "g", false, "generate static site")
	flag.StringVar(&outputDir, "o", ".", "where the generated files store")
	flag.StringVar(&basePath, "b", "/", "base path of the static site")
	flag.Parse()

	site := loadSite(conf)
	if g {
		genStaticSite(site, keynotesDir, outputDir, basePath)
	} else {
		startServer(port, host, keynotesDir, site)
	}
}
