package renderer

import (
	log "github.com/Sirupsen/logrus"
	"github.com/shurcooL/github_flavored_markdown"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// Renderer - type which renderer md to html files
type Renderer struct {
	path string //path to md-files
	page Page   //page of Content
}

// Page - type which keep information about the page
type Page struct {
	Content []byte //main html-Content of the page
	Sidebar []byte //Sidebar html-Content
	Header  []byte //Header html-Content
	Footer  []byte //Footer html-Content
}

// NewRenderer - create an instance of renderer
func NewRenderer(path string) *Renderer {
	return &Renderer{
		path: path,
	}
}

// addMainContent - parse Content from one of main files: home.md, index.md or README.md
func (r *Renderer) addMainContent(filepath string) ([]byte, error) {
	var result []byte
	bts, err := ioutil.ReadFile(filepath)
	if err != nil {
		return result, err
	}

	result = github_flavored_markdown.Markdown(bts)
	return result, nil
}

// Run - run renderer
func (r *Renderer) Run() {
	files, err := ioutil.ReadDir(r.path)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) == ".md" {
			apath, err := filepath.Abs(filepath.Join(r.path, f.Name()))
			if err != nil {
				log.Error(err)
			}

			switch strings.ToLower(f.Name()) {
			case "home.md", "index.md", "README.md":
				r.page.Content, err = r.addMainContent(apath)
				if err != nil {
					log.Error(err)
				}
			case "_header.md":
				r.page.Header, err = r.addMainContent(apath)
				if err != nil {
					log.Error(err)
				}
			case "_footer.md":
				r.page.Footer, err = r.addMainContent(apath)
				if err != nil {
					log.Error(err)
				}
			case "_sidebar.md":
				r.page.Sidebar, err = r.addMainContent(apath)
				if err != nil {
					log.Error(err)
				}
			}
		}
	}
}
