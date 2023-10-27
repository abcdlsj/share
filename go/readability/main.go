package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	readability "github.com/go-shiori/go-readability"
	"github.com/gorilla/mux"
)

type article struct {
	URL     string
	Title   string
	Content string
	ErrMsg  string
}

var (
	//go:embed *.html
	tmplFiles embed.FS

	//go:embed style.css
	cssFile embed.FS

	funcMap = template.FuncMap{
		"safeHTML": func(content string) template.HTML {
			return template.HTML(content)
		},
	}

	tmpl = template.Must(template.New("article.html").Funcs(funcMap).ParseFS(tmplFiles, "article.html", "index.html"))
)

func main() {
	r := mux.NewRouter()
	r.SkipClean(true)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.FS(cssFile))))

	r.HandleFunc("/", indexHandler)
	r.PathPrefix("/read/").HandlerFunc(readHandler)
	r.PathPrefix("/read").Methods("POST").HandlerFunc(readRedirectHandler)

	log.Fatal(http.ListenAndServe(port(), r))
}

func port() string {
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}

	return ":8080"
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	err := tmpl.ExecuteTemplate(w, "index.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func readRedirectHandler(w http.ResponseWriter, r *http.Request) {
	uri := r.FormValue("url")
	http.Redirect(w, r, "/read/"+escape(uri), http.StatusSeeOther)
}

func escape(s string) string {
	replacer := strings.NewReplacer(
		"/", "%2F",
	)

	return replacer.Replace(s)
}

func unescape(s string) string {
	replacer := strings.NewReplacer(
		"%2F", "/",
	)

	return replacer.Replace(s)
}

func readHandler(w http.ResponseWriter, r *http.Request) {
	uri := r.URL.EscapedPath()[len("/read/"):]

	if uri == "" {
		http.NotFound(w, r)
		return
	}

	uri = unescape(uri)

	art, err := readability.FromURL(uri, 30*time.Second)
	if err != nil {
		render(w, article{URL: uri, ErrMsg: err.Error()})
		return
	}

	render(w, article{URL: uri, Title: art.Title, Content: art.Content})
}

func render(w http.ResponseWriter, data article) {
	err := tmpl.ExecuteTemplate(w, "article.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
