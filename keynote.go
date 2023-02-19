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
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

//go:embed templates/* templates/blocks/*
var tmplFS embed.FS

func homeRender(site *SiteConfig) func(*gin.Context) {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.htm", gin.H{"Site": site, "Year": time.Now().Year()})
	}
}

func foldersApi(rootFolder *folder_t) func(*gin.Context) {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"RootFolder": rootFolder,
		})
	}
}

func keynoteRender(c *gin.Context, site *SiteConfig, keynoteDir, keynoteName string) {
	c.HTML(http.StatusOK, "keynote.htm", gin.H{
		"KeynoteDir":  keynoteDir,
		"KeynoteName": keynoteName,
		"Site":        site,
	})
}

func noRouteHandler(rootFolder *folder_t, site *SiteConfig) func(*gin.Context) {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if keynoteName, found := strings.CutPrefix(path, "/keynotes/"); found {
			breadcrumb := strings.Split(keynoteName, "/")
			if len(breadcrumb) == 1 {
				for _, kn := range rootFolder.Keynotes {
					if kn.Name == breadcrumb[0] {
						keynoteRender(c, site, "public", keynoteName)
						return
					}
				}
			} else {
				p := rootFolder.SubFolders
				for i := 0; i < len(breadcrumb)-1; i++ {
					for _, f := range p {
						if f.Name == breadcrumb[i] {
							if i == len(breadcrumb)-2 {
								for _, kn := range f.Keynotes {
									if kn.Name == breadcrumb[len(breadcrumb)-1] {
										keynoteRender(c, site, "public", keynoteName)
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

func genKeynoteHtml(tmpl *template.Template, site *SiteConfig, folder *folder_t, path, basePath string) {
	_ = os.Mkdir(path, os.ModePerm)

	for _, kn := range folder.Keynotes {
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

func genStaticSite(site *SiteConfig, keynotesDir, outputDir, basePath string) {
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

func startServer(port int, host, keynotesDir string, site *SiteConfig) {
	router := gin.Default()

	router.SetHTMLTemplate(newTemplate())

	router.StaticFS("public", http.Dir(keynotesDir))

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

	site := loadConfig(conf)
	if g {
		genStaticSite(site, keynotesDir, outputDir, basePath)
	} else {
		startServer(port, host, keynotesDir, site)
	}
}
