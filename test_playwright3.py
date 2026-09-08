import asyncio
from playwright.async_api import async_playwright

async def main():
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        context = await browser.new_context(
            user_agent="Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36"
        )
        page = await context.new_page()
        
        url = "https://officesolution.al/en/11-inks-and-toners-for-printers-and-photocopiers"
        print(f"Navigating to {url}...")
        
        # Navigate with longer timeout
        await page.goto(url, wait_until="networkidle", timeout=120000)
        
        # Wait much longer for Cloudflare challenge
        print("Waiting for Cloudflare challenge to complete...")
        for i in range(10):
            await page.wait_for_timeout(3000)
            content = await page.content()
            if "Just a moment" not in content and "challenge" not in content.lower():
                print(f"Challenge passed after {(i+1)*3} seconds!")
                break
            print(f"Still on challenge page... ({(i+1)*3}s)")
        
        # Get final content
        content = await page.content()
        print(f"\nFinal page length: {len(content)}")
        
        # Check for products
        if "product" in content.lower():
            print("Found product content!")
            
            # Try to find product links using various selectors
            selectors = [
                "a[href*='/en/']",
                ".product a",
                "article a",
                ".item a",
                "[class*='product'] a"
            ]
            
            for selector in selectors:
                links = await page.query_selector_all(selector)
                if len(links) > 0:
                    print(f"\nSelector '{selector}' found {len(links)} links:")
                    for i, link in enumerate(links[:5]):
                        href = await link.get_attribute("href")
                        text = await link.inner_text()
                        if href and len(href) > 10:
                            print(f"  {i}: {text[:50]} - {href[:80]}")
                    break
        
        # Save HTML
        with open("/workspace/debug_page2.html", "w") as f:
            f.write(content)
        print("\nSaved debug HTML to /workspace/debug_page2.html")
        
        await browser.close()

if __name__ == "__main__":
    asyncio.run(main())
