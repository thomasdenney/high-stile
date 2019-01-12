package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/feeds"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

var ignoreCache = flag.Bool("ignore-cache", false, "Ignores any cached HTML files, choosing to regenerate them")

var newPostName string

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
	cmd := exec.Command("pandoc", "-f", "markdown", "-t", "html5", markdownPath)
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

type SiteInfo struct {
	Title       string
	Url         string
	Email       string
	Author      string
	Description string
	Avatar      string
}

var site SiteInfo

func readSiteInfo() {
	bs, err := ioutil.ReadFile("config.json")
	if err != nil {
		fmt.Println("No config.json file")
	}
	jsonErr := json.Unmarshal(bs, &site)
	if jsonErr != nil {
		fmt.Println("Failed to parse config.json")
	}
}

type Page struct {
	Title       string
	Path        string
	HeaderPath  string
	Header      template.HTML
	Contents    template.HTML
	IsHomepage  bool
	Date        string
	Prev        *Page
	Next        *Page
	ShowHistory bool
}

type ByDate []Page

func (a ByDate) Len() int           { return len(a) }
func (a ByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByDate) Less(i, j int) bool { return a[i].Time().Before(a[j].Time()) }

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

func (post Page) FeedItem() *feeds.Item {
	return &feeds.Item{
		Title:       post.Title,
		Link:        &feeds.Link{Href: fmt.Sprintf("%s/%s", site.Url, post.PostPath())},
		Description: string(post.Contents),
		Created:     post.Time(),
		Author: &feeds.Author{
			Name:  site.Title,
			Email: site.Email}}
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

func linkFiles(oldDir, newDir string) error {
	err := os.MkdirAll(newDir, os.ModePerm)
	if err != nil {
		return err
	}

	files, err := ioutil.ReadDir(oldDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		oldPath := path.Join(oldDir, file.Name())
		newPath := path.Join(newDir, file.Name())

		if file.IsDir() {
			linkFiles(oldPath, newPath)
		} else {
			err = os.Link(oldPath, newPath)
			if err != nil {
				return err
			} else {
				fmt.Println("Link", oldPath, "to", newPath)
			}
		}
	}

	return nil
}

func linkStaticFiles() {
	linkFiles("public", "static")
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

				// Resolve dependencies
				if page.HeaderPath != "" {
					page.HeaderPath = path.Join(dir, page.HeaderPath)
					headerContents, err := ioutil.ReadFile(page.HeaderPath)
					if err != nil {
						panic(err)
					}
					page.Header = template.HTML(string(headerContents))
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

func makeFeed(posts []Page) {
	feed := &feeds.Feed{
		Title:       site.Title,
		Link:        &feeds.Link{Href: site.Url},
		Description: site.Description,
		Author: &feeds.Author{
			Name:  site.Author,
			Email: site.Email}}
	items := make([]*feeds.Item, 0)
	for i := len(posts) - 1; i >= 0 && i >= len(posts)-10; i-- {
		items = append(items, posts[i].FeedItem())
	}
	feed.Items = items
	rss, err := feed.ToRss()
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(path.Join("static", "feed.xml"), []byte(rss), 0777)
	if err != nil {
		panic(err)
	}
	fmt.Println("Created RSS feed")
}

type JsonFeedAuthor struct {
	Name   string `json:"name"`
	Url    string `json:"url"`
	Avatar string `json:"avatar"`
}

type JsonFeedItem struct {
	Id            string    `json:"id"`
	Url           string    `json:"url"`
	Title         string    `json:"title"`
	ContentHTML   string    `json:"content_html"`
	DatePublished time.Time `json:"date_published"`
}

func (post Page) JsonFeedItem() *JsonFeedItem {
	return &JsonFeedItem{
		Id:            post.PostPath(),
		Title:         post.Title,
		Url:           fmt.Sprintf("%s/%s", site.Url, post.PostPath()),
		ContentHTML:   string(post.Contents),
		DatePublished: post.Time()}
}

type JsonFeed struct {
	Version     string          `json:"version"`
	Title       string          `json:"version"`
	HomePageUrl string          `json:"home_page_url"`
	FeedUrl     string          `json:"feed_url"`
	Items       []*JsonFeedItem `json:"items"`
	Author      *JsonFeedAuthor `json:"author"`
	Description string          `json:"description"`
}

func makeJsonFeed(posts []Page) {
	feed := JsonFeed{
		Version:     "https://jsonfeed.org/version/1",
		Title:       site.Title,
		HomePageUrl: site.Url,
		FeedUrl:     fmt.Sprintf("%s/%s", site.Url, "feed.json"),
		Description: site.Description,
		Author: &JsonFeedAuthor{
			Name:   site.Author,
			Url:    site.Url,
			Avatar: site.Avatar}}
	items := make([]*JsonFeedItem, 0)
	for i := len(posts) - 1; i >= 0 && i >= len(posts)-10; i-- {
		items = append(items, posts[i].JsonFeedItem())
	}
	feed.Items = items
	bs, err := json.Marshal(feed)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(path.Join("static", "feed.json"), []byte(bs), 0777)
	if err != nil {
		panic(err)
	}
	fmt.Println("Made JSON feed")
}

func makeBlog(t *template.Template) {
	posts := findPages("posts")
	sort.Sort(ByDate(posts))
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
	makeFeed(posts)
	makeJsonFeed(posts)
}

type PostMetadata struct {
	Title string
	Date  string
}

func createNewPostFile(title string) {
	d := time.Now()
	meta := PostMetadata{
		Title: title,
		Date:  d.Format("2006-01-02 15:04:05")}

	filename := strings.ToLower(title)
	whitespace := regexp.MustCompile(`\s`)
	filename = whitespace.ReplaceAllString(filename, "-")
	badChars := regexp.MustCompile(`[^\w\d-]*`)
	filename = badChars.ReplaceAllString(filename, "")
	filename = d.Format("2006-01-02") + "-" + filename

	bs, err := json.Marshal(meta)
	if err == nil {
		ioutil.WriteFile(path.Join("posts", filename+".json"), bs, os.ModePerm)
		f, err := os.Create(path.Join("posts", filename+".md"))
		if err == nil {
			defer f.Close()
		}
	}
}

func init() {
	flag.StringVar(&newPostName, "new-post", "", "Creates a new blog post dated now")
	flag.Parse()
}

func main() {
	if len(newPostName) > 0 {
		createNewPostFile(newPostName)
	}
	fmt.Println("High Stile: A static site generator")
	readSiteInfo()
	clean()
	linkStaticFiles()
	t, err := template.ParseFiles("templates/page.html", "templates/post.html", "templates/paginate.html")
	if err != nil {
		panic(err)
	}

	makePages(t)
	makeBlog(t)
}
