import asyncio
from playwright.async_api import async_playwright

async def main():
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        context = await browser.new_context(
            user_agent="Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36"
        )
        page = await context.new_page()
        
        # Set headers before navigation
        await page.set_extra_http_headers({
            "Accept": "application/json, text/javascript, */*; q=0.01",
            "X-Requested-With": "XMLHttpRequest"
        })
        
        url = "https://officesolution.al/en/11-inks-and-toners-for-printers-and-photocopiers?page=2&from-xhr"
        print(f"Navigating to {url}...")
        await page.goto(url, wait_until="networkidle", timeout=30000)
        
        # Wait a bit for any dynamic content
        await page.wait_for_timeout(3000)
        
        # Get the page content
        content = await page.content()
        print(f"Page length: {len(content)}")
        print("First 2000 chars:", content[:2000])
        
        await browser.close()

if __name__ == "__main__":
    asyncio.run(main())
