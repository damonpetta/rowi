package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	log "github.com/Sirupsen/logrus"
	"github.com/rjeczalik/notify"
	"github.com/shurcooL/github_flavored_markdown"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Renderer - type which renderer md to html files
type Renderer struct {
	address         string           // address of http-server
	path            string           // path to md-files
	page            CommonPage       // page of Content
	message         chan interface{} // channel for sending update information
	relativePath    string           // RelativePath in case if server has this option set
	contents        map[string]*Page // set of all available pages
	isMainPageExist bool             // set false in case of no index page: home.md, index.md and README.md
}

// Page - type to keep page-related information
type Page struct {
	Content  template.HTML
	Title    string
	EditLink string
}

// CommonPage - type to keep information about all pages
type CommonPage struct {
	Sidebar        Page   // Sidebar html-Content
	Header         Page   // Header html-Content
	Footer         Page   // Footer html-Content
	Content        *Page  // All page-related content
	LastModifiedBy string // User who modified this repo last time
	LastModifiedAt string // Date when this repo was modified last time
	IsCustomCSS    bool   // If doc includes custom css
	IsCustomJS     bool   // If doc includes custom js
	RelativePath   string
}

type GitLogUser struct {
	Name  string    `json:"name"`
	Email string    `json:"email"`
	Date  time.Time `json:"date"`
}

type GitLog struct {
	Commit               string     `json:"commit"`
	AbbreviatedCommit    string     `json:"abbreviated_commit"`
	Tree                 string     `json:"tree"`
	AbbreviatedTree      string     `json:"abbreviated_tree"`
	Parent               string     `json:"parent"`
	AbbreviatedParent    string     `json:"abbreviated_parent"`
	Refs                 string     `json :"refs"`
	Encoding             string     `json:"encoding"`
	Subject              string     `json:"subject"`
	SanitizedSubjectLine string     `json:"sanitized_subject_line"`
	Body                 string     `json:"body"`
	CommitNotes          string     `json:"commit_notes"`
	VerificationFlag     string     `json:"verification_flag"`
	Signer               string     `json:"signer"`
	SignerKey            string     `json:"signer_key"`
	Author               GitLogUser `json:"author"`
	Commiter             GitLogUser `json:"commiter"`
}

// NewRenderer - create an instance of renderer
func NewRenderer(path string, message chan interface{}) *Renderer {
	return &Renderer{
		contents:     make(map[string]*Page),
		address:      "",
		path:         path,
		message:      message,
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
	editLink := fmt.Sprintf("%s/_edit", title)
	if len(matches) > 0 {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(matches[0][1]))
		if err == nil {
			title = doc.Selection.Text()
		}
	}

	return Page{
		Title:    title,
		Content:  template.HTML(str),
		EditLink: editLink,
	}, nil
}

// updateWatcher - cycle for monitoring changes in filesystem
func (r *Renderer) updateWatcher() {
	dataCh := make(chan notify.EventInfo, 1000)
	notify.Watch(r.path, dataCh, notify.All)
	defer notify.Stop(dataCh)
	var isStop, isData bool

	updateCh := time.NewTicker(time.Second * 5).C
	// monitoring cycle
	for {
		<-updateCh
		isStop = false
		for !isStop {
			select {
			case <-dataCh:
				isData = true
			case <-time.After(time.Millisecond * 100):
				isStop = true
			}
		}

		if isData {
			r.scanStorage()
			r.message <- true
			isData = false
		}
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

	// init data storage
	r.scanStorage()
}

func (r *Renderer) scanStorage() {
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

			r.contents["/"] = &page
			r.isMainPageExist = true
		case "_header.md":
			header, err := r.addContent(apath)
			if err != nil {
				log.Error(err)
			}

			commonPage.Header = header
		case "_footer.md":
			footer, err := r.addContent(apath)
			if err != nil {
				log.Error(err)
			}

			commonPage.Footer = footer
		case "_sidebar.md":
			sidebar, err := r.addContent(apath)
			if err != nil {
				log.Error(err)
			}

			commonPage.Sidebar = sidebar
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

				r.contents[strings.TrimRight(f.Name(), ".md")] = &page
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

		out, err = exec.Command("/usr/bin/git", "--git-dir", filepath.Join(r.path, ".git"), "remote", "get-url", "origin").Output()
		if err != nil {
			log.Error(err)
		}

		editLinkHost := strings.TrimSpace(string(out))
		editLinkHost = strings.Replace(editLinkHost, ".git", "", -1)
		editLinkHost = strings.Replace(editLinkHost, ".wiki", "/wiki", -1)
		fmt.Println(editLinkHost)
		for ind, l := range r.contents {
			if l.EditLink != "" {
				r.contents[ind].EditLink = editLinkHost + "/" + l.EditLink
			}
		}

		if commonPage.Sidebar.EditLink != "" {
			commonPage.Sidebar.EditLink = editLinkHost + "/" + commonPage.Sidebar.EditLink
		}

		if commonPage.Header.EditLink != "" {
			commonPage.Header.EditLink = editLinkHost + "/" + commonPage.Header.EditLink
		}

		if commonPage.Footer.EditLink != "" {
			commonPage.Footer.EditLink = editLinkHost + "/" + commonPage.Footer.EditLink
		}

	} else {
		for _, l := range r.contents {
			l.EditLink = ""
		}
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

func round(f float64) int {
	if f < -0.5 {
		return int(f - 0.5)
	}
	if f > 0.5 {
		return int(f + 0.5)
	}
	return 0
}

// GetHistory - get commit history
func (r *Renderer) GetHistory(limit, skip int) ([]GitLog, int) {
	out, err := exec.Command("/usr/bin/git", "--git-dir", filepath.Join(r.path, ".git"), "rev-list", "--count", "HEAD").Output()
	count, err := strconv.Atoi(strings.TrimSpace(string(out)))

	out, err = exec.Command("/usr/bin/git", "--git-dir", filepath.Join(r.path, ".git"), "log", "--max-count", strconv.Itoa(limit), "--skip", strconv.Itoa(skip), `--pretty=format:{%n  "commit": "%H",%n  "abbreviated_commit": "%h",%n  "tree": "%T",%n  "abbreviated_tree": "%t",%n  "parent": "%P",%n  "abbreviated_parent": "%p",%n  "refs": "%D",%n  "encoding": "%e",%n  "subject": "%s",%n  "sanitized_subject_line": "%f",%n  "body": "%b",%n  "commit_notes": "%N",%n  "verification_flag": "%G?",%n  "signer": "%GS",%n  "signer_key": "%GK",%n  "author": {%n    "name": "%aN",%n    "email": "%aE",%n    "date": "%aI"%n  },%n  "commiter": {%n    "name": "%cN",%n    "email": "%cE",%n    "date": "%cI"%n  }%n}%n`).Output()
	if err != nil {
		log.Error(err)
	}

	results := []GitLog{}
	obj := GitLog{}
	data := bytes.NewReader(out)
	decoder := json.NewDecoder(data)
	for {
		if err := decoder.Decode(&obj); err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}

		results = append(results, obj)
	}

	return results, round(float64(count) / float64(limit))
}

func (r *Renderer) GetDiff(first, second string) string {
	out, err := exec.Command("/usr/bin/git", "--git-dir", filepath.Join(r.path, ".git"), "diff", first, second).Output()
	if err != nil {
		log.Error(err)
	}

	return string(out)
}
