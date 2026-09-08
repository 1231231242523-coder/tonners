# OfficeSolution.al Product Scraper

A Go-based scraper for extracting product data from officesolution.al. This scraper uses XHR/fetch requests to efficiently collect product information without browser automation.

## Prerequisites

- Docker (required for running the scraper)
- No local Go installation needed - everything runs in a Docker container

## How It Works

The scraper makes direct HTTP requests with XHR-like headers to fetch product listing pages and product details:
- Uses `X-Requested-With: XMLHttpRequest` header to mimic AJAX requests
- Parses JSON responses from pagination endpoints
- Extracts product details using regex-based HTML parsing
- Outputs data to CSV format

## Running with Docker

### Build the Docker Image

```bash
docker build -t officesolution-scraper .
```

### Run the Scraper

```bash
docker run --rm -v $(pwd)/output:/app/output officesolution-scraper
```

This will:
1. Run the scraper in a container
2. Mount the `./output` directory to store results
3. Automatically create the output directory if it doesn't exist

### Output

The scraper generates a CSV file at:
```
output/officesolution/officesolution-products.csv
```

Containing columns:
- `name` - Product name
- `price` - Product price
- `sku` - Product SKU/reference
- `image_url` - Main product image URL
- `description` - Product description
- `in_stock` - Stock availability status
- `url` - Product page URL

## Handling Cloudflare Protection

If you encounter Cloudflare blocking:

1. **Manual Cookie Extraction** (Recommended):
   - Visit [officesolution.al](https://officesolution.al) in your browser
   - Open Developer Tools → Network tab
   - Copy the `Cookie` header from any request
   - Pass it as an environment variable:
   
   ```bash
   docker run --rm \
     -e OFFICESOLUTION_COOKIE="your-cookie-string-here" \
     -v $(pwd)/output:/app/output \
     officesolution-scraper
   ```

2. **Adjust Rate Limiting**:
   Modify the delay between requests in the code if needed.

## Project Structure

```
/workspace
├── Dockerfile                    # Docker configuration
├── README.md                     # This file
└── suppliers/
    └── officesolution/
        ├── go.mod                # Go module definition
        ├── go.sum                # Go dependencies checksum
        ├── main.go               # Main entry point
        └── officesolution.go     # Scraper implementation
```

## Troubleshooting

### "no such file or directory" error
Ensure the output directory exists or is properly mounted:
```bash
mkdir -p output/officesolution
docker run --rm -v $(pwd)/output:/app/output officesolution-scraper
```

### Empty CSV or no products found
- The site may have changed its structure
- Cloudflare protection may be blocking requests (try cookie method above)
- Check network connectivity

### Build errors
Ensure you're in the project root directory when building:
```bash
cd /workspace
docker build -t officesolution-scraper .
```

## Notes

- The scraper respects the site's pagination and fetches all available pages
- Default timeout is set to 30 seconds per request
- User-Agent mimics Chrome 152 on macOS for better compatibility
- All requests include proper XHR headers to match the site's expected AJAX behavior
