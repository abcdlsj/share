package main

import (
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis"
	readability "github.com/go-shiori/go-readability"
	"github.com/gorilla/mux"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
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

	REDIS_URL = os.Getenv("REDIS_URL")

	redisclient *redis.Client

	mdparser = goldmark.New(
		goldmark.WithExtensions(
			meta.Meta,
			highlighting.Highlighting,
			extension.GFM,
			extension.Footnote,
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithHardWraps(), html.WithUnsafe()),
	)
)

func init() {
	opt, _ := redis.ParseURL(REDIS_URL)
	redisclient = redis.NewClient(opt)

	if err := redisclient.Ping().Err(); err != nil {
		log.Fatalf("Failed to connect to redis, URL: %s, error: %s", REDIS_URL, err.Error())
	}
}

func main() {
	r := mux.NewRouter()
	r.SkipClean(true)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.FS(cssFile))))

	r.HandleFunc("/", indexHandler)
	r.PathPrefix("/read/").HandlerFunc(readHandler)
	r.PathPrefix("/read").Methods("POST").HandlerFunc(readRedirectHandler)
	r.PathPrefix("/delete/").HandlerFunc(deleteHandler)

	log.Fatal(http.ListenAndServe(port(), r))
}

func port() string {
	if port := os.Getenv("PORT"); port != "" {
		log.Println("Listening on address, http://localhost:" + port)
		return ":" + port
	}

	log.Println("Listening on address, http://localhost:8080")
	return ":8080"
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	last10arts, err := getLastNArticles(10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	err = tmpl.ExecuteTemplate(w, "index.html", map[string]interface{}{
		"Recents": last10arts,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func readRedirectHandler(w http.ResponseWriter, r *http.Request) {
	uri := r.FormValue("url")
	http.Redirect(w, r, "/read/"+escape(uri), http.StatusTemporaryRedirect)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	uri, _, _ := parseURL(r.URL, len("/delete/"))

	if uri == "" {
		http.NotFound(w, r)
		return
	}

	if err := deleteArticle(uri); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
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
	uri, nocache, md := parseURL(r.URL, len("/read/"))

	if uri == "" {
		http.NotFound(w, r)
		return
	}

	uri = unescape(uri)

	render(w, readabyFormURL(uri, nocache, md))
}

func parseURL(u *url.URL, trimlen int) (string, bool, bool) {
	nocache, md := false, false

	query := u.RawQuery
	if u.Query().Get("nocache") == "true" {
		nocache = true

		query = strings.ReplaceAll(query, "&nocache=true", "")
	}

	if u.Query().Get("md") == "true" {
		md = true
		query = strings.ReplaceAll(query, "&md=true", "")
	}

	uri := u.EscapedPath()[trimlen:]

	if query != "" {
		uri = fmt.Sprintf("%s?%s", uri, query)
	}

	log.Printf("url: %s, nocache: %v, md: %v", uri, nocache, md)

	return uri, nocache, md
}

func readabyFormURL(uri string, nocache, md bool) *article {
	var art *article
	var err error
	var fromcache bool

	defer func() {
		log.Printf("defer readFormURL, article == nil: %v, err == nil: %v, nocache: %v", art == nil, err == nil, nocache)
		if err == nil && !fromcache && !nocache && art != nil && art.Content != "" {
			setArticleToCache(uri, art)
		}
	}()

	if !nocache {
		art, err = getArticleFromCache(uri)
		if err != nil || art != nil {
			fromcache = true
			return art
		}
	}

	title, content := "", ""

	if !md {
		var fromdata readability.Article
		fromdata, err = readability.FromURL(uri, 30*time.Second)
		if err != nil {
			return &article{URL: uri, ErrMsg: err.Error()}
		}

		title = fromdata.Title
		content = fromdata.Content
	} else {
		log.Printf("read markdown: %s", uri)
		var data []byte
		data, err = getDataFromURL(uri)
		if err != nil {
			return &article{URL: uri, ErrMsg: err.Error()}
		}
		var buf bytes.Buffer
		context := parser.NewContext()
		err = mdparser.Convert(data, &buf, parser.WithContext(context))
		if err != nil {
			return &article{URL: uri, ErrMsg: err.Error()}
		}

		title = "Readability - MD"
		content = buf.String()
	}

	art = &article{URL: uri, Title: title, Content: content}

	return art
}

func render(w http.ResponseWriter, data *article) {
	err := tmpl.ExecuteTemplate(w, "article.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func setArticleToCache(key string, art *article) error {
	data, err := json.Marshal(art)
	if err != nil {
		log.Printf("failed to marshal article to json: %s", err.Error())
		return err
	}

	defer func() {
		if err := lpushToRedis(key); err != nil {
			log.Printf("failed to push article to redis queue: %s", err.Error())
			return
		}
	}()

	if err := redisclient.Set(key, compress(data), 0).Err(); err != nil {
		log.Printf("failed to set article to redis cache: %s", err.Error())
		return err
	}

	return nil
}

func getArticleFromCache(key string) (*article, error) {
	var data []byte

	if err := redisclient.Get(key).Scan(&data); err != nil {
		if err == redis.Nil {
			return nil, nil
		}

		return &article{URL: key, ErrMsg: err.Error()}, errors.New("failed to get article from cache")
	}

	var art article
	if err := json.Unmarshal(uncompress(data), &art); err != nil {
		return &article{URL: key, ErrMsg: err.Error()}, errors.New("failed to unmarshal article from json")
	}

	log.Printf("get article from cache: %s", key)
	defer incrViewCount(key)

	return &art, nil
}

func incrViewCount(key string) error {
	return redisclient.ZIncrBy("readability-viewcount", 1, key).Err()
}

func lpushToRedis(key string) error {
	return redisclient.LPush("readability-timequeue", key).Err()
}

func getLastNArticles(n int) ([]string, error) {
	records := make([]string, 0, n)

	if err := redisclient.LRange("readability-timequeue", 0, int64(n)).ScanSlice(&records); err != nil {
		log.Printf("failed to get last %d articles from redis: %s", n, err.Error())
		return nil, err
	}

	return records, nil
}

func deleteArticle(uri string) error {
	redisclient.TxPipelined(func(pipe redis.Pipeliner) error {
		if err := pipe.Del(uri).Err(); err != nil {
			log.Printf("redis del failed: %s", err.Error())
			return err
		}

		if err := pipe.LRem("readability-timequeue", 0, uri).Err(); err != nil {
			log.Printf("lrem failed: %s", err.Error())
			return err
		}

		if err := pipe.ZRem("readability-viewcount", uri).Err(); err != nil {
			log.Printf("zrem failed: %s", err.Error())
			return err
		}

		return nil
	})

	return nil
}

func getDataFromURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func compress(data []byte) []byte {
	var cp bytes.Buffer
	gw := gzip.NewWriter(&cp)
	_, err := gw.Write(data)
	if err != nil {
		log.Printf("failed to compress data: %s", err.Error())
		return nil
	}
	err = gw.Close()
	if err != nil {
		log.Printf("failed to close gzip writer: %s", err.Error())
		return nil
	}

	return cp.Bytes()
}

func uncompress(data []byte) []byte {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		log.Printf("failed to uncompress data: %s", err.Error())
		return nil
	}
	defer gr.Close()

	var cp bytes.Buffer
	_, err = cp.ReadFrom(gr)
	if err != nil {
		log.Printf("failed to read uncompressed data: %s", err.Error())
		return nil
	}

	return cp.Bytes()
}
