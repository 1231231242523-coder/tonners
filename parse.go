package main

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
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

var (
	digitsOnly     = regexp.MustCompile(`[^\d]`)
	resultCountRe  = regexp.MustCompile(`of\s+(\d+)\s+results`)
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
	// Drop an existing /page/N suffix so we can rewrite it.
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
