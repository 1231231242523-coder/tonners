package main

import (
	"flag"
	"fmt"
	"os"

	"eurooffice/suppliers/eurooffice"
	"eurooffice/suppliers/officesolution"
)

func main() {
	categoryURL := flag.String("url", "", "category listing URL")
	outDir := flag.String("out", "", "output directory for CSV file")
	workers := flag.Int("workers", 5, "concurrent product fetches")
	supplier := flag.String("supplier", "eurooffice", "supplier to scrape: eurooffice or officesolution")
	flag.Parse()

	var err error
	switch *supplier {
	case "officesolution":
		if *categoryURL == "" {
			*categoryURL = "https://officesolution.al/en/11-inks-and-toners-for-printers-and-photocopiers"
		}
		if *outDir == "" {
			*outDir = "output/officesolution"
		}
		err = officesolution.Run(*categoryURL, *outDir, *workers)
	default:
		if *categoryURL == "" {
			*categoryURL = "https://eurooffice.al/product-category/bojra-printeri/"
		}
		if *outDir == "" {
			*outDir = "output/eurooffice"
		}
		err = eurooffice.Run(*categoryURL, *outDir, *workers)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
