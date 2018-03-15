package server

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/damonpetta/rowi/renderer"
	"github.com/gin-gonic/gin"
	"github.com/gobuffalo/packr"
	"github.com/gorilla/websocket"
	"net/http"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Server - type which handler http-requests
type Server struct {
	address      string
	page         renderer.Page
	message      chan interface{}
	clients      []*websocket.Conn
	relativePath string
}

// NewServer - create new instance a Server instance
func NewServer(address, relativePath string) *Server {
	return &Server{
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

	// this route uses just to communicate with datasource (renderer)
	v1.GET("/source", func(c *gin.Context) {
		conn, _ := upgrader.Upgrade(c.Writer, c.Request, nil) // error ignored for sake of simplicity
		s.clients = append(s.clients, conn)
		defer conn.Close()

		for {
			page := renderer.Page{}
			// Read message from browser
			err := conn.ReadJSON(&page)
			if err != nil {
				return
			}

			s.message <- page
			s.page = page
		}
	})

	// this route uses just to communicate with frontend
	v1.GET("/front", func(c *gin.Context) {
		conn, _ := upgrader.Upgrade(c.Writer, c.Request, nil) // error ignored for sake of simplicity
		s.clients = append(s.clients, conn)
		defer conn.Close()

		log.Printf("Client was added: %v", conn.RemoteAddr())
		err := s.sendJSON(conn, s.page)
		if err != nil {
			log.Error(err)
		}
	})

	v1.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", box.Bytes("index.html"))
	})

	return r
}

func (s *Server) sendJSON(conn *websocket.Conn, page renderer.Page) error {
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
			err := s.sendJSON(conn, page.(renderer.Page))
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
