#!/usr/bin/env python3
"""
Officesolution.al product scraper using Playwright to bypass Cloudflare.
This script extracts all products from each page of the category.
"""

import asyncio
import csv
import json
import os
import re
from playwright.async_api import async_playwright

OUTPUT_DIR = "output/officesolution"
OUTPUT_FILE = os.path.join(OUTPUT_DIR, "officesolution-products.csv")
BASE_URL = "https://officesolution.al/en/11-inks-and-toners-for-printers-and-photocopiers"

async def scrape_products():
    """Scrape all products from officesolution.al using Playwright."""
    
    os.makedirs(OUTPUT_DIR, exist_ok=True)
    
    all_products = []
    
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        context = await browser.new_context(
            user_agent="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
            viewport={"width": 1920, "height": 1080}
        )
        page = await context.new_page()
        
        # Visit main page first to pass Cloudflare challenge
        print(f"Visiting {BASE_URL}...")
        await page.goto(BASE_URL, wait_until="networkidle", timeout=120000)
        
        # Wait for Cloudflare challenge to complete
        print("Waiting for security verification...")
        for i in range(15):
            content = await page.content()
            if "Just a moment" not in content and "challenge" not in content.lower():
                print(f"Challenge passed after {(i+1)*2} seconds!")
                break
            await page.wait_for_timeout(2000)
            print(f"Still waiting... ({(i+1)*2}s)")
        
        # Get total pages
        total_pages = 1
        page_content = await page.content()
        page_matches = re.findall(r'page=(\d+)', page_content)
        if page_matches:
            total_pages = max(int(p) for p in page_matches)
        
        # Try to find pagination info
        pagination = await page.query_selector_all(".pagination li")
        if pagination:
            for item in pagination:
                text = await item.inner_text()
                if text.isdigit():
                    total_pages = max(total_pages, int(text))
        
        print(f"Found {total_pages} pages to scrape")
        
        # Scrape each page
        for page_num in range(1, total_pages + 1):
            print(f"\nScraping page {page_num}/{total_pages}...")
            
            url = f"{BASE_URL}?page={page_num}" if page_num > 1 else BASE_URL
            
            try:
                await page.goto(url, wait_until="networkidle", timeout=60000)
                await page.wait_for_timeout(3000)
                
                # Extract products from this page
                products = await extract_products_from_page(page)
                all_products.extend(products)
                print(f"  Found {len(products)} products on page {page_num}")
                
            except Exception as e:
                print(f"  Error scraping page {page_num}: {e}")
        
        await browser.close()
    
    # Save to CSV
    save_to_csv(all_products)
    print(f"\nTotal: {len(all_products)} products saved to {OUTPUT_FILE}")
    return all_products


async def extract_products_from_page(page):
    """Extract product information from the current page."""
    products = []
    
    # Find all product links
    product_selectors = [
        "article.product a",
        ".product a",
        "a[href*='/en/'][href*='product']",
        ".item a",
        "[class*='product'] a"
    ]
    
    for selector in product_selectors:
        links = await page.query_selector_all(selector)
        if links:
            for link in links:
                href = await link.get_attribute("href")
                if href and "/en/" in href:
                    # Get product details
                    product = await extract_product_details(page, link, href)
                    if product:
                        products.append(product)
            break
    
    # If no products found with selectors, try to find all links and filter
    if not products:
        all_links = await page.query_selector_all("a[href]")
        for link in all_links:
            href = await link.get_attribute("href")
            if href and "/en/" in href and len(href) > 30:
                text = await link.inner_text()
                if text and len(text) > 3:
                    product = {
                        "name": text.strip(),
                        "url": href,
                        "price": "",
                        "sku": "",
                        "image_url": "",
                        "description": "",
                        "in_stock": ""
                    }
                    products.append(product)
    
    return products


async def extract_product_details(page, element, href):
    """Extract detailed product information."""
    try:
        # Get text content
        text = await element.inner_text()
        
        # Get image if available
        img = await element.query_selector("img")
        img_src = ""
        if img:
            img_src = await img.get_attribute("src") or await img.get_attribute("data-src") or ""
        
        # Try to extract price
        price_elem = await element.query_selector(".price, [class*='price']")
        price = ""
        if price_elem:
            price = await price_elem.inner_text()
        
        product = {
            "name": text.strip() if text else "",
            "url": href,
            "price": price.strip() if price else "",
            "sku": "",
            "image_url": img_src or "",
            "description": "",
            "in_stock": ""
        }
        
        return product
    except Exception as e:
        print(f"    Error extracting product: {e}")
        return None


def save_to_csv(products):
    """Save products to CSV file."""
    if not products:
        print("No products to save")
        return
    
    fieldnames = ["name", "price", "original_price", "sale_price", "currency", 
                  "sku", "in_stock", "image_url", "description", "url", "id"]
    
    with open(OUTPUT_FILE, 'w', newline='', encoding='utf-8') as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        
        for product in products:
            row = {k: product.get(k, '') for k in fieldnames}
            writer.writerow(row)


if __name__ == "__main__":
    asyncio.run(scrape_products())
