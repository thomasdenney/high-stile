package main

import (
	"encoding/json"
	"fmt"
	"github.com/russross/blackfriday"
	"html/template"
	"io/ioutil"
	"os"
	"path"
)

type Page struct {
	Title      string
	Path       string
	Contents   template.HTML
	IsHomepage bool
}

type PageMetadata struct {
	Title string
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

func makePages(t *template.Template) {
	files, err := ioutil.ReadDir("pages")
	if err != nil {
		panic(err)
	}
	pages := make([]Page, 0)
	for _, file := range files {
		fileName := file.Name()
		if path.Ext(fileName) == ".html" || path.Ext(fileName) == ".md" {
			fullPath := path.Join("pages", fileName)
			justName := fileName[:len(fileName)-len(path.Ext(fileName))]
			contents, err := ioutil.ReadFile(fullPath)

			if err != nil {
				panic(err)
			}

			//Parse Markdown, if necessary
			if path.Ext(fileName) == ".md" {
				contents = blackfriday.MarkdownBasic(contents)
			}

			page := Page{
				Path:       justName,
				Contents:   template.HTML(string(contents)),
				IsHomepage: false}

			jsonPath := path.Join("pages", justName+".json")
			if _, existsErr := os.Stat(jsonPath); existsErr == nil {
				var metadata PageMetadata
				jsonContents, err := ioutil.ReadFile(jsonPath)
				if err != nil {
					panic(err)
				}
				jsonErr := json.Unmarshal(jsonContents, &metadata)
				if jsonErr == nil {
					page.Title = metadata.Title
				}
			}

			pages = append(pages, page)
		}
	}

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

func main() {
	fmt.Println("High Stile: A static site generator")
	clean()
	linkStaticFiles()
	t, err := template.ParseFiles("templates/page.html")
	if err != nil {
		panic(err)
	}

	makePages(t)

}
