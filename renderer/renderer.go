package renderer

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"github.com/shurcooL/github_flavored_markdown"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// Renderer - type which renderer md to html files
type Renderer struct {
	path    string    // path to md-files
	page    Page      // page of Content
	updater chan Page // channel for sending update information
}

// Page - type which keep information about the page
type Page struct {
	Content string //main html-Content of the page
	Sidebar string //Sidebar html-Content
	Header  string //Header html-Content
	Footer  string //Footer html-Content
}

// NewRenderer - create an instance of renderer
func NewRenderer(path string) *Renderer {
	return &Renderer{
		updater: make(chan Page, 100),
		path:    path,
	}
}

// addContent - parse Content from one of main files: home.md, index.md or README.md
func (r *Renderer) addContent(filepath string) (string, error) {
	bts, err := ioutil.ReadFile(filepath)
	if err != nil {
		return string(bts), err
	}

	bts = github_flavored_markdown.Markdown(bts)
	return string(bts), nil
}

func (r *Renderer) notificator() {
	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/source"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	for {
		page := <-r.updater
		log.Println("Run page")

		w, err := c.NextWriter(websocket.TextMessage)
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

		//err := c.WriteJSON(page)
		//if err != nil {
		//	log.Println("write:", err)
		//	return
		//}
	}
}

// Run - run renderer
func (r *Renderer) Run() {
	time.Sleep(time.Second * 30)
	go r.notificator()

	files, err := ioutil.ReadDir(r.path)
	if err != nil {
		log.Fatal(err)
	}

	page := Page{}
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".md" {
			apath, err := filepath.Abs(filepath.Join(r.path, f.Name()))
			if err != nil {
				log.Error(err)
			}

			switch strings.ToLower(f.Name()) {
			case "home.md", "index.md", "README.md":
				page.Content, err = r.addContent(apath)
				if err != nil {
					log.Error(err)
				}
			case "_header.md":
				page.Header, err = r.addContent(apath)
				if err != nil {
					log.Error(err)
				}
			case "_footer.md":
				page.Footer, err = r.addContent(apath)
				if err != nil {
					log.Error(err)
				}
			case "_sidebar.md":
				page.Sidebar, err = r.addContent(apath)
				if err != nil {
					log.Error(err)
				}
			}
		}
	}

	r.updater <- page
}
