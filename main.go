package main

import (
	"flag"
	"github.com/damonpetta/rowi/renderer"
	"github.com/damonpetta/rowi/server"
)

var address = flag.String("listen", "0.0.0.0:8000", "Server address")
var docroot = flag.String("docroot", "./wiki", "Document root directory")
var relativePath = flag.String("prefix", "", " Url path relativePath")

func main() {
	flag.Parse()

	renderer := renderer.NewRenderer(*address, *relativePath, *docroot)
	go renderer.Run()

	srv := server.NewServer(*address, *relativePath)
	srv.Run()
}
