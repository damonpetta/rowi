package server

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/gobuffalo/packr"
	"github.com/gorilla/websocket"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Server - type which handler http-requests
type Server struct {
	address      string
	renderer     *Renderer
	message      chan interface{}
	clients      []*websocket.Conn
	relativePath string
}

// NewServer - create new instance a Server instance
func NewServer(address, relativePath, wikiPath string) *Server {
	renderer := NewRenderer(wikiPath)
	renderer.Run()

	return &Server{
		renderer:     renderer,
		address:      address,
		message:      make(chan interface{}, 100),
		relativePath: relativePath,
	}
}

func (s *Server) routes() *gin.Engine {
	box := packr.NewBox("templates/")

	r := gin.Default()

	if s.relativePath == "" {
		s.relativePath = "/"
	}

	v1 := r.Group(s.relativePath)

	// this route uses just to communicate with frontend
	v1.GET("/front", func(c *gin.Context) {
		conn, _ := upgrader.Upgrade(c.Writer, c.Request, nil) // error ignored for sake of simplicity
		s.clients = append(s.clients, conn)
		defer conn.Close()

		log.Printf("Client was added: %v", conn.RemoteAddr())
		page, err := s.renderer.GetPage("/")
		if err != nil {
			log.Error(err)
		}

		err = s.sendJSON(conn, page)
		if err != nil {
			log.Error(err)
		}
	})

	r.NoRoute(func(c *gin.Context) {
		templateTxt := box.String("index.html")

		t, err := template.New("index").Parse(templateTxt)
		if err != nil {
			log.Error(err)
		}

		page, err := s.renderer.GetPage(filepath.Base(c.Request.URL.Path))
		if err != nil {
			statName := filepath.Base(c.Request.URL.Path)
			_, err := os.Stat(filepath.Join(s.renderer.path, statName))
			if err == nil {
				c.File(filepath.Join(s.renderer.path, statName))
				return
			}

			c.AbortWithError(http.StatusNotFound, err)
			return
		}

		err = t.ExecuteTemplate(c.Writer, "index", page)
	})

	return r
}

func (s *Server) sendJSON(conn *websocket.Conn, page Page) error {
	w, err := conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	//disable html-encode
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)

	err1 := encoder.Encode(page)
	w.Close()
	if err1 != nil {
		return err1
	}

	return nil
}

func (s *Server) worker() {
	for {
		page := <-s.message
		for _, conn := range s.clients {
			err := s.sendJSON(conn, page.(Page))
			if err != nil {
				log.Error(err)
			}
		}
	}
}

// Run - run web server
func (s *Server) Run() {
	go s.worker()
	r := s.routes()

	fmt.Println(s.address)
	r.Run(s.address)
}
