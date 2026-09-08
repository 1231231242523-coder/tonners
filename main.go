package main

import (
	"flag"
	"fmt"
	"os"

	"eurooffice/suppliers/eurooffice"
)

func main() {
	categoryURL := flag.String("url", "https://eurooffice.al/product-category/bojra-printeri/", "category listing URL")
	outDir := flag.String("out", "output/eurooffice", "output directory for CSV file")
	workers := flag.Int("workers", 5, "concurrent product fetches")
	flag.Parse()

	if err := eurooffice.Run(*categoryURL, *outDir, *workers); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
