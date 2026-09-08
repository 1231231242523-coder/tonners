package officesolution

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

const (
	defaultBaseURL = "https://officesolution.al/en/11-inks-and-toners-for-printers-and-photocopiers"
	outputPath     = "output/officesolution/officesolution-products.csv"
	workers        = 3
	userAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

type ProductLink struct {
	URL string
	ID  string
}

type Product struct {
	Name          string
	Price         string
	OriginalPrice string
	SalePrice     string
	Currency      string
	SKU           string
	InStock       string
	ImageURL      string
	Description   string
	URL           string
	ID            string
}

// Run executes the officesolution scraper with the given configuration.
// If outputDir is empty, defaults to "output/officesolution".
// If workers is 0, defaults to 3.
func Run(categoryURL, outputDir string, numWorkers int) error {
	if categoryURL == "" {
		categoryURL = defaultBaseURL
	}
	if outputDir == "" {
		outputDir = "output/officesolution"
	}
	if numWorkers <= 0 {
		numWorkers = 3
	}

	outputPath := outputDir + "/officesolution-products.csv"
	s := newScraper()

	fmt.Println("Collecting product links from category pages...")
	links, pages, err := s.collectProductLinks(categoryURL)
	if err != nil {
		return fmt.Errorf("error listing products: %w", err)
	}
	fmt.Printf("Found %d unique products across %d listing pages\n", len(links), pages)

	products := make([]Product, 0, len(links))
	var mu sync.Mutex
	var errCount int
	sem := make(chan struct{}, numWorkers)
	var wg sync.WaitGroup

	for i, link := range links {
		wg.Add(1)
		go func(i int, link ProductLink) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			time.Sleep(200 * time.Millisecond)

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

	if err := writeCSV(outputPath, products); err != nil {
		return fmt.Errorf("error writing CSV: %w", err)
	}
	fmt.Printf("Wrote %d products to %s (%d errors)\n", len(products), outputPath, errCount)
	return nil
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

type scraper struct {
	chromeCtx context.Context
	cancel    context.CancelFunc
}

func newScraper() *scraper {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.UserAgent(userAgent),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, _ := chromedp.NewContext(allocCtx)

	return &scraper{
		chromeCtx: ctx,
		cancel:    cancel,
	}
}

func (s *scraper) getDoc(rawURL string) (*goquery.Document, error) {
	var content string
	ctx, cancel := chromedp.NewContext(s.chromeCtx)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.Navigate(rawURL),
		chromedp.Sleep(3*time.Second), // Wait for Cloudflare challenge
		chromedp.OuterHTML("html", &content, chromedp.NodeVisible),
	)
	if err != nil {
		return nil, err
	}

	return goquery.NewDocumentFromReader(strings.NewReader(content))
}

func (s *scraper) collectProductLinks(categoryURL string) ([]ProductLink, int, error) {
	pageURL, err := url.Parse(categoryURL)
	if err != nil {
		return nil, 0, err
	}

	first, err := listingPageURL(categoryURL, 1)
	if err != nil {
		return nil, 0, err
	}
	doc, err := s.getDoc(first)
	if err != nil {
		return nil, 0, err
	}
	links, maxPage := parseListing(doc, pageURL)
	if maxPage < 1 {
		maxPage = 1
	}

	seen := make(map[string]struct{}, len(links))
	var all []ProductLink
	add := func(batch []ProductLink) {
		for _, l := range batch {
			if _, ok := seen[l.URL]; ok {
				continue
			}
			seen[l.URL] = struct{}{}
			all = append(all, l)
		}
	}
	add(links)

	for page := 2; page <= maxPage; page++ {
		u, err := listingPageURL(categoryURL, page)
		if err != nil {
			return all, maxPage, err
		}
		doc, err := s.getDoc(u)
		if err != nil {
			return all, maxPage, fmt.Errorf("listing page %d: %w", page, err)
		}
		batch, _ := parseListing(doc, pageURL)
		add(batch)
		time.Sleep(500 * time.Millisecond)
	}
	return all, maxPage, nil
}

func (s *scraper) scrapeProduct(link ProductLink) (Product, error) {
	doc, err := s.getDoc(link.URL)
	if err != nil {
		return Product{}, err
	}
	p := parseProductHTML(doc, link.URL)
	p.ID = link.ID

	if p.Name == "" && p.Price == "" {
		return p, fmt.Errorf("no name or price on %s", link.URL)
	}
	return p, nil
}

var (
	digitsOnly    = regexp.MustCompile(`[^\d]`)
	resultCountRe = regexp.MustCompile(`of\s+(\d+)\s+results`)
	htmlTagRe     = regexp.MustCompile(`<[^>]+>`)
)

func parseListing(doc *goquery.Document, pageURL *url.URL) ([]ProductLink, int) {
	maxPage := 1
	doc.Find("a.page-numbers[data-page-num]").Each(func(_ int, s *goquery.Selection) {
		n, err := strconv.Atoi(strings.TrimSpace(s.AttrOr("data-page-num", "")))
		if err == nil && n > maxPage {
			maxPage = n
		}
	})

	seen := make(map[string]struct{})
	var links []ProductLink
	doc.Find("article.product-miniature, div.product-miniature").Each(func(_ int, item *goquery.Selection) {
		href, ok := item.Find("a.product-link, a[href]").Attr("href")
		if !ok {
			return
		}
		href = strings.TrimSpace(href)
		if href == "" || strings.Contains(href, "add-to-cart") {
			return
		}
		abs := resolveURL(pageURL, href)
		if abs == "" {
			return
		}
		if _, dup := seen[abs]; dup {
			return
		}
		seen[abs] = struct{}{}

		id := extractID(href)
		links = append(links, ProductLink{URL: abs, ID: id})
	})

	if len(links) == 0 {
		doc.Find("div.wf-cell[data-post-id], li.product, div.product").Each(func(_ int, item *goquery.Selection) {
			href, ok := item.Find("h4.entry-title a, h3.product-title a, a.product-link, a[href]").First().Attr("href")
			if !ok {
				href, ok = item.Attr("href")
				if !ok {
					return
				}
			}
			href = strings.TrimSpace(href)
			if href == "" || strings.Contains(href, "add-to-cart") {
				return
			}
			abs := resolveURL(pageURL, href)
			if abs == "" {
				return
			}
			if _, dup := seen[abs]; dup {
				return
			}
			seen[abs] = struct{}{}

			id := extractID(href)
			links = append(links, ProductLink{URL: abs, ID: id})
		})
	}

	return links, maxPage
}

func extractID(href string) string {
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	path := u.Path
	parts := strings.Split(path, "-")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		digits := regexp.MustCompile(`\d+`).FindString(lastPart)
		if digits != "" {
			return digits
		}
	}
	return ""
}

func parseResultCount(doc *goquery.Document) int {
	text := strings.TrimSpace(doc.Find(".woocommerce-result-count").First().Text())
	m := resultCountRe.FindStringSubmatch(text)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func parseProductHTML(doc *goquery.Document, pageURL string) Product {
	p := Product{URL: pageURL}

	name := strings.TrimSpace(doc.Find("h1.page-title, h1.product-name, h1#product_name").First().Text())
	if name == "" {
		name = strings.TrimSpace(doc.Find("h1.entry-title").First().Text())
	}
	p.Name = name

	priceSel := doc.Find("div.product-prices .price, p.price, span.price, .current-price").First()
	if priceSel.Length() == 0 {
		priceSel = doc.Find(".product-price .price, .price-box .price").First()
	}

	if ins := priceSel.Find("ins .woocommerce-Price-amount").First(); ins.Length() > 0 {
		p.SalePrice, p.Currency = parseAmount(ins)
		p.Price = p.SalePrice
	} else if amt := priceSel.Find(".woocommerce-Price-amount").First(); amt.Length() > 0 {
		p.Price, p.Currency = parseAmount(amt)
	} else if priceSel.Length() > 0 {
		priceText := strings.TrimSpace(priceSel.Text())
		p.Price, p.Currency = parsePriceText(priceText)
	}

	if del := priceSel.Find("del .woocommerce-Price-amount").First(); del.Length() > 0 {
		p.OriginalPrice, _ = parseAmount(del)
	} else if del := priceSel.Find("del, .old-price").First(); del.Length() > 0 {
		delText := strings.TrimSpace(del.Text())
		if p.OriginalPrice == "" {
			p.OriginalPrice, _ = parsePriceText(delText)
		}
	}

	p.SKU = strings.TrimSpace(doc.Find(".sku, .reference, .product-reference").First().Text())
	if strings.HasPrefix(p.SKU, "Reference:") {
		p.SKU = strings.TrimPrefix(p.SKU, "Reference:")
		p.SKU = strings.TrimSpace(p.SKU)
	}

	p.ImageURL = strings.TrimSpace(doc.Find(`meta[property="og:image"]`).AttrOr("content", ""))
	if p.ImageURL == "" {
		p.ImageURL = strings.TrimSpace(doc.Find(".woocommerce-product-gallery img, #product-images img, .product-cover img").First().AttrOr("src", ""))
	}

	desc := strings.TrimSpace(doc.Find(".woocommerce-product-details__short-description, .product-description-short, .short-description").First().Text())
	if desc == "" {
		desc = strings.TrimSpace(doc.Find("#tab-description p, .product-description, #description").First().Text())
	}
	p.Description = collapseSpace(desc)

	bodyClass := doc.Find("body").AttrOr("class", "")
	articleClass := doc.Find("article.product, div.product").First().AttrOr("class", "")
	classes := bodyClass + " " + articleClass
	switch {
	case strings.Contains(classes, "outofstock") || strings.Contains(strings.ToLower(doc.Text()), "out of stock"):
		p.InStock = "false"
	case strings.Contains(classes, "instock") || strings.Contains(strings.ToLower(doc.Text()), "in stock"):
		p.InStock = "true"
	}

	if p.InStock == "" {
		if doc.Find(".available-now, .in-stock").Length() > 0 {
			p.InStock = "true"
		} else if doc.Find(".unavailable, .out-of-stock").Length() > 0 {
			p.InStock = "false"
		}
	}

	return p
}

func parseAmount(s *goquery.Selection) (price, currency string) {
	currency = strings.TrimSpace(s.Find(".woocommerce-Price-currencySymbol").First().Text())
	clone := s.Clone()
	clone.Find(".woocommerce-Price-currencySymbol").Remove()
	price = normalizePrice(clone.Text())
	return price, currency
}

func parsePriceText(raw string) (price, currency string) {
	raw = strings.ReplaceAll(raw, "\u00a0", " ")
	raw = collapseSpace(raw)

	currencyMatchers := []string{"ALL", "Lek", "€", "EUR", "$", "USD", "£", "GBP"}
	for _, curr := range currencyMatchers {
		if strings.Contains(raw, curr) {
			currency = curr
			raw = strings.ReplaceAll(raw, curr, "")
			break
		}
	}

	price = digitsOnly.ReplaceAllString(strings.TrimSpace(raw), "")
	return price, currency
}

func normalizePrice(raw string) string {
	raw = strings.ReplaceAll(raw, "\u00a0", " ")
	raw = collapseSpace(raw)
	return digitsOnly.ReplaceAllString(raw, "")
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func listingPageURL(categoryURL string, page int) (string, error) {
	u, err := url.Parse(categoryURL)
	if err != nil {
		return "", err
	}
	if page <= 1 {
		return u.String(), nil
	}

	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func resolveURL(base *url.URL, href string) string {
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if base == nil {
		return ref.String()
	}
	return base.ResolveReference(ref).String()
}
