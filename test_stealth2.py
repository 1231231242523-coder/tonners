import asyncio
from playwright.async_api import async_playwright
from playwright_stealth import stealth_async

async def main():
    async with async_playwright() as p:
        browser = await p.chromium.launch(
            headless=True,
            args=[
                '--disable-blink-features=AutomationControlled',
                '--no-sandbox',
                '--disable-dev-shm-usage',
            ]
        )
        
        context = await browser.new_context(
            user_agent="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
            viewport={"width": 1920, "height": 1080}
        )
        
        page = await context.new_page()
        
        # Apply stealth
        await stealth_async(page)
        
        url = "https://officesolution.al/en/11-inks-and-toners-for-printers-and-photocopiers"
        print(f"Visiting {url}...")
        
        await page.goto(url, wait_until="domcontentloaded", timeout=60000)
        
        # Wait longer
        for i in range(20):
            await page.wait_for_timeout(2000)
            content = await page.content()
            if "Just a moment" not in content and "challenge" not in content.lower() and "security" not in content.lower():
                print(f"Page loaded after {(i+1)*2}s!")
                
                # Check for products
                links = await page.query_selector_all("a[href*='/en/']")
                print(f"Found {len(links)} links")
                
                for link in links[:5]:
                    href = await link.get_attribute("href")
                    text = await link.inner_text()
                    if href and text:
                        print(f"  {text[:40]} - {href[:60]}")
                break
            print(f"Waiting... ({(i+1)*2}s)")
        
        await browser.close()

if __name__ == "__main__":
    asyncio.run(main())
