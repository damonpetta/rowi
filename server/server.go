package server

import (
	"bufio"
	"bytes"
	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/gobuffalo/packr"
	"github.com/gorilla/websocket"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
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
	clients      map[*websocket.Conn]string
	clientsMX    sync.Mutex
	relativePath string
}

// FrontData - type which keep info about frontend
type FrontData struct {
	Url string `json:"url"`
}

// NewServer - create new instance a Server instance
func NewServer(address, relativePath, wikiPath string) *Server {
	message := make(chan interface{}, 100)
	renderer := NewRenderer(wikiPath, message)
	renderer.Run()

	return &Server{
		renderer:     renderer,
		address:      address,
		clients:      make(map[*websocket.Conn]string),
		message:      message,
		relativePath: relativePath,
	}
}

func (s *Server) routes() *gin.Engine {
	box := packr.NewBox("templates/")

	r := gin.Default()

	v1 := r.Group(s.relativePath)

	// this route uses just to communicate with frontend
	v1.GET("/front", func(c *gin.Context) {
		conn, _ := upgrader.Upgrade(c.Writer, c.Request, nil) // error ignored for sake of simplicity
		log.Printf("Client was added: %v", conn.RemoteAddr())

		frontRequest := FrontData{}
		err := conn.ReadJSON(&frontRequest)
		if err != nil {
			log.Error(err)
		}

		s.clients[conn] = frontRequest.Url
	})

	v1.GET("/history/:first/:second", func(c *gin.Context) {
		templateTxt := box.String("compare.html")

		t, err := template.New("compare").Parse(templateTxt)
		if err != nil {
			log.Error(err)
		}

		styles := box.String("styles.html")

		history, count := s.renderer.GetHistory(2, 0)
		c.Status(http.StatusOK)
		err = t.ExecuteTemplate(c.Writer, "compare", struct {
			Styles       template.HTML
			Diff         string
			Commits      []GitLog
			Count        int
			RelativePath string
		}{
			template.HTML(styles),
			s.renderer.GetDiff(c.Param("first"), c.Param("second")),
			history,
			count,
			s.relativePath,
		})
		if err != nil {
			log.Error(err)
		}
	})

	v1.GET("/history", func(c *gin.Context) {
		templateTxt := box.String("history.html")
		pageParam := c.Query("page")
		limitParam := c.Query("limit")

		limit, _ := strconv.Atoi(limitParam)
		if limit == 0 {
			limit = 10
		}

		page, _ := strconv.Atoi(pageParam)
		if page == 0 {
			page = 1
		}

		t, err := template.New("history").Parse(templateTxt)
		if err != nil {
			log.Error(err)
		}

		styles := box.String("styles.html")

		history, count := s.renderer.GetHistory(limit, (page-1)*limit)

		pages := []int{}
		rightCount := 2
		leftCount := 2

		if count-page < 2 {
			leftCount += 2 - (count - page)
			rightCount -= 2 - (count - page)
		}

		if page < 3 {
			rightCount += 2 - (page - 1)
			leftCount -= 2 - (page - 1)
		}

		for i := leftCount; i > 0; i-- {
			if page-i > 0 {
				pages = append(pages, page-i)
			}
		}

		pages = append(pages, page)

		for i := 0; i < rightCount; i++ {
			if page+i+1 <= count {
				pages = append(pages, page+i+1)
			}
		}

		c.Status(http.StatusOK)
		err = t.ExecuteTemplate(c.Writer, "history", struct {
			Commits      []GitLog
			Count        int
			Styles       template.HTML
			RelativePath string
			Page         int
			PrevPage     int
			NextPage     int
			Limit        int
			Pages        []int
		}{
			history,
			count,
			template.HTML(styles),
			s.relativePath,
			page,
			page - 1,
			page + 1,
			limit,
			pages,
		})
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
		page.Content = &Page{Content: template.HTML(content.String())}

		styles := box.String("styles.html")

		c.Status(http.StatusOK)
		err = t.ExecuteTemplate(c.Writer, "index", struct {
			Page         CommonPage
			Pages        map[string]string
			RelativePath string
			Styles       template.HTML
		}{page,
			pages,
			s.relativePath,
			template.HTML(styles),
		})
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

		styles := box.String("styles.html")

		c.Status(http.StatusOK)
		err = t.ExecuteTemplate(c.Writer, "index", struct {
			Page         CommonPage
			Pages        map[string]string
			RelativePath string
			Styles       template.HTML
		}{page,
			pages,
			s.relativePath,
			template.HTML(styles),
		})
		if err != nil {
			log.Error(err)
		}
	})

	return r
}

func (s *Server) worker() {
	for {
		<-s.message
		s.clientsMX.Lock()
		for conn, _ := range s.clients {
			err := conn.WriteJSON(gin.H{"test-message": "true"})
			if err != nil {
				log.Error(err)
			}
		}
		s.clientsMX.Unlock()
	}
}

func (s *Server) keepAliveWatcher() {
	updater := time.NewTicker(time.Second * 2).C
	for {
		<-updater
		s.clientsMX.Lock()
		for conn, _ := range s.clients {
			err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(time.Millisecond*100))
			if err != nil {
				delete(s.clients, conn)
				log.Printf("Client was deleted: %v", conn.RemoteAddr())
			}
		}
		s.clientsMX.Unlock()
	}
}

// Run - run web server
func (s *Server) Run() {
	go s.worker()
	go s.keepAliveWatcher()
	r := s.routes()

	r.Run(s.address)
}
