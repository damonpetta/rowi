package main

import (
	"github.com/rowi/renderer"
	server "github.com/rowi/server"
)

func main() {
	renderer := renderer.NewRenderer("../rowi.wiki")
	go renderer.Run()

	srv := server.NewServer()
	srv.Run()
}
