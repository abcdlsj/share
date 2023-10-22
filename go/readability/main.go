package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	readability "github.com/go-shiori/go-readability"
)

type Article struct {
	URL      string
	Title    string
	Content  string
	ErrorMsg string
}

var (
	//go:embed *.html
	tmplFiles embed.FS

	funcMap = template.FuncMap{
		"safeHTML": func(content string) template.HTML {
			return template.HTML(content)
		},
	}

	tmpl *template.Template
)

func init() {
	tmpl = template.Must(template.New("article.html").Funcs(funcMap).ParseFS(tmplFiles, "article.html", "index.html"))
}

func main() {
	http.HandleFunc("/", index)
	http.HandleFunc("/read", read)

	log.Fatal(http.ListenAndServe(port(), nil))
}

func port() string {
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}

	return ":8080"
}

func index(w http.ResponseWriter, r *http.Request) {
	err := tmpl.ExecuteTemplate(w, "index.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func read(w http.ResponseWriter, r *http.Request) {
	url := r.FormValue("url")

	article, err := readability.FromURL(url, 30*time.Second)
	if err != nil {
		render(w, Article{URL: url, ErrorMsg: err.Error()})
		return
	}

	render(w, Article{URL: url, Title: article.Title, Content: article.Content})
}

func render(w http.ResponseWriter, data Article) {
	err := tmpl.ExecuteTemplate(w, "article.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
