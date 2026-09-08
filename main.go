package main

import (
	"flag"
	"fmt"
	"os"

	"eurooffice/suppliers/eurooffice"
	"eurooffice/suppliers/officesolution"
)

func main() {
        supplier := flag.String("supplier", "", "supplier to scrape: eurooffice or officesolution (required)")
        flag.Parse()

        if *supplier == "" {
                fmt.Fprintln(os.Stderr, "Error: -supplier flag is required (eurooffice or officesolution)")
                os.Exit(1)
        }

        var categoryURL, outDir string
        workers := 5

        switch *supplier {
        case "officesolution":
                categoryURL = "https://officesolution.al/en/11-inks-and-toners-for-printers-and-photocopiers"
                outDir = "output/officesolution"
                err := officesolution.Run(categoryURL, outDir, workers)
                if err != nil {
                        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
                        os.Exit(1)
                }
        default:
                categoryURL = "https://eurooffice.al/product-category/bojra-printeri/"
                outDir = "output/eurooffice"
                err := eurooffice.Run(categoryURL, outDir, workers)
                if err != nil {
                        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
                        os.Exit(1)
                }
        }
}
