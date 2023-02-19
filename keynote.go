package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed templates/* templates/blocks/*
var tmplFS embed.FS

func homeRender() func(*gin.Context) {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.htm", gin.H{})
	}
}

func foldersApi(rootFolder *folder_t) func(*gin.Context) {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"RootFolder": rootFolder,
		})
	}
}

func keynoteRender(c *gin.Context, keynoteDir, keynoteName string) {
	c.HTML(http.StatusOK, "keynote.htm", gin.H{
		"KeynoteDir":  keynoteDir,
		"KeynoteName": keynoteName,
	})
}

func noRouteHandler(rootFolder *folder_t) func(*gin.Context) {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if keynoteName, found := strings.CutPrefix(path, "/keynotes/"); found {
			breadcrumb := strings.Split(keynoteName, "/")
			if len(breadcrumb) == 1 {
				for _, kn := range rootFolder.Keynotes {
					if kn.Name == breadcrumb[0] {
						keynoteRender(c, "public", keynoteName)
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
										keynoteRender(c, "public", keynoteName)
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

func genStaticSite(keynotesDir, outputDir string) {
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

	indexHtm, _ := os.Create(indexPath)
	tmpl.ExecuteTemplate(indexHtm, "index.htm", gin.H{})

	rootFolder := loadKeynotes(keynotesDir, "/", []string{"/"})

	if data, err := json.Marshal(gin.H{
		"RootFolder": rootFolder,
	}); err != nil {
		fatalErr(err)
	} else {
		err = os.WriteFile(foldersJsonPath, data, os.ModePerm)
		fatalErr(err)
	}
}

func startServer(port int, host, keynotesDir string) {
	router := gin.Default()

	router.SetHTMLTemplate(newTemplate())

	router.StaticFS("public", http.Dir(keynotesDir))

	rootFolder := loadKeynotes(keynotesDir, "/", []string{"/"})
	router.GET("/", homeRender())
	router.GET("/folders.json", foldersApi(rootFolder))
	router.NoRoute(noRouteHandler(rootFolder))

	router.SetTrustedProxies(nil)
	router.Run(fmt.Sprintf("%s:%d", host, port))
}

func main() {
	var (
		host        string
		port        int
		g           bool
		keynotesDir string
		outputDir   string
	)
	flag.StringVar(&host, "H", "0.0.0.0", "the host that server listen on")
	flag.IntVar(&port, "p", 8000, "the port that server listen on")
	flag.StringVar(&keynotesDir, "d", "keynotes", "where the keynotes store")
	flag.BoolVar(&g, "g", false, "generate static site")
	flag.StringVar(&outputDir, "o", ".", "where the generated files store")
	flag.Parse()

	if g {
		genStaticSite(keynotesDir, outputDir)
	} else {
		startServer(port, host, keynotesDir)
	}
}
