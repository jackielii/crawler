package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jackielii/crawler"
)

const usage = `Usage:
	crawler [-v] <url>
`

func main() {
	flag.BoolVar(&crawler.Verbose, "v", false, "verbose logging")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Println(usage)
		os.Exit(1)
	}

	u := flag.Arg(0)
	page, err := crawler.Crawl(u)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to crawl %s: %v", u, err)
		os.Exit(2)
	}

	print(page, 0)
}

var printed = make(map[string]bool)

func print(p *crawler.Page, indent int) {
	fmt.Print(strings.Repeat(" ", indent))
	if printed[p.Info.URI] {
		fmt.Printf("(showed) %s \"%s\"\n", p.Info.URI, p.Info.Description)
		return
	}
	fmt.Printf("%s \"%s\"\n", p.Info.URI, p.Info.Description)
	printed[p.Info.URI] = true

	// skip dup on the same level
	unique := make(map[string]bool)
	for _, p := range p.Links {
		if unique[p.Info.URI] {
			continue
		}
		unique[p.Info.URI] = true
		print(p, indent+2)
	}
}
