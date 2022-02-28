package henle

import (
	"fmt"
	"github.com/gocolly/colly"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

func setupBookPagesCollectors(c *colly.Collector, c2 *colly.Collector, c3 *colly.Collector, stdout io.Writer) {
	// Before making a request print "Visiting ..."
	c.OnRequest(func(r *colly.Request) {
		fmt.Fprintln(stdout, "c Visiting", r.URL.String())
	})
	c2.OnRequest(func(r *colly.Request) {
		fmt.Fprintln(stdout, "c2 Visiting", r.URL.String())
	})
	c3.OnRequest(func(r *colly.Request) {
		fmt.Fprintln(stdout, "c3 Visiting", r.URL.String())
	})

	// Let 2nd collector visit pageflip associated with HN numbers listed on a search page
	c.OnHTML("article.result-item > div.result-column-left > div.result-content > div.short-facts-container > p.short-facts", func(e *colly.HTMLElement) {
		parts := strings.Split(e.Text, "HN ")
		hn, _ := strconv.Atoi(strings.TrimSpace(parts[len(parts)-1]))
		fmt.Fprintf(stdout, "HN found: %04d\n", hn)
		link := fmt.Sprintf("https://www.henle.de/en/detail/?pageflip=%04d", hn)
		err := c2.Visit(link)
		if err != nil {
			fmt.Fprintf(stdout, "c2.Visiting %s error: %s", link, err)
		}
	})

	// Get numbers of pages of a book
	c2.OnHTML("script", func(e *colly.HTMLElement) {
		//
		//        $(window).load(function(){
		//            Pageflip.init({"pages":0,"pagePath":"\/pageflip\/w500\/0650\/","pagePathZoom":"\/pageflip\/w1500\/0650\/","pageWidth":1000,"pageHeight":null,"pageIgnoreFirst":0,"paginationPrefix":"Page ","paginationFirst":"","paginationSecond":"","paginationNextToLast":"","paginationLast":"","goToPage":"Go to page\u2026","zoomExitHint":"Press ESC to exit Zoom","henleId":"0650","mainTitle":"Alb\u00e9niz, Isaac","subTitle":"Iberia \u00b7 Fourth Book","voices":[]});
		//        });
		//
		for _, s := range strings.Split(e.Text, "{") {
			if strings.HasPrefix(s, "\"pages\":") {
				pages, err := strconv.Atoi(strings.TrimPrefix(strings.Split(s, ",")[0], "\"pages\":"))
				if err != nil {
					fmt.Fprintf(stdout, "c2.pages %s error: %s", e.Request.URL, err)
				}
				if pages == 0 {
					// pages not available
					return
				} else {
					for i := 1; i <= pages; i++ {
						q := e.Request.URL.Query()
						hn := q.Get("pageflip")
						link := fmt.Sprintf("https://www.henle.de/pageflip/w1500/%s/%04d.jpg", hn, i)
						err := c3.Visit(link)
						if err != nil {
							fmt.Fprintf(stdout, "c3.Visiting %s error: %s", link, err)
						}
					}
				}
			}
		}
	})

	// Saves returned book pages
	c3.OnResponse(func(response *colly.Response) {
		path := strings.Split(response.Request.URL.Path, "/")
		hn := path[len(path)-2]
		filename := path[len(path)-1]
		//filename := response.FileName()
		err := os.MkdirAll(fmt.Sprintf("data/henle/%s/w1500", hn), 0666)
		if err != nil {
			fmt.Fprintf(stdout, "os.MkdirAll %s error: %s", fmt.Sprintf("data/henle/%s/w1500", hn), err)
		}
		err = response.Save(fmt.Sprintf("data/henle/%s/w1500/%s", hn, filename))
		if err != nil {
			fmt.Fprintf(stdout, "c3.Save %s error: %s", response.Request.URL, err)
		}
	})
}

func ScrapeBookImages(verbose int, outDir *os.File) {
	var verbout io.Writer
	switch verbose {
	case 0:
		verbout, _ = os.Create("~console-output-*****.log")
	default:
		verbout = os.Stdout
	}

	c := colly.NewCollector(
		colly.AllowedDomains("www.henle.de"),
		colly.CacheDir("./cache"),
	)
	c2 := c.Clone()
	c3 := c.Clone()

	c2.Limit(&colly.LimitRule{
		DomainGlob:  "*henle.*",
		Parallelism: 1, // max collectors running
		//Delay:       2 * time.Second,  // delay between each call. If collectors finish before delay, only parallelism=1.
	})
	c3.Limit(&colly.LimitRule{
		DomainGlob:  "*henle.*",
		Parallelism: 2, // max collectors running
		//Delay:       2 * time.Second,  // delay between each call. If collectors finish before delay, only parallelism=1.
	})

	setupBookPagesCollectors(c, c2, c3, verbout)

	// Start scraping on ...
	// List View
	err := c.Visit("https://www.henle.de/en/search/?Scoring=Keyboard+instruments&Instrument=Piano+solo")

	// Detail View: Normal
	// Visit("https://www.henle.de/en/detail/?Title=Suite+Espagnole+op.+47_783")
	// Detail View: No preview available
	// Visit("https://www.henle.de/en/detail/?Title=Iberia+%C2%B7+Fourth+Book_650")

	if err != nil {
		log.Println("c.Visit error:", err)
	}

	c2.Wait()
	c3.Wait()
}
