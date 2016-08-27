package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"time"
)

var ignoreCache = flag.Bool("ignore-cache", false, "Ignores any cached HTML files, choosing to regenerate them")

func markdownToHTML(markdownPath string) (template.HTML, error) {
	mdStat, _ := os.Stat(markdownPath)
	dir := path.Dir(markdownPath)
	cacheDir := path.Join(dir, "_cache")
	basePath := path.Base(markdownPath)
	basePath = basePath[:len(basePath)-len(path.Ext(basePath))] + ".html"
	cachePath := path.Join(cacheDir, basePath)
	cacheStat, err := os.Stat(cachePath)
	if err == nil && !(*ignoreCache) {
		if mdStat.ModTime().Before(cacheStat.ModTime()) {
			bs, err := ioutil.ReadFile(cachePath)
			return template.HTML(bs), err
		}
	}
	cmd := exec.Command("pandoc", "-f", "markdown", "-t", "html", markdownPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return "", err
	}
	os.MkdirAll(cacheDir, 0777)
	ioutil.WriteFile(cachePath, out.Bytes(), 0777)
	return template.HTML(out.String()), nil
}

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

func (post Page) PostHTML(t *template.Template) template.HTML {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	t.ExecuteTemplate(w, "post.html", post)
	w.Flush()
	return template.HTML(string(buf.Bytes()))
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

			var page Page

			//Parse Markdown, if necessary
			if path.Ext(fileName) == ".md" {
				contents, err := markdownToHTML(fullPath)
				if err != nil {
					panic(err)
				}
				page.Contents = contents
			} else {
				contents, err := ioutil.ReadFile(fullPath)

				if err != nil {
					panic(err)
				}
				page.Contents = template.HTML(string(contents))
			}

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

func writePage(t *template.Template, page Page, outPath string, kind string, links ...string) {
	outPath = path.Join("static", outPath, "index.html")
	err := os.MkdirAll(path.Dir(outPath), 0777)
	if err != nil {
		panic(err)
	}
	f, err := os.Create(outPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	t.ExecuteTemplate(f, "page.html", page)
	fmt.Println(kind, page.Title, "created at", outPath)

	for _, link := range links {
		link = path.Join("static", link, "index.html")
		dir := path.Dir(link)
		os.MkdirAll(dir, 0777)
		os.Link(outPath, link)
		fmt.Println(" >", link)
	}
}

func makeBlogPages(t *template.Template, posts []Page) {
	type PaginatedPage struct {
		Posts              []template.HTML
		HasNewer, HasOlder bool
		Newer, Older       string
	}
	pages := make([]PaginatedPage, 0)
	for i := len(posts) - 1; i >= 0; i-- {
		if (len(pages) == 0) || (len(pages[len(pages)-1].Posts) == 10) {
			pages = append(pages, PaginatedPage{})
		}
		pages[len(pages)-1].Posts = append(pages[len(pages)-1].Posts, posts[i].PostHTML(t))
	}

	for i, _ := range pages {
		if i > 0 {
			pages[i].HasNewer = true
			pages[i].Newer = fmt.Sprintf("/blog/page/%d", i)
		}
		if i < len(pages)-1 {
			pages[i].HasOlder = true
			pages[i].Older = fmt.Sprintf("/blog/page/%d", i+2)
		}
		if i == 1 {
			pages[i].Newer = "/"
		}
	}
	for i, page := range pages {
		var buf bytes.Buffer
		w := bufio.NewWriter(&buf)
		t.ExecuteTemplate(w, "paginate.html", page)
		w.Flush()
		contents := template.HTML(string(buf.Bytes()))
		outPage := Page{
			Title:    fmt.Sprintf("Page %d", i+1),
			Contents: contents,
			Path:     fmt.Sprintf("/blog/page/%d", i+1)}
		if i == 0 {
			outPage.Title = "Blog"
			outPage.IsHomepage = true
			writePage(t, outPage, outPage.Path, "Paginated page", "blog", "")
		} else {
			writePage(t, outPage, outPage.Path, "Paginated page")
		}
	}
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
		posts[i].Path = replaceDatesInPath.ReplaceAllString(posts[i].Path, "")
	}
	for _, post := range posts {
		post.ShowHistory = true
		post.Contents = post.PostHTML(t)
		writePage(t, post, post.PostPath(), "Post", post.PostPath2())
	}
	makeBlogPages(t, posts)
}

func init() {
	flag.Parse()
}

func main() {
	fmt.Println("High Stile: A static site generator")
	clean()
	linkStaticFiles()
	t, err := template.ParseFiles("templates/page.html", "templates/post.html", "templates/paginate.html")
	if err != nil {
		panic(err)
	}

	makePages(t)
	makeBlog(t)
}
