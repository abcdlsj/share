package main

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

var gpm = &ProbeMgr{
	Probes: make(map[string]*Probe),
}

var (
	//go:embed static tmpl
	FS embed.FS

	tmplFuncs = template.FuncMap{
		"sub": func(a, b int) int {
			return a - b
		},
	}

	tmpl = template.Must(template.New("").Funcs(tmplFuncs).ParseFS(FS, "tmpl/*.html"))

	HOMEDIR, _ = os.UserHomeDir()

	defaultFile = filepath.Join(HOMEDIR, ".gprobe.json")

	PORT = os.Getenv("PORT")
)

func main() {
	gpm.InitFromFile()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.SetHTMLTemplate(tmpl)

	fe, _ := fs.Sub(FS, "static")
	r.StaticFS("/static", http.FS(fe))

	r.GET("/", indexPageHandler)
	r.POST("/probe/add", addProbeHandler)
	r.POST("/probe/delete/:uid", removeProbeHandler)
	r.GET("/ping/:uid", pingPageHandler)
	r.GET("/ping/:uid/latest", latestPingHandler)

	c := cron.New()
	c.AddFunc("@every 30s", gpm.CronProbe)
	c.Start()

	r.Run(":" + PORT)
}

func pingPageHandler(c *gin.Context) {
	uid := c.Param("uid")

	probe := gpm.GetProbe(uid)
	if probe == nil {
		c.String(400, "Invalid probe ID")
		return
	}

	c.HTML(200, "ping.html", probe)
}

func latestPingHandler(c *gin.Context) {
	uid := c.Param("uid")

	probe := gpm.GetProbe(uid)
	if probe == nil {
		c.String(400, "Invalid probe ID")
		return
	}

	result := probe.Ping(false)

	c.JSON(200, gin.H{
		"duration": int64(result.Duration / time.Millisecond),
	})
}

func indexPageHandler(c *gin.Context) {
	c.HTML(200, "index.html", gpm.GetProbes())
}

func addProbeHandler(c *gin.Context) {
	url := c.Request.FormValue("url")
	method := c.Request.FormValue("method")

	probe := &Probe{
		Uid:    Uuid(),
		URL:    url,
		Method: method,
	}

	_ = probe.Ping(true)

	gpm.AddProbe(probe)
	gpm.FlushFile()

	c.Redirect(302, "/")
}

func removeProbeHandler(c *gin.Context) {
	uid := c.Param("uid")

	probe := gpm.GetProbe(uid)
	if probe == nil {
		c.String(400, "Invalid probe ID")
		return
	}

	gpm.RemoveProbe(probe)
	gpm.FlushFile()

	c.Redirect(302, "/")
}

func Uuid() string {
	return uuid.New().String()[:8]
}

type Probe struct {
	Uid     string
	URL     string
	Method  string
	Results []Result
}

func (p *Probe) Ping(save bool) Result {
	start := time.Now()

	req, _ := http.NewRequest(p.Method, p.URL, nil)
	httpc := http.Client{
		Timeout: 5 * time.Second,
	}

	var status int
	resp, err := httpc.Do(req)
	if err != nil {
		status = 500
		log.Printf("Error probing %s: %s", p.URL, err)
	} else {
		status = resp.StatusCode
	}

	duration := time.Since(start)

	result := Result{
		Timestamp: time.Now(),
		Status:    status,
		Duration:  duration,
	}

	if save {
		p.Results = append(p.Results, result)
	}

	return result
}

type Result struct {
	Status    int
	Timestamp time.Time
	Duration  time.Duration
}

type ProbeMgr struct {
	Probes map[string]*Probe
	mu     sync.Mutex
}

func (pm *ProbeMgr) AddProbe(probe *Probe) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.Probes[probe.Uid] = probe
}

func (pm *ProbeMgr) RemoveProbe(probe *Probe) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	delete(pm.Probes, probe.Uid)
}

func (pm *ProbeMgr) GetProbe(id string) *Probe {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return pm.Probes[id]
}

func (pm *ProbeMgr) GetProbes() map[string]*Probe {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return pm.Probes
}

func (pm *ProbeMgr) InitFromFile() {
	if _, err := os.Stat(defaultFile); os.IsNotExist(err) {
		os.Create(defaultFile)
		return
	}

	data, err := os.ReadFile(defaultFile)
	if err != nil {
		return
	}

	probes := make(map[string]*Probe)
	_ = json.Unmarshal(data, &probes)
	pm.Probes = probes

	log.Printf("Loaded %d probes from %s", len(probes), defaultFile)
}

func (pm *ProbeMgr) FlushFile() {
	file, err := os.OpenFile(defaultFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return
	}

	defer file.Close()

	data, _ := json.Marshal(pm.GetProbes())
	_, _ = file.Write(data)

	log.Printf("Flushed %d probes to %s", len(pm.GetProbes()), defaultFile)
}

func (pm *ProbeMgr) CronProbe() {
	probes := pm.GetProbes()

	for _, probe := range probes {
		probe.Ping(true)
	}
}
