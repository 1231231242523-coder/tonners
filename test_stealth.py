import asyncio
from playwright.async_api import async_playwright
import json

async def main():
    async with async_playwright() as p:
        # Launch with additional args to appear more like a real browser
        browser = await p.chromium.launch(
            headless=True,
            args=[
                '--disable-blink-features=AutomationControlled',
                '--no-sandbox',
                '--disable-dev-shm-usage',
                '--disable-web-security',
                '--disable-features=IsolateOrigins,site-per-process'
            ]
        )
        
        context = await browser.new_context(
            user_agent="Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36",
            viewport={"width": 1920, "height": 1080}
        )
        
        # Add init script to evade detection
        await context.add_init_script("""
            Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
            Object.defineProperty(navigator, 'plugins', { get: () => [1, 2, 3] });
            Object.defineProperty(navigator, 'languages', { get: () => ['en-US', 'en'] });
        """)
        
        page = await context.new_page()
        
        # Extra headers
        await page.set_extra_http_headers({
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
            "Accept-Language": "en-GB,en-US;q=0.9,en;q=0.8",
            "Upgrade-Insecure-Requests": "1",
            "Sec-Fetch-Dest": "document",
            "Sec-Fetch-Mode": "navigate",
            "Sec-Fetch-Site": "none",
            "Sec-Fetch-User": "?1"
        })
        
        url = "https://officesolution.al/en/11-inks-and-toners-for-printers-and-photocopiers"
        print(f"Navigating to {url}...")
        
        await page.goto(url, wait_until="domcontentloaded", timeout=60000)
        
        # Wait for potential redirect or challenge completion
        print("Waiting for page load...")
        await page.wait_for_timeout(10000)
        
        content = await page.content()
        print(f"Page length: {len(content)}")
        
        if "Just a moment" in content or "challenge" in content.lower():
            print("Still on Cloudflare challenge page.")
            # Try clicking on the page to simulate human interaction
            await page.mouse.move(100, 100)
            await page.mouse.click(100, 100)
            await page.wait_for_timeout(5000)
            
            content = await page.content()
            print(f"After interaction - Page length: {len(content)}")
        
        if "product" in content.lower():
            print("Found product content!")
            # Find all links
            links = await page.query_selector_all("a[href]")
            print(f"Found {len(links)} links total")
            
            product_links = []
            for link in links:
                href = await link.get_attribute("href")
                if href and "/en/" in href and len(href) > 20:
                    text = await link.inner_text()
                    if text and len(text) > 3:
                        product_links.append((href, text))
            
            print(f"Found {len(product_links)} potential product links:")
            for href, text in product_links[:10]:
                print(f"  {text[:40]} - {href[:80]}")
        else:
            print("No product content found")
            print("First 500 chars:", content[:500])
        
        await browser.close()

if __name__ == "__main__":
    asyncio.run(main())
