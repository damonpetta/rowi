package renderer

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"github.com/rjeczalik/notify"
	"github.com/shurcooL/github_flavored_markdown"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Renderer - type which renderer md to html files
type Renderer struct {
	address      string    // address of http-server
	path         string    // path to md-files
	page         Page      // page of Content
	updater      chan Page // channel for sending update information
	relativePath string    // relativePath in case if server has this option set
}

// Page - type which keep information about the page
type Page struct {
	Content        string // main html-Content of the page
	Sidebar        string // Sidebar html-Content
	Header         string // Header html-Content
	Footer         string // Footer html-Content
	LastModifiedBy string // User who modified this repo last time
	LastModifiedAt string // Date when this repo was modified last time
}

// NewRenderer - create an instance of renderer
func NewRenderer(address, relativePath, path string) *Renderer {
	return &Renderer{
		address:      address,
		path:         path,
		updater:      make(chan Page, 100),
		relativePath: relativePath,
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
	u := url.URL{Scheme: "ws", Host: r.address, Path: filepath.Join(r.relativePath, "source")}
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
	}
}

// updateWatcher - cycle for monitoring changes in filesystem
func (r *Renderer) updateWatcher() {
	ch := make(chan notify.EventInfo, 1000)
	notify.Watch(r.path, ch, notify.All)
	defer notify.Stop(ch)

	// monitoring cycle
	for {
		ei := <-ch
		log.Println("Got event:", ei)
	}
}

// Run - run renderer
func (r *Renderer) Run() {
	go r.notificator()
	go r.updateWatcher()

	files, err := ioutil.ReadDir(r.path)
	if err != nil {
		log.Fatal(err)
	}

	isGitRepo := false
	page := Page{}
	for _, f := range files {
		fmt.Println(f.Name())
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

		// check if this dir is git repo
		if f.Name() == ".git" {
			fi, err := os.Stat(f.Name())
			if err != nil {
				log.Error(err)
			}

			// if object with name .git is dir
			if fi.IsDir() {
				isGitRepo = true
			}
		}
	}

	if isGitRepo {
		out, err := exec.Command("/usr/local/bin/git", "log").Output()
		if err != nil {
			log.Fatal(err)
		}

		reAuthor := regexp.MustCompile(`Author: ([^<]*)`)
		dateAuthor := regexp.MustCompile(`Date: ([^\n]*)`)
		//fmt.Println(string(out))

		rps := reAuthor.FindAllStringSubmatch(string(out), 1)
		author := strings.TrimSpace(rps[0][1])

		rps = dateAuthor.FindAllStringSubmatch(string(out), 1)
		date, err := time.Parse("Mon Jan _2 15:04:05 2006 -0700", strings.TrimSpace(rps[0][1]))
		if err != nil {
			log.Error(err)
		}

		page.LastModifiedBy = author
		page.LastModifiedAt = date.Format("2006-01-02 15:04:05")
	}

	r.updater <- page
}
