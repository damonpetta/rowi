package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/gobuffalo/packr"
	"github.com/gorilla/websocket"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	//if s.relativePath == "" {
	//	s.relativePath = "/"
	//}

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

	v1.GET("/all_files", func(c *gin.Context) {
		templateTxt := box.String("all_files.html")

		t1, err := template.New("all_files").Parse(templateTxt)
		if err != nil {
			log.Error(err)
		}

		indexTxt := box.String("index.html")
		t, err := template.New("index").Parse(indexTxt)
		if err != nil {
			log.Error(err)
		}

		page, err := s.renderer.GetPage("/")

		content := bytes.Buffer{}
		bf := bufio.NewWriter(&content)

		pages := s.renderer.GetPages()

		err = t1.ExecuteTemplate(bf, "all_files", pages)
		if err != nil {
			log.Error(err)
		}

		bf.Flush()
		page.Content = Page{Content: template.HTML(content.String())}

		c.Status(http.StatusOK)
		err = t.ExecuteTemplate(c.Writer, "index", page)
		if err != nil {
			log.Error(err)
		}

	})

	r.NoRoute(func(c *gin.Context) {
		if !s.renderer.IsMainPageExist() {
			c.Redirect(http.StatusTemporaryRedirect, filepath.Join(s.relativePath, "all_files"))
			return
		}

		templateTxt := box.String("index.html")
		t, err := template.New("index").Parse(templateTxt)
		if err != nil {
			log.Error(err)
		}

		path := c.Request.URL.Path
		if s.relativePath != "" {
			if strings.Contains(path, s.relativePath) == false {
				c.AbortWithError(http.StatusNotFound, err)
				return
			}

			path = strings.Replace(path, s.relativePath, "", -1)
		}

		page, err := s.renderer.GetPage(filepath.Base(path))
		if err != nil {
			statName := filepath.Base(c.Request.URL.Path)
			stat, err := os.Stat(filepath.Join(s.renderer.path, statName))
			if err == nil && stat.IsDir() == false {
				c.File(filepath.Join(s.renderer.path, statName))
				return
			}

			c.AbortWithError(http.StatusNotFound, err)
			return
		}

		pages := s.renderer.GetPages()

		c.Status(http.StatusOK)
		err = t.ExecuteTemplate(c.Writer, "index", struct {
			Page         CommonPage
			Pages        map[string]string
			RelativePath string
		}{page,
			pages,
			s.relativePath,
		})
		if err != nil {
			log.Error(err)
		}
	})

	return r
}

func (s *Server) sendJSON(conn *websocket.Conn, page CommonPage) error {
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
			err := s.sendJSON(conn, page.(CommonPage))
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

	r.Run(s.address)
}
