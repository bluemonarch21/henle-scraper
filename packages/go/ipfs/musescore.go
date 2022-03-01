package ipfs

import (
	"archive/zip"
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/gocolly/colly"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func DownloadMuseScore(verbose int, outDir string, msczFilePath string, parallelism int) {
	var stdout io.Writer
	var err error
	switch verbose {
	case 0:
		stdout, err = os.Create("~console-output-msc.log")
		log.Fatal(err)
	default:
		stdout = os.Stdout
	}

	c := colly.NewCollector(
		colly.CacheDir(filepath.Join(outDir, "../cache")),
		// Turn on/off asynchronous requests
		colly.Async(false),
	)

	if err := c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: parallelism, // max collectors running
		//Delay:       2 * time.Second, // delay between each call. If collectors finish before delay, only parallelism=1.
	}); err != nil {
		log.Fatal(err)
	}

	// Before making a request print "Visiting ..."
	c.OnRequest(func(r *colly.Request) {
		fmt.Fprintln(stdout, "c.OnRequest", r.URL.String())
	})

	urlToId := make(map[string]string)

	c.RedirectHandler = func(req *http.Request, via []*http.Request) error {
		var value string
		var ok bool
		for _, r := range via {
			value, ok = urlToId[r.URL.String()]
			if ok {
				urlToId[req.URL.String()] = value
				break
			}
		}
		if !ok {
			return errors.New(fmt.Sprintf("cannot find old URL %v", via))
		}
		return nil
	}

	// Saves returned file
	c.OnResponse(func(response *colly.Response) {
		fmt.Fprintf(stdout, "c.OnResponse %s\n", response.Request.URL.String())
		id, ok := urlToId[response.Request.URL.String()]
		if !ok {
			fmt.Fprintf(stdout, "key not found %s\n", response.Request.URL.String())
			return
		}
		zfp := filepath.Join(outDir, fmt.Sprintf("%s.zip", id))
		if err := response.Save(zfp); err != nil {
			fmt.Fprintf(stdout, "c.Save %s error: %s\n", response.Request.URL, err)
			return
		}
		// Try opening saved file to check validity
		r, err := zip.OpenReader(zfp)
		if err != nil {
			if err := os.MkdirAll(filepath.Join(outDir, "../bad"), 0666); err != nil {
				log.Fatal(err)
			}
			if err := os.Rename(zfp, filepath.Join(outDir, "../bad", fmt.Sprintf("%s-%s.zip", id, time.Now().Format("YYYY-MM-DD-hh-mm-ss")))); err != nil {
				if err := os.Remove(zfp); err != nil {
					log.Fatal(err)
				}
				log.Fatal(err)
			}
		}
		if err := r.Close(); err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(stdout, "Valid %s\n", zfp)
	})

	matches, _ := filepath.Glob(fmt.Sprintf("%s/*.zip", outDir))
	existingIds := make([]string, len(matches))
	for n, match := range matches {
		parts := strings.SplitN(filepath.Base(match), ".", 2)
		i := sort.Search(n, func(i int) bool { return existingIds[i] >= parts[0] })
		existingIds[i] = parts[0]
	}

	urls := make(chan struct {
		url string
		id  string
	}, parallelism)
	done := make(chan bool, 1)
	limit := make(chan bool, parallelism)
	for i := 0; i < parallelism; i++ {
		limit <- true
	}

	msczFile, err := os.Open(msczFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer msczFile.Close()
	reader := csv.NewReader(msczFile)
	_, err = reader.Read() // skip header line
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for {
			records, err := reader.Read()
			if err == nil {
				id := records[0]
				i := sort.SearchStrings(existingIds, id)
				if len(existingIds) > 0 && i != len(existingIds) && existingIds[i] == id {
					continue
				}
				ref := records[1]
				ipns := ref[6:]
				u := fmt.Sprintf("https://ipfs.infura.io/ipfs/%s/", ipns)
				parsedUrl, err := url.Parse(u)
				if err != nil {
					fmt.Fprintln(stdout, "BAD URL", u)
					continue
				}
				u = parsedUrl.String()
				urlToId[u] = id
				urls <- struct {
					url string
					id  string
				}{u, id}
			} else if err == io.EOF {
				break
			} else {
				fmt.Fprintln(stdout, err)
				break
			}
		}
		done <- true
	}()

	go func() {
		for pair := range urls {
			<-limit
			go func(u string, id string) {
				fmt.Fprintf(stdout, "\n[%s]\nc Visiting %s\n", id, u)
				if err := c.Visit(u); err != nil {
					fmt.Fprintf(stdout, "c.Visiting error: %s\n", err)
				}
				limit <- true
			}(pair.url, pair.id)
		}
	}()
	<-done
	c.Wait()
}
