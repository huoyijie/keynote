package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed templates/* templates/blocks/*
var tmplFS embed.FS

func homeRender(keynotes []keynote_t) func(*gin.Context) {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.htm", gin.H{"Keynotes": keynotes})
	}
}

type get_keynote_t struct {
	KeynoteName string `uri:"name" binding:"required"`
}

func keynoteRender() func(*gin.Context) {
	return func(c *gin.Context) {
		var query get_keynote_t
		if err := c.BindUri(&query); err != nil {
			c.Redirect(http.StatusFound, "/")
			return
		}

		c.HTML(http.StatusOK, "keynote.htm", gin.H{
			"KeynoteName": query.KeynoteName,
		})
	}
}

func startServer(port int, host, keynotesDir string) {
	router := gin.Default()

	tmpl := template.Must(template.New("").ParseFS(tmplFS, "templates/*.htm", "templates/blocks/*.htm"))
	router.SetHTMLTemplate(tmpl)

	router.StaticFS("public", http.Dir(keynotesDir))

	router.GET("/", homeRender(loadKeynotes(keynotesDir)))

	router.GET("keynotes/:name", keynoteRender())

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
	flag.StringVar(&keynotesDir, "d", "./keynotes", "where the keynotes store")
	flag.Parse()

	if g {

	} else {
		startServer(port, host, keynotesDir)
	}
}
