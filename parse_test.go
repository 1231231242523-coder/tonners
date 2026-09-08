package main

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestParseListingFromExample(t *testing.T) {
	f, err := os.Open("examples/product-pages.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		t.Fatal(err)
	}
	base, err := url.Parse("https://eurooffice.al/product-category/bojra-printeri/")
	if err != nil {
		t.Fatal(err)
	}

	links, maxPage := parseListing(doc, base)
	if maxPage != 15 {
		t.Fatalf("max page = %d, want 15", maxPage)
	}
	if got := parseResultCount(doc); got != 240 {
		t.Fatalf("result count = %d, want 240", got)
	}
	if len(links) != 16 {
		t.Fatalf("unique products = %d, want 16", len(links))
	}
	if links[0].ID != "2712" {
		t.Fatalf("first id = %q, want 2712", links[0].ID)
	}
	wantURL := "https://eurooffice.al/product/boje-hp-301-ch561ee-black/"
	if links[0].URL != wantURL {
		t.Fatalf("first url = %q, want %s", links[0].URL, wantURL)
	}
}

func TestListingPageURL(t *testing.T) {
	base := "https://eurooffice.al/product-category/bojra-printeri/"
	got, err := listingPageURL(base, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got != base {
		t.Fatalf("page 1 = %q, want %q", got, base)
	}
	got, err = listingPageURL(base, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://eurooffice.al/product-category/bojra-printeri/page/3/"
	if got != want {
		t.Fatalf("page 3 = %q, want %q", got, want)
	}
}

func TestParseProductHTML(t *testing.T) {
	html := `
	<html>
	<body class="single-product">
	<meta property="og:image" content="https://eurooffice.al/img.jpg">
	<div class="product instock sale">
	  <div class="summary entry-summary">
	    <h1 class="product_title entry-title">Test Ink</h1>
	    <p class="price">
	      <del><span class="woocommerce-Price-amount amount"><bdi>3 500&nbsp;<span class="woocommerce-Price-currencySymbol">L</span></bdi></span></del>
	      <ins><span class="woocommerce-Price-amount amount"><bdi>3 200&nbsp;<span class="woocommerce-Price-currencySymbol">L</span></bdi></span></ins>
	    </p>
	    <div class="woocommerce-product-details__short-description"><p>Short desc</p></div>
	  </div>
	</div>
	</body>
	</html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	p := parseProductHTML(doc, "https://eurooffice.al/product/test/")
	if p.Name != "Test Ink" {
		t.Fatalf("name = %q", p.Name)
	}
	if p.Price != "3200" {
		t.Fatalf("price = %q, want 3200", p.Price)
	}
	if p.OriginalPrice != "3500" {
		t.Fatalf("original = %q, want 3500", p.OriginalPrice)
	}
	if p.SalePrice != "3200" {
		t.Fatalf("sale = %q, want 3200", p.SalePrice)
	}
	if p.Currency != "L" {
		t.Fatalf("currency = %q, want L", p.Currency)
	}
	if p.InStock != "true" {
		t.Fatalf("in_stock = %q", p.InStock)
	}
	if p.Description != "Short desc" {
		t.Fatalf("description = %q", p.Description)
	}
}

func TestApplyStoreFallback(t *testing.T) {
	p := Product{URL: "https://example/product/x"}
	sp := storeProduct{
		Name:             "API Name",
		SKU:              "",
		ShortDescription: "<p>From API</p>",
		IsInStock:        true,
	}
	sp.OnSale = true
	sp.Prices.Price = "8000"
	sp.Prices.RegularPrice = "9500"
	sp.Prices.SalePrice = "8000"
	sp.Prices.CurrencySym = "L"
	sp.Images = []struct {
		Src string `json:"src"`
	}{{Src: "https://img"}}
	applyStoreFallback(&p, sp)
	if p.Price != "8000" || p.OriginalPrice != "9500" || p.SalePrice != "8000" || p.Currency != "L" {
		t.Fatalf("prices: %+v", p)
	}
	if p.Description != "From API" {
		t.Fatalf("description = %q", p.Description)
	}
	if p.InStock != "true" {
		t.Fatalf("in_stock = %q", p.InStock)
	}
}
