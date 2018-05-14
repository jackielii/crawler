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

// QueueSize is the size of the queue to fetch urls concurrently
var QueueSize = 100
var globalTaskQueue = make(chan struct{}, QueueSize)

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
func parse(u *url.URL, r io.Reader) ([]URL, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse html")
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}

	var links []URL
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			link := URL{}
			for _, a := range n.Attr {
				if a.Key == "href" {
					u1, err := url.Parse(a.Val)
					if err != nil {
						panic(err)
					}

					if u1.Hostname() != "" && u1.Hostname() != u.Hostname() {
						continue
					}

					link.URI = u.ResolveReference(u1).Path
					if n.FirstChild != nil {
						link.Description = sanitise(n.FirstChild.Data)
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
	var allLinksLock sync.Mutex // protect all links read & write
	var siteRoot *url.URL
	var siteTitle = urlstring

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

		// the key value to test uniqueness
		key := sanitise(u.Path)

		if u.Scheme != "http" && u.Scheme != "https" {
			debugf("!!!unsupported scheme %s at url %s\n", u.Scheme, u.String())
			return nil, nil
		}

		allLinksLock.Lock()
		existing := allLinks[key]

		if existing != nil {
			allLinksLock.Unlock()
			return existing, nil
		}
		page := &Page{
			Info: URL{URI: key, Description: description},
		}
		allLinks[key] = page
		allLinksLock.Unlock()

		debugf("crawling %s ...\n", u.String())
		resp, err := doGet(u.String())
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != 200 {
			debugf("!!!server returned %d for %s\n", resp.StatusCode, u.String())
			page.Info.Description = fmt.Sprintf("%s (%d)", description, resp.StatusCode)
			return page, nil
		}

		urls, err := parse(u, resp.Body)
		if err != nil {
			return nil, err
		}
		resp.Body.Close()

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
				if page != nil {
					links = append(links, page)
				}
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

	ctx := context.Background()

	return crawl(ctx, urlstring, siteTitle)
}

func doGet(u string) (*http.Response, error) {
	globalTaskQueue <- struct{}{}
	defer func() { <-globalTaskQueue }()
	return http.Get(u)
}

func debugf(format string, args ...interface{}) {
	if Verbose {
		fmt.Printf(format, args...)
	}
}

func sanitise(s string) string {
	s = strings.Replace(s, "\n", "", -1)
	s = strings.Replace(s, " ", "", -1)
	return s
}
