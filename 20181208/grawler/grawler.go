package main

import (
	"fmt"
	"golang.org/x/net/html"
	"net/http"
	"net/url"
	"runtime"
	"sync"
)

type UrlManager struct {
	data map[string]int		// url: (visited, unvisited)
	sync.Mutex
}

const (
	UNVISITED = iota + 1
	VISITED
)

var urlManager = &UrlManager{data: make(map[string]int)}

func (this *UrlManager) addUrl(url string) {
	this.Lock()
	defer this.Unlock()
	if checkUrlValidation(url) && this.data[url] == 0 {
		this.data[url] = UNVISITED
	}
}

func (this *UrlManager) markAsVisited(url string) {
	this.Lock()
	defer this.Unlock()

	if this.data[url] == UNVISITED {
		this.data[url] = VISITED
	}
}

func (this *UrlManager) isVisited(url string) bool {
	this.Lock()
	defer this.Unlock()
	return this.data[url] == VISITED
}

func (this *UrlManager) printAll() {
	this.Lock()
	defer this.Unlock()

	for k, v := range this.data {
		fmt.Printf("url: %s [%s]\n", k, func(v int) string {
			if v == VISITED {
				return "VISITED"
			}
			return "UNVISITED"
		}(v))
	}
}

func fetch(url string) (*html.Node, error) {
	res, err := http.Get(url)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	doc, err := html.Parse(res.Body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return doc, nil
}

func parse(doc *html.Node, urls chan string) {
	go func() {
		// declaration
		var p func (node *html.Node)	// declare type of function
		p = func(node *html.Node) {
			if node.Type == html.ElementNode && node.Data == "a" {
				for _, attr := range node.Attr {
					if attr.Key == "href" && checkUrlValidation(attr.Val) {
						urlManager.addUrl(attr.Val)
						urls <- attr.Val		// send to urls channel
					}
				}
			}
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				p(c)
			}
		}
		// declaration
		p(doc)
	}()
}

func crawl(url string, urls chan string, result chan<- string) {
	if urlManager.isVisited(url) {
		return
	}

	doc, err := fetch(url)
	if err != nil {
		go func(u string) {
			urls <- u
		}(url)
		return
	}

	urlManager.markAsVisited(url)
	parse(doc, urls)
	result <- fmt.Sprintf("Visited: %s", url)
}

func worker(n int, done <-chan struct{}, urls chan string, result chan<- string) {
	fmt.Printf("Worker[%d] generated\n", n)

	for u := range urls {
		select {
		case <- done:
			return
		default:
			crawl(u, urls, result)
		}
	}
}

func main() {
	nCPUs := runtime.NumCPU()
	runtime.GOMAXPROCS(nCPUs)

	chJobQueue := make(chan string)
	sigKill := make(chan struct{})
	chResult := make(chan string)

	var wg sync.WaitGroup
	numberOfWorkers := nCPUs * 2
	wg.Add(numberOfWorkers)

	for i := 0; i < numberOfWorkers; i++ {
		go func(n int) {
			worker(n + 1, sigKill, chJobQueue, chResult)
			wg.Done()
		}(i)
	}

	go func() {
		wg.Wait()
		close(chResult)
	}()

	chJobQueue <- "https://www.naver.com"
	for r := range chResult {
		fmt.Println(r)
	}
}

func checkUrlValidation(rawUrl string) bool {
	u, err := url.Parse(rawUrl)
	if err != nil {
		return false
	}
	return len(u.Host) > 0
}