package server

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
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
	address         string          // address of http-server
	path            string          // path to md-files
	page            CommonPage      // page of Content
	updater         chan CommonPage // channel for sending update information
	relativePath    string          // RelativePath in case if server has this option set
	contents        map[string]Page // set of all available pages
	isMainPageExist bool            // set false in case of no index page: home.md, index.md and README.md
}

// Page - type to keep page-related information
type Page struct {
	Content template.HTML
	Title   string
}

// CommonPage - type to keep information about all pages
type CommonPage struct {
	Sidebar        template.HTML // Sidebar html-Content
	Header         template.HTML // Header html-Content
	Footer         template.HTML // Footer html-Content
	Content        Page          // All page-related content
	LastModifiedBy string        // User who modified this repo last time
	LastModifiedAt string        // Date when this repo was modified last time
	IsCustomCSS    bool          // If doc includes custom css
	IsCustomJS     bool          // If doc includes custom js
	RelativePath   string
}

// NewRenderer - create an instance of renderer
func NewRenderer(path string) *Renderer {
	return &Renderer{
		contents:     make(map[string]Page),
		address:      "",
		path:         path,
		updater:      make(chan CommonPage, 100),
		relativePath: "",
	}
}

// addContent - parse Content from one of main files: home.md, index.md or README.md
func (r *Renderer) addContent(path string) (page Page, err error) {
	var bts []byte
	bts, err = ioutil.ReadFile(path)
	if err != nil {
		return
	}

	bts = github_flavored_markdown.Markdown(bts)
	str := string(bts)

	// stripping .md from urls /Home.md -> /Home
	mdLinkRe := regexp.MustCompile(`<a href="(.*)\.md`)
	matches := mdLinkRe.FindAllStringSubmatch(str, -1)
	for _, row := range matches {
		fmt.Println(row[1])
		str = strings.Replace(str, row[0], fmt.Sprintf(`<a href="%s`, row[1]), -1)
	}

	//<title> tag of the page should be the first H1 in the markdown
	titleLinRe := regexp.MustCompile(`(?Us)(<h1[^>]*>.*</h1>)`)
	matches = titleLinRe.FindAllStringSubmatch(str, -1)

	title := strings.TrimRight(filepath.Base(path), ".md")
	if len(matches) > 0 {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(matches[0][1]))
		if err == nil {
			title = doc.Selection.Text()
		}
	}

	return Page{
		Title:   title,
		Content: template.HTML(str),
	}, nil
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
func (r *Renderer) GetPage(docPath string) (CommonPage, error) {
	if _, ok := r.contents[docPath]; !ok {
		return CommonPage{}, fmt.Errorf("Can't find the page")
	}

	// build page with data
	return CommonPage{
		Header:         r.page.Header,
		Footer:         r.page.Footer,
		Sidebar:        r.page.Sidebar,
		Content:        r.contents[docPath],
		IsCustomCSS:    r.page.IsCustomCSS,
		IsCustomJS:     r.page.IsCustomJS,
		LastModifiedAt: r.page.LastModifiedAt,
		LastModifiedBy: r.page.LastModifiedBy,
	}, nil
}

// Run - run renderer
func (r *Renderer) Run() {
	go r.updateWatcher()

	files, err := ioutil.ReadDir(r.path)
	if err != nil {
		log.Fatal(err)
	}

	r.isMainPageExist = false
	isGitRepo := false
	commonPage := CommonPage{}
	for _, f := range files {
		fmt.Println(f.Name())
		apath, err := filepath.Abs(filepath.Join(r.path, f.Name()))
		if err != nil {
			log.Error(err)
		}

		switch strings.ToLower(f.Name()) {
		case "home.md", "index.md", "README.md":
			page, err := r.addContent(apath)
			if err != nil {
				log.Error(err)
			}

			r.contents["/"] = page
			r.isMainPageExist = true
		case "_header.md":
			header, err := r.addContent(apath)
			if err != nil {
				log.Error(err)
			}

			commonPage.Header = header.Content
		case "_footer.md":
			footer, err := r.addContent(apath)
			if err != nil {
				log.Error(err)
			}

			commonPage.Footer = footer.Content
		case "_sidebar.md":
			sidebar, err := r.addContent(apath)
			if err != nil {
				log.Error(err)
			}

			commonPage.Sidebar = sidebar.Content
		case "custom.css":
			commonPage.IsCustomCSS = true
		case "custom.js":
			commonPage.IsCustomJS = true
		default:
			if filepath.Ext(f.Name()) == ".md" {
				page, err := r.addContent(apath)
				if err != nil {
					log.Error(err)
				}

				r.contents[strings.TrimRight(f.Name(), ".md")] = page
			}
		}

		// check if this dir is git repo
		if f.Name() == ".git" {
			fi, err := os.Stat(apath)
			if err != nil {
				log.Error(err)
				continue
			}

			// if object with name .git is dir
			if fi.IsDir() {
				isGitRepo = true
			}
		}
	}

	if isGitRepo {
		out, err := exec.Command("/usr/bin/git", "--git-dir", filepath.Join(r.path, ".git"), "log").Output()
		if err != nil {
			log.Error(err)
		}

		reAuthor := regexp.MustCompile(`Author: ([^<]*)`)
		dateAuthor := regexp.MustCompile(`Date: ([^\n]*)`)

		rps := reAuthor.FindAllStringSubmatch(string(out), 1)
		author := strings.TrimSpace(rps[0][1])

		rps = dateAuthor.FindAllStringSubmatch(string(out), 1)
		date, err := time.Parse("Mon Jan _2 15:04:05 2006 -0700", strings.TrimSpace(rps[0][1]))
		if err != nil {
			log.Error(err)
		}

		commonPage.LastModifiedBy = author
		commonPage.LastModifiedAt = date.Format("2006-01-02 15:04:05")
	}

	r.page = commonPage
}

// IsMainPageExist - check if main page is exist
func (r *Renderer) IsMainPageExist() bool {
	return r.isMainPageExist
}

func (r *Renderer) GetPages() map[string]string {
	result := make(map[string]string)
	for key, val := range r.contents {
		result[key] = val.Title
	}

	return result
}