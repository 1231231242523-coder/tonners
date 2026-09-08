package officesolution

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL = "https://officesolution.al/en/11-inks-and-toners-for-printers-and-photocopiers"
	outputPath     = "output/officesolution/officesolution-products.csv"
	workers        = 3
	userAgent      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36"
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
	client *http.Client
}

func newScraper() *scraper {
	return &scraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// fetchJSON makes an XHR-like request to get JSON data from the server
func (s *scraper) fetchJSON(rawURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// fetchHTML makes a regular request to get HTML content
func (s *scraper) fetchHTML(rawURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (s *scraper) collectProductLinks(categoryURL string) ([]ProductLink, int, error) {
	pageURL, err := url.Parse(categoryURL)
	if err != nil {
		return nil, 0, err
	}

	// First, fetch page 1 to determine max pages
	xhrURL := categoryURL
	if !strings.Contains(categoryURL, "?") {
		xhrURL = categoryURL + "?page=1&from-xhr"
	} else if !strings.Contains(categoryURL, "from-xhr") {
		xhrURL = categoryURL + "&from-xhr"
	}

	jsonData, err := s.fetchJSON(xhrURL)
	if err != nil {
		return nil, 0, err
	}

	links, maxPage, err := parseListingJSON(jsonData, pageURL)
	if err != nil {
		return nil, 0, err
	}
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

	// Fetch remaining pages via XHR
	for page := 2; page <= maxPage; page++ {
		u, err := listingPageURL(categoryURL, page)
		if err != nil {
			return all, maxPage, err
		}
		// Add from-xhr parameter for XHR request
		if !strings.Contains(u, "from-xhr") {
			if strings.Contains(u, "?") {
				u = u + "&from-xhr"
			} else {
				u = u + "?from-xhr"
			}
		}
		jsonData, err := s.fetchJSON(u)
		if err != nil {
			return all, maxPage, fmt.Errorf("listing page %d: %w", page, err)
		}
		batch, _, err := parseListingJSON(jsonData, pageURL)
		if err != nil {
			return all, maxPage, fmt.Errorf("parsing page %d: %w", page, err)
		}
		add(batch)
		time.Sleep(500 * time.Millisecond)
	}
	return all, maxPage, nil
}

func (s *scraper) scrapeProduct(link ProductLink) (Product, error) {
	htmlData, err := s.fetchHTML(link.URL)
	if err != nil {
		return Product{}, err
	}
	p := parseProductHTML(string(htmlData), link.URL)
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

// ListingJSON represents the JSON response from the XHR request
type ListingJSON struct {
	Products []ProductItem `json:"products"`
	Pages    int           `json:"pages"`
}

// ProductItem represents a product in the JSON listing
type ProductItem struct {
	URL string `json:"url"`
	ID  string `json:"id"`
}

func parseListingJSON(jsonData []byte, pageURL *url.URL) ([]ProductLink, int, error) {
	var result ListingJSON
	if err := json.Unmarshal(jsonData, &result); err != nil {
		// If JSON parsing fails, try parsing as HTML fallback
		return parseListingHTML(string(jsonData), pageURL), 1, nil
	}

	seen := make(map[string]struct{})
	var links []ProductLink

	for _, item := range result.Products {
		href := strings.TrimSpace(item.URL)
		if href == "" || strings.Contains(href, "add-to-cart") {
			continue
		}
		abs := resolveURL(pageURL, href)
		if abs == "" {
			continue
		}
		if _, dup := seen[abs]; dup {
			continue
		}
		seen[abs] = struct{}{}

		id := item.ID
		if id == "" {
			id = extractID(href)
		}
		links = append(links, ProductLink{URL: abs, ID: id})
	}

	maxPage := result.Pages
	if maxPage < 1 {
		maxPage = 1
	}

	return links, maxPage, nil
}

func parseListingHTML(content string, pageURL *url.URL) []ProductLink {
	// Simple regex-based parsing for HTML content
	// This is a fallback if JSON parsing fails
	var links []ProductLink
	seen := make(map[string]struct{})

	// Match product links in HTML
	re := regexp.MustCompile(`<a[^>]*href=["']([^"']+)["'][^>]*class=["'][^"']*product-link[^"']*["']`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		href := strings.TrimSpace(match[1])
		if href == "" || strings.Contains(href, "add-to-cart") {
			continue
		}
		abs := resolveURL(pageURL, href)
		if abs == "" {
			continue
		}
		if _, dup := seen[abs]; dup {
			continue
		}
		seen[abs] = struct{}{}
		id := extractID(href)
		links = append(links, ProductLink{URL: abs, ID: id})
	}

	return links
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

func parseProductHTML(htmlContent string, pageURL string) Product {
	p := Product{URL: pageURL}

	// Extract name using regex
	nameRe := regexp.MustCompile(`<h1[^>]*class=["'][^"']*(?:page-title|product-name|product_name)[^"']*["'][^>]*>([^<]+)</h1>`)
	if match := nameRe.FindStringSubmatch(htmlContent); len(match) > 1 {
		p.Name = strings.TrimSpace(match[1])
	}
	if p.Name == "" {
		nameRe = regexp.MustCompile(`<h1[^>]*class=["'][^"']*entry-title[^"']*["'][^>]*>([^<]+)</h1>`)
		if match := nameRe.FindStringSubmatch(htmlContent); len(match) > 1 {
			p.Name = strings.TrimSpace(match[1])
		}
	}

	// Extract price using regex
	priceRe := regexp.MustCompile(`<[^>]*class=["'][^"']*(?:price|current-price)[^"']*["'][^>]*>([^<]*(?:<[^>]*>[^<]*)*)</[^>]*>`)
	if match := priceRe.FindStringSubmatch(htmlContent); len(match) > 1 {
		priceText := htmlTagRe.ReplaceAllString(match[1], "")
		p.Price, p.Currency = parsePriceText(strings.TrimSpace(priceText))
	}

	// Extract SKU
	skuRe := regexp.MustCompile(`<[^>]*class=["'][^"']*(?:sku|reference|product-reference)[^"']*["'][^>]*>([^<]+)</[^>]*>`)
	if match := skuRe.FindStringSubmatch(htmlContent); len(match) > 1 {
		p.SKU = strings.TrimSpace(match[1])
		if strings.HasPrefix(p.SKU, "Reference:") {
			p.SKU = strings.TrimPrefix(p.SKU, "Reference:")
			p.SKU = strings.TrimSpace(p.SKU)
		}
	}

	// Extract image URL
	imgRe := regexp.MustCompile(`<meta[^>]*property=["']og:image["'][^>]*content=["']([^"']+)["']`)
	if match := imgRe.FindStringSubmatch(htmlContent); len(match) > 1 {
		p.ImageURL = strings.TrimSpace(match[1])
	}
	if p.ImageURL == "" {
		imgRe = regexp.MustCompile(`<img[^>]*class=["'][^"']*(?:woocommerce-product-gallery|product-cover)[^"']*["'][^>]*src=["']([^"']+)["']`)
		if match := imgRe.FindStringSubmatch(htmlContent); len(match) > 1 {
			p.ImageURL = strings.TrimSpace(match[1])
		}
	}

	// Extract description
	descRe := regexp.MustCompile(`<[^>]*class=["'][^"']*(?:product-description-short|short-description)[^"']*["'][^>]*>([^<]*(?:<[^>]*>[^<]*)*)</[^>]*>`)
	if match := descRe.FindStringSubmatch(htmlContent); len(match) > 1 {
		desc := htmlTagRe.ReplaceAllString(match[1], "")
		p.Description = collapseSpace(strings.TrimSpace(desc))
	}

	// Extract stock status
	if strings.Contains(htmlContent, "outofstock") || strings.Contains(strings.ToLower(htmlContent), "out of stock") {
		p.InStock = "false"
	} else if strings.Contains(htmlContent, "instock") || strings.Contains(strings.ToLower(htmlContent), "in stock") {
		p.InStock = "true"
	}

	return p
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
