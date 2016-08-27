package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/shurcooL/github_flavored_markdown"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
)

var serve = flag.Bool("serve", false, "Serve the contents of static after run")

type Page struct {
	Title      string
	Path       string
	Contents   template.HTML
	IsHomepage bool
	Date       string
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
		err := os.MkdirAll(path.Join("static", page.Path), 0777)
		if err != nil {
			panic(err)
		}
		f, err := os.Create(path.Join("static", page.Path, "index.html"))
		if err != nil {
			panic(err)
		}
		defer f.Close()
		t.ExecuteTemplate(f, "page.html", page)
		fmt.Println("Page", page.Title, "created at", page.Path)
	}
}

func init() {
	flag.Parse()
}

func main() {
	fmt.Println("High Stile: A static site generator")
	clean()
	linkStaticFiles()
	t, err := template.ParseFiles("templates/page.html")
	if err != nil {
		panic(err)
	}

	makePages(t)

	if *serve {
		fmt.Println("Begin serving on port 8080")
		panic(http.ListenAndServe(":8080", http.FileServer(http.Dir("static"))))
	}
}
