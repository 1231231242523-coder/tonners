# OfficeSolution Scraper

A Go-based scraper for extracting product data from OfficeSolution.al (similar to EuroOffice scraper).

## Features

- Extracts product links from category pages
- Scrapes product details: name, price, description, SKU, image URL, stock status
- Supports pagination across multiple listing pages
- Concurrent scraping with configurable workers
- Outputs data to CSV format

## Requirements

**Important:** OfficeSolution.al uses Cloudflare protection, which requires a real browser to bypass the JavaScript challenge. You must have one of the following installed:

- Google Chrome
- Chromium
- ChromeDriver

### Installing Chromium (Debian/Ubuntu)

```bash
apt-get update
apt-get install -y chromium
```

## Usage

Run from the project root:

```bash
# Scrape OfficeSolution (default category: inks and toners)
./scraper -supplier officesolution

# Specify custom category URL
./scraper -supplier officesolution -url "https://officesolution.al/en/11-inks-and-toners-for-printers-and-photocopiers"

# Custom output directory
./scraper -supplier officesolution -out "output/my-products"

# Adjust concurrency (default: 5 workers)
./scraper -supplier officesolution -workers 3
```

### Command-line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-supplier` | Supplier to scrape (`eurooffice` or `officesolution`) | `eurooffice` |
| `-url` | Category listing URL to scrape | Auto-detected per supplier |
| `-out` | Output directory for CSV file | `output/<supplier>` |
| `-workers` | Number of concurrent product fetches | `5` |

## Output

The scraper generates a CSV file with the following columns:

- `name` - Product name
- `price` - Current price (numeric only)
- `original_price` - Original price before discount
- `sale_price` - Sale/discounted price
- `currency` - Currency code/symbol
- `sku` - Product SKU/reference
- `in_stock` - Stock availability (true/false)
- `image_url` - Main product image URL
- `description` - Product description
- `url` - Product page URL

Example output location: `output/officesolution/officesolution-products.csv`

## How It Works

1. **Browser Automation**: Uses chromedp to launch a headless Chrome browser
2. **Cloudflare Bypass**: Waits for Cloudflare challenge to complete (3 second delay)
3. **Link Collection**: Parses category listing pages to extract product URLs
4. **Product Scraping**: Visits each product page and extracts:
   - Product name from h1 tags
   - Price from price containers
   - Description from product description sections
   - Image URLs from meta tags or gallery
   - Stock status from CSS classes
5. **CSV Export**: Writes all collected data to a CSV file

## Similar Structure to EuroOffice

This scraper follows the same pattern as the EuroOffice scraper:
- Same `Run()` function signature
- Same Product struct fields
- Same CSV output format
- Same concurrency model with workers
- Compatible with the main.go command-line interface

## Troubleshooting

### "executable file not found in $PATH"

Install Chromium or Google Chrome:
```bash
apt-get install -y chromium
```

### Cloudflare blocking requests

The scraper includes a 3-second delay to allow Cloudflare challenges to complete. If you're still being blocked:
- Increase the sleep duration in `getDoc()`
- Reduce the number of workers
- Add longer delays between requests

### No products found

Verify the category URL is correct and contains products. The scraper looks for common e-commerce selectors like:
- `article.product-miniature`
- `div.product-miniature`
- `li.product`
- `div.product`

## License

Same license as the parent project.
