package crawler

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/net/html"
)

// Verbose is the flag toggle verbose logging
var Verbose bool = true

// Page represents a web page
type Page struct {
	Info  URL
	Links []*Page // links within the page of the link
}

// URL represents the page's metadata
type URL struct {
	URI         string
	Description string
}

// parse reads from r and returns all think from r
func parse(r io.Reader) ([]URL, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse html")
	}

	var links []URL
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			link := URL{}
			for _, a := range n.Attr {
				if a.Key == "href" {
					link.URI = a.Val
					if n.FirstChild != nil {
						link.Description = n.FirstChild.Data
					} else {
						link.Description = a.Val
					}
					break
				}
			}
			// only add internal relative links
			if link.URI != "" &&
				!strings.HasPrefix(link.URI, "http://") &&
				!strings.HasPrefix(link.URI, "https://") {
				links = append(links, link)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return links, nil
}

// Crawl crawls the page from the url link and it's sublinks
func Crawl(urlstring string) (*Page, error) {

	// use a hash map to keep unique links
	var allLinks = make(map[string]*Page)
	var siteRoot *url.URL
	var siteTitle = "site root"

	var crawl func(urlstring, description string) (*Page, error)
	crawl = func(urlstring, description string) (*Page, error) {
		u, err := url.Parse(urlstring)
		if err != nil {
			return nil, err
		}
		// normalise root
		if u.Path == "" {
			u.Path = "/"
		}

		if u.Hostname() != "" {
			root, err := url.Parse("/")
			if err != nil {
				panic(err)
			}
			siteRoot = u.ResolveReference(root)
		} else if siteRoot != nil {
			u = siteRoot.ResolveReference(u)
		} else {
			return nil, errors.New("unable to recognise site root url")
		}

		if allLinks[u.String()] != nil {
			return allLinks[u.String()], nil
		}

		if u.Scheme != "http" && u.Scheme != "https" {
			debugf("!!!unsupported scheme %s at url %s\n", u.Scheme, u.String())
			return nil, nil
		}

		debugf("crawling %s ...\n", u.String())
		resp, err := http.Get(u.String())
		if err != nil {
			return nil, err
		}

		page := &Page{
			Info: URL{URI: u.Path, Description: description},
		}
		allLinks[u.String()] = page

		if resp.StatusCode != 200 {
			debugf("!!!server returned %d for %s\n", resp.StatusCode, u.String())
			page.Info.Description = fmt.Sprintf("%s (%d)", description, resp.StatusCode)
			return page, nil
		}

		urls, err := parse(resp.Body)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var links []*Page
		for _, url := range urls {
			l, err := crawl(url.URI, url.Description)
			if err != nil {
				return nil, err
			}
			if l == nil {
				continue
			}
			links = append(links, l)
		}

		page.Links = links

		return page, nil
	}

	return crawl(urlstring, siteTitle)
}

func debugf(format string, args ...interface{}) {
	if Verbose {
		fmt.Printf(format, args...)
	}
}
