package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	baseURL    = "https://eurooffice.al/product-category/bojra-printeri/"
	outputPath = "output/eurooffice/eurooffice-products.csv"
	workers    = 5
	userAgent  = "Mozilla/5.0 (compatible; eurooffice-scraper/1.0; +https://eurooffice.al)"
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

func main() {
	s := newScraper()

	fmt.Println("Collecting product links from category pages...")
	links, pages, err := s.collectProductLinks(baseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing products: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Found %d unique products across %d listing pages\n", len(links), pages)

	products := make([]Product, 0, len(links))
	var mu sync.Mutex
	var errCount int
	sem := make(chan struct{}, workers)
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

	if err := writeCSV(outputPath, products); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing CSV: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %d products to %s (%d errors)\n", len(products), outputPath, errCount)
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
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *scraper) get(rawURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: HTTP %d", rawURL, resp.StatusCode)
	}
	return body, nil
}

func (s *scraper) getDoc(rawURL string) (*goquery.Document, error) {
	body, err := s.get(rawURL)
	if err != nil {
		return nil, err
	}
	return goquery.NewDocumentFromReader(strings.NewReader(string(body)))
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
		time.Sleep(150 * time.Millisecond)
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

	if p.Price == "" && link.ID != "" {
		body, err := s.get(storeProductURL(link.ID))
		if err != nil {
			return p, fmt.Errorf("store api fallback for %s: %w", link.URL, err)
		}
		sp, err := parseStoreProduct(body)
		if err != nil {
			return p, err
		}
		applyStoreFallback(&p, sp)
	}
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
	doc.Find("div.wf-cell[data-post-id]").Each(func(_ int, cell *goquery.Selection) {
		id := strings.TrimSpace(cell.AttrOr("data-post-id", ""))
		href, ok := cell.Find("h4.entry-title a").Attr("href")
		if !ok {
			return
		}
		href = strings.TrimSpace(href)
		if href == "" || strings.Contains(href, "add-to-cart=") {
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
		links = append(links, ProductLink{URL: abs, ID: id})
	})
	return links, maxPage
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

	name := strings.TrimSpace(doc.Find("h1.product_title").First().Text())
	if name == "" {
		name = strings.TrimSpace(doc.Find("h1.entry-title").First().Text())
	}
	p.Name = name

	summaryPrice := doc.Find("div.summary p.price").First()
	if summaryPrice.Length() == 0 {
		summaryPrice = doc.Find("p.price").First()
	}
	if ins := summaryPrice.Find("ins .woocommerce-Price-amount").First(); ins.Length() > 0 {
		p.SalePrice, p.Currency = parseAmount(ins)
		p.Price = p.SalePrice
	} else if amt := summaryPrice.Find(".woocommerce-Price-amount").First(); amt.Length() > 0 {
		p.Price, p.Currency = parseAmount(amt)
	}
	if del := summaryPrice.Find("del .woocommerce-Price-amount").First(); del.Length() > 0 {
		p.OriginalPrice, _ = parseAmount(del)
	}

	p.SKU = strings.TrimSpace(doc.Find(".sku").First().Text())
	p.ImageURL = strings.TrimSpace(doc.Find(`meta[property="og:image"]`).AttrOr("content", ""))
	if p.ImageURL == "" {
		p.ImageURL = strings.TrimSpace(doc.Find(".woocommerce-product-gallery img").First().AttrOr("src", ""))
	}

	desc := strings.TrimSpace(doc.Find(".woocommerce-product-details__short-description").First().Text())
	if desc == "" {
		desc = strings.TrimSpace(doc.Find("#tab-description p").First().Text())
	}
	p.Description = collapseSpace(desc)

	bodyClass := doc.Find("body").AttrOr("class", "")
	articleClass := doc.Find("article.product, div.product").First().AttrOr("class", "")
	classes := bodyClass + " " + articleClass
	switch {
	case strings.Contains(classes, "outofstock"):
		p.InStock = "false"
	case strings.Contains(classes, "instock"):
		p.InStock = "true"
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
	path := strings.TrimSuffix(u.Path, "/")
	if i := strings.LastIndex(path, "/page/"); i >= 0 {
		path = path[:i]
	}
	u.Path = path + "/page/" + strconv.Itoa(page) + "/"
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

type storeProduct struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	SKU              string `json:"sku"`
	ShortDescription string `json:"short_description"`
	Description      string `json:"description"`
	Permalink        string `json:"permalink"`
	IsInStock        bool   `json:"is_in_stock"`
	OnSale           bool   `json:"on_sale"`
	Prices           struct {
		Price        string `json:"price"`
		RegularPrice string `json:"regular_price"`
		SalePrice    string `json:"sale_price"`
		CurrencyCode string `json:"currency_code"`
		CurrencySym  string `json:"currency_symbol"`
	} `json:"prices"`
	Images []struct {
		Src string `json:"src"`
	} `json:"images"`
}

func storeProductURL(id string) string {
	return "https://eurooffice.al/wp-json/wc/store/v1/products/" + id
}

func parseStoreProduct(body []byte) (storeProduct, error) {
	var sp storeProduct
	if err := json.Unmarshal(body, &sp); err != nil {
		return storeProduct{}, fmt.Errorf("decode store product: %w", err)
	}
	return sp, nil
}

func applyStoreFallback(p *Product, sp storeProduct) {
	if p.Name == "" {
		p.Name = sp.Name
	}
	if p.Price == "" {
		p.Price = normalizePrice(sp.Prices.Price)
	}
	if p.OriginalPrice == "" {
		p.OriginalPrice = normalizePrice(sp.Prices.RegularPrice)
	}
	if p.SalePrice == "" && sp.OnSale {
		p.SalePrice = normalizePrice(sp.Prices.SalePrice)
		if p.Price == "" {
			p.Price = p.SalePrice
		}
	}
	if p.Currency == "" {
		p.Currency = strings.TrimSpace(sp.Prices.CurrencySym)
		if p.Currency == "" {
			p.Currency = sp.Prices.CurrencyCode
		}
	}
	if p.SKU == "" {
		p.SKU = sp.SKU
	}
	if p.ImageURL == "" && len(sp.Images) > 0 {
		p.ImageURL = sp.Images[0].Src
	}
	if p.Description == "" {
		p.Description = stripHTML(sp.ShortDescription)
		if p.Description == "" {
			p.Description = stripHTML(sp.Description)
		}
	}
	if p.InStock == "" {
		if sp.IsInStock {
			p.InStock = "true"
		} else {
			p.InStock = "false"
		}
	}
	if p.URL == "" {
		p.URL = sp.Permalink
	}
}

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return collapseSpace(s)
}
