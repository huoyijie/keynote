package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"net/http"
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

func noRouteHandler(rootFolder *folder_t) func(*gin.Context) {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if keynoteName, found := strings.CutPrefix(path, "/keynotes/"); found {
			breadcrumb := strings.Split(keynoteName, "/")
			if len(breadcrumb) == 1 {
				for _, kn := range rootFolder.Keynotes {
					if kn.Name == breadcrumb[0] {
						c.HTML(http.StatusOK, "keynote.htm", gin.H{
							"KeynoteName": keynoteName,
						})
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
										c.HTML(http.StatusOK, "keynote.htm", gin.H{
											"KeynoteName": keynoteName,
										})
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

func startServer(port int, host, keynotesDir string) {
	router := gin.Default()

	tmpl := template.Must(template.New("").ParseFS(tmplFS, "templates/*.htm", "templates/blocks/*.htm"))
	router.SetHTMLTemplate(tmpl)

	router.StaticFS("public", http.Dir(keynotesDir))

	rootFolder := loadKeynotes(keynotesDir, "/", []string{"/"})
	router.GET("/", homeRender())
	router.GET("/folders", foldersApi(rootFolder))
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
	)
	flag.StringVar(&host, "H", "0.0.0.0", "the host that server listen on")
	flag.IntVar(&port, "p", 8000, "the port that server listen on")
	flag.BoolVar(&g, "g", false, "generate static site")
	flag.StringVar(&keynotesDir, "d", "keynotes", "where the keynotes store")
	flag.Parse()

	if g {

	} else {
		startServer(port, host, keynotesDir)
	}
}
