package main

import (
	"encoding/csv"
	"fmt"
	"github.com/gocolly/colly"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Book struct {
	URL             string
	Title           string
	Composer        string
	Authors         []Contributor
	Price           string
	Instrumentation string
	BookInfo        string
	HN              int
	ISMN            string
	Description     string
	Details         []Detail
	CoverLink       string
}

type Contributor struct {
	Name string
	Role string
	URL  string
}

type Detail struct {
	Title           string
	HenleDifficulty string
	ABRSMDifficulty []string
	Section         string
	Composer		string
}

func main() {
	// Create output file
	outFile, err := os.Create("output.csv")
	if err != nil {
		log.Fatal(err)
	}
	defer outFile.Close()

	// Instantiate default collector
	c := colly.NewCollector(
		// Visit only domains: www.henle.de
		colly.AllowedDomains("www.henle.de"),
		//colly.CacheDir("./cache"),
	)

	// Create another collector to scrape book details
	c2 := c.Clone()

	// Channel to collect books
	books := make(chan Book, 10)

	// Limit the number of threads started by colly to two
	// when visiting links which domains' matches "*httpbin.*" glob
	c2.Limit(&colly.LimitRule{
		DomainGlob:  "*henle.*",
		Parallelism: 2,  // max collectors running
		Delay:       3 * time.Second,  // delay between each call. If collectors finish before delay, only parallelism=1.
	})

	// Before making a request print "Visiting ..."
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("c Visiting", r.URL.String())
	})
	c2.OnRequest(func(r *colly.Request) {
		fmt.Println("c2 Visiting", r.URL.String())
	})

	// Let detail collector visit pages linked on book cover from a search page
	c.OnHTML("article.result-item > div.result-column-left > figure.result-cover > a", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		fmt.Printf("Link found: %s\n", link)
		err := c2.Visit(e.Request.AbsoluteURL(link))
		if err != nil {
			fmt.Printf("c2.Visiting %s error: %s", e.Request.AbsoluteURL(link), err)
		}
	})

	// Extract details of the book
	c2.OnHTML(":root", func(e *colly.HTMLElement) {
		title := make(chan string, 1)
		composer := make(chan string, 1)
		price := make(chan string, 1)
		description := make(chan string, 1)
		coverLink := make(chan string, 1)
		instrumentation := make(chan string, 1)
		authors := make(chan []Contributor, 1)
		bookInfo := make(chan string, 1)
		hn := make(chan int, 1)
		ismn := make(chan string, 1)
		// collect 'Book' information
		e.ForEachWithBreak("div.detail-hero", func(i int, e *colly.HTMLElement) bool {
			go func() { title <- e.ChildText("h2.main-title") }()
			go func() { composer <- e.ChildText("h2.sub-title") }()
			go func() { price <- strings.Replace(e.DOM.Find("div.column-cart > p.price").Contents().Not("br,span").Text(), " ", " ", 1) }()
			go func() { description <- strings.Replace(e.ChildText("div.article-text"), "\n", "\\n", -1) }()
			go func() { coverLink <- e.Request.AbsoluteURL(e.ChildAttr("figure.cover-container > a > img", "data-src")) }()
			go func() {
				var inst []string
				e.ForEach("ul.breadcrumb > li", func(i int, e *colly.HTMLElement) {
					inst = append(inst, e.Text)
				})
				instrumentation <- strings.Join(inst, ">")
			}()
			go func() {
				var bookInfos []string
				var contributors []Contributor
				e.ForEach("div.short-facts > p", func(i int, e *colly.HTMLElement) {
					if role := e.ChildText("span.role"); role != "" {
						link := e.ChildAttr("a", "href")
						if link != "" {
							link = e.Request.AbsoluteURL(link)
						} else {
							link = "nil"
						}
						contributors = append(contributors, Contributor{
							strings.TrimSpace(strings.TrimSuffix(e.Text, role)),
							strings.Trim(role, "()"),
							link,
						})
					} else {
						if e.Text != "" {
							bookInfos = append(bookInfos, e.Text)
						}
						if strings.HasPrefix(e.Text, "HN ") {
							tmp := strings.Split(e.Text, "·")
							hn_, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(tmp[0], "HN ")))
							hn <- hn_
							ismn <- strings.TrimPrefix(tmp[1], " ISMN ")
						}
					}
				})
				bookInfo <- strings.Join(bookInfos, "\\n")
				authors <- contributors
			}()
			return false
		})

		detailsChan := make(chan []Detail, 1)
		// collect 'Detail' difficulty information
		e.ForEachWithBreak("div.article-contents", func(i int, e *colly.HTMLElement) bool {
			go func() {
				section := "nil"
				var details []Detail
				e.ForEach("ul", func(i int, e *colly.HTMLElement) {
					// skip table header
					if i == 0 {
						return
					}
					if tmp := e.ChildText("li.column-title > strong"); tmp != "" {
						// section
						section = tmp
						details = append(details, Detail{
							section,
							"nil",
							nil,
							"I am the section",
							"nil",
						})
					} else {
						// title
						var title string
						composer := e.ChildText("li.column-title > em")
						if composer != "" {
							title = e.DOM.Find("li.column-title").Contents().Not("em").Text()
						} else {
							title = e.ChildText("li.column-title")
						}
						instrument := strings.Split(e.ChildText("li.column-difficulty"), " ")[0]
						difficulty :=  instrument + " " + e.ChildText("li.column-difficulty > span.grade-circle")
						var abrsm []string
						e.ForEach("li.column-difficulty > a", func(i int, e *colly.HTMLElement) {
							abrsm = append(abrsm, e.Text)
						})
						details = append(details, Detail{
							title,
							difficulty,
							abrsm,
							section,
							composer,
						})
					}
				})
				detailsChan <- details
			}()
			return false
		})

		book := Book{
			URL:             e.Request.URL.String(),
			Title:           <-title,
			Composer:        <-composer,
			Authors:         <-authors,
			Price:           <-price,
			Instrumentation: <-instrumentation,
			BookInfo:        <-bookInfo,
			HN:              <-hn,
			ISMN:            <-ismn,
			Description:     <-description,
			Details:         <-detailsChan,
			CoverLink:       <-coverLink,
		}
		//fmt.Println("Sending a book")
		books <- book
		//fmt.Println("Sent a book")
	})


	////enc := json.NewEncoder(os.Stdout)
	//enc := json.NewEncoder(outFile)
	//enc.SetIndent("", "  ")
	//enc.Encode(books)

	// Write to CSV whenever new book is added to the books chan
	writer := csv.NewWriter(outFile)
	done := make(chan bool)
	go func() {
		for {
			book, ok := <-books
			if !ok {
				fmt.Println("books closed!")
				done <- true
				return
			}
			//fmt.Println("Got a book?")
			//fmt.Println(book)
			for _, detail := range book.Details {
				row := []string{
					detail.Section,
					detail.Title,
					detail.Composer,
					fmt.Sprint(detail.HenleDifficulty),
					strings.Join(detail.ABRSMDifficulty, "|"),
					book.URL,
					book.Title,
					book.Composer,
					book.Price,
					book.Instrumentation,
					book.BookInfo,
					fmt.Sprint(book.HN),
					book.ISMN,
					book.Description,
					book.CoverLink,
				}
				for _, author := range book.Authors {
					row = append(row, author.Name, author.Role, author.URL)
				}
				//fmt.Println("writing this row:", row)
				err = writer.Write(row)
				if err != nil {
					fmt.Println(err)
				}
				writer.Flush()
			}
		}
	}()


	// Start scraping on ...
	// List View
	err = c.Visit("https://www.henle.de/en/search/?Scoring=Keyboard+instruments&Instrument=Piano+solo")

	// Detail View: Just 1 title
	//err = c2.Visit("https://www.henle.de/en/detail/?Title=Allegro+barbaro_1400")
	// Detail View: Header
	//err = c2.Visit("https://www.henle.de/en/detail/?Title=Chants+d%27Espagne+op.+232_782")
	// Detail View: Header + Hidden Items
	//err = c2.Visit("https://www.henle.de/en/detail/?Title=Selected+Piano+Works_393")
	// Detail View: 2 ABRSM
	//err = c2.Visit("https://www.henle.de/en/detail/?Title=Piano+Sonata+no.+26+E+flat+major+op.+81a+%28Les+Adieux%29_1223")
	// String instruments > Violin and Piano. Content has Authors.
	//err = c2.Visit("https://www.henle.de/en/detail/?Title=Volume+II_353")
	if err != nil {
		fmt.Println("c.Visit error:", err)
	}

	c2.Wait()
	close(books)
	<-done
}
