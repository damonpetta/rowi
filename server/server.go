package server

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rowi/renderer"
	"time"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Server - type which handler http-requests
type Server struct {
	message chan []byte
	clients []*websocket.Conn
}

// NewServer - create new instance a Server instance
func NewServer() *Server {
	return &Server{
		message: make(chan []byte, 100),
	}
}

func (s *Server) routes() *gin.Engine {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.GET("/ws", func(c *gin.Context) {
		conn, _ := upgrader.Upgrade(c.Writer, c.Request, nil) // error ignored for sake of simplicity
		s.clients = append(s.clients, conn)
		defer conn.Close()

		for {
			// Read message from browser
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}

			fmt.Println(msgType)
			// Print the message to the console
			fmt.Printf("%s sent: %s\n", conn.RemoteAddr(), string(msg))

			s.message <- msg
		}
	})

	// ToDo: built-in templates file!!!
	r.StaticFile("/", "./server/templates/index.html")

	//r.Use(static.Serve("/templates))

	return r
}

func (s *Server) worker() {
	for bts := range <-s.message {
		fmt.Println(string(bts))
	}
}

func (s *Server) updater() {
	for {
		for _, conn := range s.clients {
			conn.WriteJSON(renderer.Page{})
		}

		time.Sleep(time.Second * 5)
	}
}

// Run - run web server
func (s *Server) Run() {
	go s.updater()
	go s.worker()
	r := s.routes()
	r.Run()
}
