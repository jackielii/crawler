package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/net/html"
)

// Verbose is the flag toggle verbose logging
var Verbose bool

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
	var allLinksLock sync.RWMutex // protect all links read & write
	var siteRoot *url.URL
	var siteTitle = "site root"

	var crawl func(ctx context.Context, urlstring, description string) (*Page, error)
	crawl = func(ctx context.Context, urlstring, description string) (*Page, error) {
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

		allLinksLock.RLock()
		existing := allLinks[u.String()]
		allLinksLock.RUnlock()

		if existing != nil {
			return existing, nil
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
		allLinksLock.Lock()
		allLinks[u.String()] = page
		allLinksLock.Unlock()

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

		wg := &sync.WaitGroup{}
		pc := make(chan *Page)
		errs := make(chan error)

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		for _, u := range urls {
			wg.Add(1)
			go func(u URL) {
				defer wg.Done()
				l, err := crawl(ctx, u.URI, u.Description)
				if err != nil {
					errs <- err
				} else {
					pc <- l
				}
			}(u)
		}
		go func() {
			wg.Wait()
			close(errs)
		}()

		var links []*Page
	loop:
		for {
			select {
			case page := <-pc:
				links = append(links, page)
			case err := <-errs:
				if err != nil {
					cancel()
					return nil, err
				}
				break loop
			case <-ctx.Done():
				if ctx.Err() == context.Canceled {
					break loop
				} else {
					return nil, ctx.Err()
				}
			}
		}

		page.Links = links

		return page, nil
	}

	return crawl(context.Background(), urlstring, siteTitle)
}

func debugf(format string, args ...interface{}) {
	if Verbose {
		fmt.Printf(format, args...)
	}
}
