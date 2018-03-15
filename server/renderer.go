package server

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/rjeczalik/notify"
	"github.com/shurcooL/github_flavored_markdown"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Renderer - type which renderer md to html files
type Renderer struct {
	address      string                   // address of http-server
	path         string                   // path to md-files
	page         Page                     // page of Content
	updater      chan Page                // channel for sending update information
	relativePath string                   // relativePath in case if server has this option set
	contents     map[string]template.HTML // set of all available pages
}

// Page - type which keep information about the page
type Page struct {
	Sidebar        template.HTML // Sidebar html-Content
	Header         template.HTML // Header html-Content
	Footer         template.HTML // Footer html-Content
	Content        template.HTML // main html-Content of the page
	LastModifiedBy string        // User who modified this repo last time
	LastModifiedAt string        // Date when this repo was modified last time
}

// NewRenderer - create an instance of renderer
func NewRenderer(path string) *Renderer {
	return &Renderer{
		contents:     make(map[string]template.HTML),
		address:      "",
		path:         path,
		updater:      make(chan Page, 100),
		relativePath: "",
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

// GetPage - return page content
func (r *Renderer) GetPage(docPath string) (Page, error) {
	if _, ok := r.contents[docPath]; !ok {
		return Page{}, fmt.Errorf("Can't find the page")
	}

	return Page{
		Header:  r.page.Header,
		Footer:  r.page.Footer,
		Sidebar: r.page.Sidebar,
		Content: r.contents[docPath],
	}, nil
}

// Run - run renderer
func (r *Renderer) Run() {
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
				content, err := r.addContent(apath)
				if err != nil {
					log.Error(err)
				}

				r.contents["/"] = template.HTML(content)
			case "_header.md":
				header, err := r.addContent(apath)
				if err != nil {
					log.Error(err)
				}

				page.Header = template.HTML(header)
			case "_footer.md":
				footer, err := r.addContent(apath)
				if err != nil {
					log.Error(err)
				}

				page.Footer = template.HTML(footer)
			case "_sidebar.md":
				sidebar, err := r.addContent(apath)
				if err != nil {
					log.Error(err)
				}

				page.Sidebar = template.HTML(sidebar)
			default:
				content, err := r.addContent(apath)
				if err != nil {
					log.Error(err)
				}

				r.contents[strings.TrimRight(f.Name(), ".md")] = template.HTML(content)
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

	r.page = page
}
