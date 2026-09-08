package main

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"
)

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

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
