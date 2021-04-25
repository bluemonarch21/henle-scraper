package main

import (
	"alignment/henle"
	"log"
	"os"
)

func scrapeToFile() {
	// Create output file
	outFile, err := os.Create("output.csv")
	if err != nil {
		log.Fatal(err)
	}
	defer outFile.Close()

	henle.Scrape(outFile, "csv", 1)
}

func scrapeToStdout() {
	henle.Scrape(os.Stdout, "json", 0)
}

func main() {

}
