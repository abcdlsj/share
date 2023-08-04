package main

import (
	"bytes"
	"database/sql"
	"embed"
	"encoding/json"
	"flag"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

var (
	HOMEDIR, _ = os.UserHomeDir()

	db *sql.DB

	//go:embed static tmpl
	FS embed.FS

	SQLSchema = `CREATE TABLE IF NOT EXISTS links (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		short VARCHAR(32) NOT NULL,
		long VARCHAR(500) NOT NULL,
		redirects INTEGER NOT NULL DEFAULT 0
	)`

	DbPath = filepath.Join(HOMEDIR, ".g.db")

	needAddEtcHostItem = false
)

func addEtcHostItem() {
	raw, err := os.ReadFile("/etc/hosts")
	if err != nil {
		log.Fatalf("failed to read hosts file: %v", err)
	}

	newRow := "127.0.0.1" + " " + "go"
	if !bytes.Contains(raw, []byte(newRow)) {
		raw = append(raw, []byte(newRow)...)
	}

	err = os.WriteFile("/etc/hosts", raw, 0644)
	if err != nil {
		log.Fatalf("failed to write hosts file: %v", err)
	}
}

func init() {
	flag.StringVar(&DbPath, "db", DbPath, "path to sqlite db")
	flag.BoolVar(&needAddEtcHostItem, "i", false, "add item to /etc/hosts")
	flag.Parse()

	if needAddEtcHostItem {
		// need run as root
		addEtcHostItem()
	}

	var err error
	db, err = sql.Open("sqlite", DbPath)
	if err != nil {
		log.Fatalf("failed to open sqlite: %v", err)
	}

	_, err = db.Exec(SQLSchema)
	if err != nil {
		log.Fatalf("failed to create table: %v", err)
	}
}

func main() {
	serve()
}

type Link struct {
	ID        int    `json:"id"`
	Short     string `json:"short"`
	Long      string `json:"long"`
	Redirects int    `json:"redirects"`
}

func serve() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	tmpl := template.Must(template.New("").ParseFS(FS, "tmpl/*.html"))
	r.SetHTMLTemplate(tmpl)

	fe, _ := fs.Sub(FS, "static")
	r.StaticFS("/static", http.FS(fe))

	r.GET("/", func(c *gin.Context) {
		var links []Link
		rows, err := db.Query("SELECT * FROM links ORDER BY redirects DESC")
		if err != nil {
			c.String(http.StatusInternalServerError, "failed to query: %v", err)
			return
		}

		for rows.Next() {
			var link Link
			err = rows.Scan(&link.ID, &link.Short, &link.Long, &link.Redirects)
			if err != nil {
				c.String(500, "failed to scan: %v", err)
				return
			}

			links = append(links, link)
		}

		c.HTML(http.StatusOK, "index.html", gin.H{
			"links": links,
		})
	})

	r.GET("/:s", func(c *gin.Context) {
		s := c.Param("s")

		var link Link
		rows, err := db.Query("SELECT * FROM links WHERE short = ?", s)
		if err != nil {
			c.String(http.StatusInternalServerError, "failed to query: %v", err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			err = rows.Scan(&link.ID, &link.Short, &link.Long, &link.Redirects)
			if err != nil {
				c.String(http.StatusInternalServerError, "failed to scan: %v", err)
				return
			}
		}

		if link.Short == "" {
			c.String(http.StatusNotFound, "not found")
			return
		}

		go func() {
			link.Redirects++
			_, _ = db.Exec("UPDATE links SET redirects = ? WHERE id = ?", link.Redirects, link.ID)
		}()

		c.Redirect(http.StatusTemporaryRedirect, link.Long)
	})

	type LinkReq struct {
		Short string `form:"short" binding:"required"`
		Long  string `form:"long" binding:"required"`
	}

	r.POST("/", func(c *gin.Context) {
		var req LinkReq
		if err := c.ShouldBind(&req); err != nil {
			c.String(http.StatusBadRequest, "missing short or long link")
			return
		}

		if len(req.Short) > 32 {
			c.String(http.StatusBadRequest, "short link too long")
			return
		}

		if len(req.Long) > 500 {
			c.String(http.StatusBadRequest, "long link too long")
			return
		}

		_, err := db.Exec("INSERT INTO links (short, long) VALUES (?, ?)", req.Short, req.Long)
		if err != nil {
			c.String(http.StatusInternalServerError, "failed to insert: %v", err)
			return
		}

		c.Redirect(http.StatusTemporaryRedirect, req.Long)
	})

	r.POST("/.export", func(c *gin.Context) {
		var links []Link
		rows, err := db.Query("SELECT * FROM links")
		if err != nil {
			c.String(http.StatusInternalServerError, "failed to query: %v", err)
			return
		}

		for rows.Next() {
			var link Link
			err = rows.Scan(&link.ID, &link.Short, &link.Long, &link.Redirects)
			if err != nil {
				c.String(500, "failed to scan: %v", err)
				return
			}

			links = append(links, link)
		}

		jsonData, err := json.Marshal(links)
		if err != nil {
			c.String(http.StatusInternalServerError, "failed to marshal: %v", err)
		}
		c.Data(http.StatusOK, "application/json", jsonData)
	})

	r.POST("/.import", func(c *gin.Context) {
		var links []Link
		err := c.ShouldBind(&links)
		if err != nil {
			c.String(http.StatusBadRequest, "failed to bind: %v", err)
			return
		}

		for _, link := range links {
			_, err := db.Exec("INSERT INTO links (id, short, long, redirects) VALUES (?, ?, ?, ?)", link.ID, link.Short, link.Long, link.Redirects)
			if err != nil {
				c.String(http.StatusInternalServerError, "failed to insert: %v", err)
				return
			}
		}

		c.String(http.StatusOK, "ok")
	})

	r.Run(":80")
}
