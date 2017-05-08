package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

var crawls sync.WaitGroup
var downloads sync.WaitGroup

func crawlPage(url string, ch chan string, sem chan bool) {
	sem <- true

	defer func() {
		crawls.Done()
		<-sem
	}()

	fmt.Printf("Fetching Page: %v \n", url)
	resp, err := goquery.NewDocument(url)

	if err != nil {
		fmt.Printf("ERROR: Failed to crawl \"" + url + "\"\n")
		return
	}

	// select each image, extract src
	resp.Find("img").Each(func(index int, item *goquery.Selection) {
		link, _ := item.Attr("src")

		if link != "" {
			fmt.Printf("Found: %v \n", link)
			ch <- link
		}
	})
}

func crawlPages(urls []string, imageUrls chan string) {
	concurrency := 5
	sem := make(chan bool, concurrency)

	// Crawl process (concurrently)
	for _, url := range urls {
		if url[:4] != "http" {
			url = "http://" + url
		}
		crawls.Add(1)
		go crawlPage(url, imageUrls, sem)
	}
}

// takes a channel of incoming URLs and outputs a channel of unique URLs
func ensureUnique(in chan string, out chan string) {
	allUrls := make(map[string]bool)

	go func() {
		for url := range in {
			if !allUrls[url] {
				allUrls[url] = true
				out <- url
			}
		}
		// close 'out' once 'in' closes and the last url is pushed onto 'out'
		close(out)
	}()
}

func downloadImage(url string, folder string, sem chan bool) {
	os.Mkdir(folder, os.FileMode(0777))

	sem <- true
	defer func() {
		downloads.Done()
		<-sem
	}()

	if url[:4] != "http" {
		url = "http:" + url
	}
	parts := strings.Split(url, "/")
	name := parts[len(parts)-1]
	file, err := os.Create(string(folder + "/" + name))
	if err != nil {
		fmt.Printf("%v \n", err)
		return
	}
	defer file.Close()
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("%v \n", err)
		return
	}
	defer resp.Body.Close()

	io.Copy(file, resp.Body)

	fmt.Printf("Saving %s \n", folder+"/"+name)
}

func downloadImages(in chan string, Folder string) {

	concurrency := 5
	sem := make(chan bool, concurrency)

	go func() {
		for ui := range in {
			downloads.Add(1)
			go downloadImage(ui, Folder, sem)
		}
	}()
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("ERROR : Less Args\nCommand should be of type : imagescraper [folder to save] [websites]\n\n")
		os.Exit(3)
	}

	Folder := os.Args[1]
	seedUrls := os.Args[2:]

	imageUrls := make(chan string)
	uniqueImgUrls := make(chan string)

	// Crawl websites and push image urls onto the imageUrls channel
	crawlPages(seedUrls, imageUrls)

	// Ingest urls from imageUrls channel and output unique images onto uniqueImageUrls channel
	ensureUnique(imageUrls, uniqueImgUrls)

	// Ingest urls from uniqueImageUrls channel, download and write into Folder
	downloadImages(uniqueImgUrls, Folder)

	// inserts waitgroup is incremented by the Crawl
	crawls.Wait()
	close(imageUrls)

	downloads.Wait()
	fmt.Printf("\n\nScraped succesfully\n\n")

}
