package server

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/gobuffalo/packr"
	"github.com/gorilla/websocket"
	"github.com/rowi/renderer"
	"net/http"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Server - type which handler http-requests
type Server struct {
	message chan interface{}
	clients []*websocket.Conn
}

// NewServer - create new instance a Server instance
func NewServer() *Server {
	return &Server{
		message: make(chan interface{}, 100),
	}
}

func (s *Server) routes() *gin.Engine {
	box := packr.NewBox("templates/")

	r := gin.Default()

	// this route uses just to communicate with datasource (renderer)
	r.GET("/source", func(c *gin.Context) {
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

			log.Println("Message read")
			s.message <- page
		}
	})

	// this route uses just to communicate with frontend
	r.GET("/front", func(c *gin.Context) {
		conn, _ := upgrader.Upgrade(c.Writer, c.Request, nil) // error ignored for sake of simplicity
		s.clients = append(s.clients, conn)
		defer conn.Close()

		log.Printf("Client was added: %v", conn.RemoteAddr())
		for {
			// Read message from browser
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})

	r.GET("/", gin.WrapH(http.FileServer(box)))
	return r
}

func (s *Server) worker() {
	for {
		page := <-s.message
		for _, conn := range s.clients {
			w, err := conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Error(err)
			}

			//disable html-encode
			encoder := json.NewEncoder(w)
			encoder.SetEscapeHTML(false)

			err1 := encoder.Encode(page)
			w.Close()
			if err1 != nil {
				log.Error(err1)
			}
		}
	}
}

// Run - run web server
func (s *Server) Run() {
	go s.worker()
	r := s.routes()
	r.Run()
}
