package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"
)

func main() {
	categoryURL := flag.String("url", "https://eurooffice.al/product-category/bojra-printeri/", "category listing URL")
	outPath := flag.String("out", "eurooffice-products.csv", "CSV output path")
	workers := flag.Int("workers", 5, "concurrent product fetches")
	flag.Parse()

	if *workers < 1 {
		*workers = 1
	}

	s := newScraper()
	fmt.Println("Collecting product links from category pages...")
	links, pages, err := s.collectProductLinks(*categoryURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing products: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Found %d unique products across %d listing pages\n", len(links), pages)

	products := make([]Product, 0, len(links))
	var mu sync.Mutex
	var errCount int
	sem := make(chan struct{}, *workers)
	var wg sync.WaitGroup

	for i, link := range links {
		wg.Add(1)
		go func(i int, link ProductLink) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			time.Sleep(100 * time.Millisecond)

			p, err := s.scrapeProduct(link)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errCount++
				fmt.Fprintf(os.Stderr, "Skip %s: %v\n", link.URL, err)
				return
			}
			products = append(products, p)
			fmt.Printf("[%d/%d] %s — %s %s\n", i+1, len(links), p.Name, p.Price, p.Currency)
		}(i, link)
	}
	wg.Wait()

	if err := writeCSV(*outPath, products); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing CSV: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %d products to %s (%d errors)\n", len(products), *outPath, errCount)
}

func writeCSV(path string, products []Product) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{
		"name", "price", "original_price", "sale_price", "currency", "sku", "in_stock", "image_url", "description", "url",
	}); err != nil {
		return err
	}
	for _, p := range products {
		if err := w.Write([]string{
			p.Name,
			p.Price,
			p.OriginalPrice,
			p.SalePrice,
			p.Currency,
			p.SKU,
			p.InStock,
			p.ImageURL,
			p.Description,
			p.URL,
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
