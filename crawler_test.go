package crawler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

const htmlHome = `
<!DOCTYPE html>
<html>
<head></head>
<a href="/">home</a>
<a href="/about">about</a>
<a href="/products">products</a>
<a href="https://google.com">google</a>
<body>
</body>
</html>
`
const htmlAbout = `
<!DOCTYPE html>
<html>
<head></head>
<a href="/">home</a>
<a href="/career">career</a>
<body>
</body>
</html>
`
const htmlCareer = `
<!DOCTYPE html>
<html>
<head></head>
<body>
</body>
</html>
`

func TestParse(t *testing.T) {
	links, err := parse(strings.NewReader(htmlHome))
	if err != nil {
		t.Fatal(err)
	}

	expects := []URL{
		{URI: "/", Description: "home"},
		{URI: "/about", Description: "about"},
		{URI: "/products", Description: "products"},
	}

	if len(links) != len(expects) {
		t.Fatalf("expect sublinks to have length of %d, got %d", len(expects), len(links))
	}

	for i := range expects {
		if expects[i].URI != links[i].URI {
			t.Errorf("expecting link uri to be %s, got %s", expects[i].URI, links[i].URI)
		}
		if expects[i].Description != links[i].Description {
			t.Errorf("expecting link description to be %s, got %s", expects[i].Description, links[i].Description)
		}
	}
}

func newTestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(htmlHome))
	})
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(htmlAbout))
	})
	mux.HandleFunc("/career", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(htmlCareer))
	})
	mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	return httptest.NewServer(mux)
}

func TestCrawl(t *testing.T) {
	server := newTestServer()
	page, err := Crawl(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if page == nil {
		t.Fatal("expect page to be not nil")
	}
	if page.Info.URI != "/" {
		t.Errorf("expect page url to be %s, got %s", "/", page.Info.URI)
	}
	if len(page.Links) != 3 {
		t.Fatalf("expect 3 links got %d", len(page.Links))
	}
	home := page.Links[0]
	if home.Info.URI != "/" {
		t.Errorf("expect home page url to be /, got %s", home.Info.URI)
	}
	about := page.Links[1]
	if about.Info.URI != "/about" {
		t.Errorf("expect about page url to be /about, got %s", about.Info.URI)
	}
	if len(about.Links) != 2 {
		t.Fatalf("expect about page to have 1 link got %d", len(about.Links))
	}
	career := about.Links[1]
	if career.Info.URI != "/career" {
		t.Errorf("expect career page url to be /career, got %s", career.Info.URI)
	}

	products := page.Links[2]
	if products.Info.URI != "/products" {
		t.Errorf("expect products page url to be /products, got %s", products.Info.URI)
	}

	spew.Dump(page)
}
