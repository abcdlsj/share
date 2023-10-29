package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
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
	Style   string
	ErrMsg  string
}

type shareArt struct {
	Path  string `json:"path"`
	Style string `json:"style"`
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

	artsCache    = make(map[string]article)
	shareStorage = make(map[string]shareArt)

	tmpl = template.Must(template.New("article.html").Funcs(funcMap).ParseFS(tmplFiles, "article.html", "index.html"))
)

func main() {
	r := mux.NewRouter()
	r.SkipClean(true)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.FS(cssFile))))

	r.HandleFunc("/", indexHandler)
	r.PathPrefix("/read/").HandlerFunc(readHandler)
	r.PathPrefix("/read").Methods("POST").HandlerFunc(readRedirectHandler)
	r.PathPrefix("/share/render/").HandlerFunc(shareRenderHandler)
	r.PathPrefix("/share").Methods("POST").HandlerFunc(shareHandler)

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

func shareHandler(w http.ResponseWriter, r *http.Request) {
	var p shareArt

	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	p.Path = strings.TrimPrefix(p.Path, "/read/")
	p.Path = unescape(p.Path)

	for uuid, s := range shareStorage {
		if s.Path == p.Path && s.Style == p.Style {
			http.Redirect(w, r, "/share/render/"+uuid, http.StatusSeeOther)
			return
		}
	}

	uuid := uuid()

	shareStorage[uuid] = p
	http.Redirect(w, r, "/share/render/"+uuid, http.StatusSeeOther)
}

func shareRenderHandler(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.EscapedPath()[len("/share/render/"):]

	if uuid == "" {
		http.NotFound(w, r)
		return
	}

	if p, ok := shareStorage[uuid]; ok {
		art := readabyFormURL(p.Path)
		art.Style = p.Style
		render(w, art)
		return
	}

	http.NotFound(w, r)
}

func uuid() string {
	uuid := make([]byte, 16)
	_, err := rand.Read(uuid)
	if err != nil {
		panic(err)
	}
	return strings.ToUpper(hex.EncodeToString(uuid))
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

	render(w, readabyFormURL(uri))
}

func readabyFormURL(uri string) article {
	if cache, ok := artsCache[uri]; ok {
		return cache
	}

	art, err := readability.FromURL(uri, 30*time.Second)
	if err != nil {
		return article{URL: uri, ErrMsg: err.Error()}
	}

	artsCache[uri] = article{URL: uri, Title: art.Title, Content: art.Content}
	return article{URL: uri, Title: art.Title, Content: art.Content}
}

func render(w http.ResponseWriter, data article) {
	err := tmpl.ExecuteTemplate(w, "article.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
