package main

import (
	"flag"
	"net/http"
	"os"
	"os/signal"

	"golang.org/x/net/webdav"
)

var (
	port string
	path string
)

func init() {
	flag.StringVar(&port, "port", ":8080", "port to listen on")
	flag.StringVar(&path, "path", ".", "path to serve")
}

func main() {
	go signalExit()

	flag.Parse()

	http.ListenAndServe(port, &webdav.Handler{
		FileSystem: webdav.Dir(path),
		LockSystem: webdav.NewMemLS(),
	})
}

func signalExit() {
	c := make(chan os.Signal, 1)
	signal.Notify(c)
	<-c

	os.Exit(0)
}
