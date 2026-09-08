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
        
        # Navigate and wait for Cloudflare challenge to complete
        await page.goto(url, wait_until="domcontentloaded", timeout=60000)
        
        # Wait for the page to load (Cloudflare challenge should auto-complete)
        print("Waiting for Cloudflare challenge...")
        try:
            await page.wait_for_selector("#challenge-stage", state="detached", timeout=10000)
        except:
            print("No challenge stage found or already passed")
        
        # Additional wait for content to load
        await page.wait_for_timeout(5000)
        
        # Get the page content
        content = await page.content()
        print(f"Page length: {len(content)}")
        
        # Check if we have products
        if "product" in content.lower() or "item" in content.lower():
            print("Found product content!")
        else:
            print("May still be on challenge page")
            
        # Try to find product links
        links = await page.query_selector_all("a.product")
        print(f"Found {len(links)} product links")
        
        if len(links) > 0:
            for i, link in enumerate(links[:3]):
                href = await link.get_attribute("href")
                print(f"  Link {i}: {href}")
        
        # Save full HTML for debugging
        with open("/workspace/debug_page.html", "w") as f:
            f.write(content)
        print("Saved debug HTML to /workspace/debug_page.html")
        
        await browser.close()

if __name__ == "__main__":
    asyncio.run(main())
