[![Go Report Card](https://goreportcard.com/badge/github.com/jackielii/crawler)](https://goreportcard.com/report/github.com/jackielii/crawler)

# crawler

A simple web crawler limited to one domain. E.g. when you start with https://monzo.com/, it would crawl all pages within monzo.com, but not follow external links, for example to the Facebook and Twitter accounts. Given a URL, it should print a simple site map, showing the links between pages.

## install

```bash
go get github.com/jackielii/crawler/cmd/crawler
```

## run

```bash
crawler https://monzo.com > monzo.txt
```

You can use `-v` to turn on a bit logging
