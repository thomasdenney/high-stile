package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/shurcooL/github_flavored_markdown"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"time"
)

var serve = flag.Bool("serve", false, "Serve the contents of static after run")

type Page struct {
	Title       string
	Path        string
	Contents    template.HTML
	IsHomepage  bool
	Date        string
	Prev        *Page
	Next        *Page
	ShowHistory bool
}

func (page Page) HasNext() bool {
	return page.Next != nil
}

func (page Page) HasPrev() bool {
	return page.Prev != nil
}

func (page Page) Time() time.Time {
	t, _ := time.Parse("2006-01-02 15:04:05", page.Date)
	return t
}

func (page Page) PostPath() string {
	t := page.Time()
	return fmt.Sprintf("blog/%d/%d/%d/%s", t.Year(), t.Month(), t.Day(), page.Path)
}

func (page Page) PostPath2() string {
	t := page.Time()
	return fmt.Sprintf("blog/%04d/%02d/%02d/%s", t.Year(), t.Month(), t.Day(), page.Path)
}

func (page Page) PrettyDate() string {
	t := page.Time()
	return fmt.Sprintf("%s %d, %d", t.Month().String(), t.Day(), t.Year())
}

func clean() {
	removeErr := os.RemoveAll("static")
	if removeErr != nil {
		panic(removeErr)
	}
	createErr := os.Mkdir("static", 0777)
	if createErr != nil {
		panic(createErr)
	}
}

func linkStaticFiles() {
	files, err := ioutil.ReadDir("public")
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		oldPath := path.Join("public", file.Name())
		newPath := path.Join("static", file.Name())
		linkErr := os.Link(oldPath, newPath)
		if linkErr != nil {
			panic(linkErr)
		} else {
			fmt.Println("Link", oldPath, "to", newPath)
		}
	}
}

func findPages(dir string) []Page {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
	}
	pages := make([]Page, 0)
	for _, file := range files {
		fileName := file.Name()
		if path.Ext(fileName) == ".html" || path.Ext(fileName) == ".md" {
			fullPath := path.Join(dir, fileName)
			justName := fileName[:len(fileName)-len(path.Ext(fileName))]
			contents, err := ioutil.ReadFile(fullPath)

			if err != nil {
				panic(err)
			}

			//Parse Markdown, if necessary
			if path.Ext(fileName) == ".md" {
				contents = github_flavored_markdown.Markdown(contents)
			}

			var page Page

			page.Contents = template.HTML(string(contents))
			page.Path = justName
			page.IsHomepage = false

			jsonPath := path.Join(dir, justName+".json")
			if _, existsErr := os.Stat(jsonPath); existsErr == nil {
				jsonContents, err := ioutil.ReadFile(jsonPath)
				if err != nil {
					panic(err)
				}
				jsonErr := json.Unmarshal(jsonContents, &page)
				if jsonErr != nil {
					panic(jsonErr)
				}
			}

			pages = append(pages, page)
		}
	}

	return pages
}

func makePages(t *template.Template) {
	pages := findPages("pages")
	for _, page := range pages {
		writePage(t, page, page.Path, "Page")
	}
}

func writePage(t *template.Template, page Page, outPath string, kind string) {
	err := os.MkdirAll(path.Join("static", outPath), 0777)
	if err != nil {
		panic(err)
	}
	f, err := os.Create(path.Join("static", outPath, "index.html"))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	t.ExecuteTemplate(f, "page.html", page)
	fmt.Println(kind, page.Title, "created at", outPath)
}

func makeBlog(t *template.Template) {
	posts := findPages("posts")
	replaceDatesInPath := regexp.MustCompile(`([0-9]{4})-([0-9]{2})-([0-9]{2})-`)
	for i, _ := range posts {
		if i > 0 {
			posts[i].Prev = &posts[i-1]
		}
		if i < len(posts)-1 {
			posts[i].Next = &posts[i+1]
		}
		posts[i].ShowHistory = true
		posts[i].Path = replaceDatesInPath.ReplaceAllString(posts[i].Path, "")
	}
	for _, post := range posts {
		var buf bytes.Buffer
		w := bufio.NewWriter(&buf)
		t.ExecuteTemplate(w, "post.html", post)
		w.Flush()
		post.Contents = template.HTML(string(buf.Bytes()))
		writePage(t, post, post.PostPath(), "Post")
		writePage(t, post, post.PostPath2(), "Post")
	}
}

func init() {
	flag.Parse()
}

func main() {
	fmt.Println("High Stile: A static site generator")
	clean()
	linkStaticFiles()
	t, err := template.ParseFiles("templates/page.html", "templates/post.html")
	if err != nil {
		panic(err)
	}

	makePages(t)
	makeBlog(t)

	if *serve {
		fmt.Println("Begin serving on port 8080")
		panic(http.ListenAndServe(":8080", http.FileServer(http.Dir("static"))))
	}
}
