package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const userAgent = "Mozilla/5.0 (compatible; eurooffice-scraper/1.0; +https://eurooffice.al)"

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
