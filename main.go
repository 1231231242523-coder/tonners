package main

import (
	"flag"
	"fmt"
	"os"

	"eurooffice/suppliers/officesolution"
)

func main() {
	categoryURL := flag.String("url", "https://officesolution.al/en/11-inks-and-toners-for-printers-and-photocopiers", "category URL to scrape")
	outDir := flag.String("out", "output/officesolution", "output directory")
	workers := flag.Int("workers", 5, "number of concurrent workers")
	flag.Parse()

	err := officesolution.Run(*categoryURL, *outDir, *workers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
