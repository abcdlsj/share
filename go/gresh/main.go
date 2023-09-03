package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/abcdlsj/cr"
	"github.com/fsnotify/fsnotify"
)

var wait = make(chan bool)
var startRun = make(chan interface{})
var ignoreRegex *regexp.Regexp
var wd string

var path string
var command string
var ignore string
var interval int64

func init() {
	flag.StringVar(&path, "p", "", "path to watch")
	flag.StringVar(&command, "c", "", "command to run")
	flag.StringVar(&ignore, "e", "", "exclude file name")
	flag.Int64Var(&interval, "i", 10, "interval to run")

	flag.Parse()

	if ignore != "" {
		ignoreRegex = regexp.MustCompile(regexp.QuoteMeta(ignore) + "$")
	}

	wd, _ = os.Getwd()
}

func main() {
	fmt.Printf("workdir: %s\npath: %s\ncommand: %s\nignore: %s\ninterval: %s\n\n", cr.PLBlue(wd), cr.PLBlue(path), cr.PLBlue(command), cr.PLBlue(ignore), cr.PLBlue(strconv.FormatInt(interval, 10)))

	fmt.Println("start watching...")
	watcher, _ := fsnotify.NewWatcher()
	defer watcher.Close()

	go run()

	go watch(watcher)

	if err := watcher.Add(path); err != nil {
		log.Fatalf("add watcher error: %v", err.Error())
	}

	<-wait
}

func shouldRun(path string, op fsnotify.Op) bool {
	base := filepath.Base(path)
	if op != fsnotify.Write {
		return false
	}
	if ignore != "" && ignoreRegex.MatchString(base) {
		return false
	}
	return true
}

func run() {
	for range startRun {
		st := time.Now()

		ss := strings.Split(command, " ")
		cmd := exec.Command(ss[0], ss[1:]...)
		cmd.Dir = filepath.Dir(filepath.Join(wd, path))
		var stdOut bytes.Buffer
		cmd.Stdout = &stdOut
		if err := cmd.Run(); err != nil {
			log.Fatalf("run command error: %v", err.Error())
			return
		}

		log.Printf("successed gresh: %s, workdir: %s, cost: %s\n", cr.PLBlue(command), cr.PLBlue(cmd.Dir), cr.PLBlue(time.Since(st).String()))
		log.Println(stdOut.String())

		flushEvents()
	}
}

func flushEvents() {
	t := time.NewTicker(time.Duration(interval) * time.Second)
	for {
		select {
		case event := <-startRun:
			_ = event
		case <-t.C:
			return
		}
	}
}

func watch(watcher *fsnotify.Watcher) {
	defer close(wait)

	for {
		select {
		case ev, ok := <-watcher.Events:
			if !ok {
				return
			}
			if !shouldRun(ev.Name, ev.Op) {
				continue
			}
			startRun <- ev.String()
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("have a error: %v", err.Error())
		}
	}
}
