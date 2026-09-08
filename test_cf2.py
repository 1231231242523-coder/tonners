import asyncio
from playwright.async_api import async_playwright

async def main():
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        context = await browser.new_context(
            user_agent="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
        )
        page = await context.new_page()
        
        # First visit the main page without XHR headers to pass Cloudflare
        url = "https://officesolution.al/en/11-inks-and-toners-for-printers-and-photocopiers"
        print(f"Visiting {url}...")
        
        response = await page.goto(url, wait_until="networkidle", timeout=60000)
        
        # Wait for any challenge
        await page.wait_for_timeout(8000)
        
        # Now try to make the XHR request from within the page context
        print("Making XHR request from page context...")
        
        xhr_url = url + "?page=2&from-xhr"
        xhr_result = await page.evaluate(f"""
            () => {{
                return new Promise((resolve, reject) => {{
                    const xhr = new XMLHttpRequest();
                    xhr.open('GET', '{xhr_url}', true);
                    xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest');
                    xhr.setRequestHeader('Accept', 'application/json, text/javascript, */*; q=0.01');
                    
                    xhr.onload = () => {{
                        if (xhr.status === 200) {{
                            resolve({{ status: xhr.status, contentLength: xhr.responseText.length, first500: xhr.responseText.substring(0, 500) }});
                        }} else {{
                            reject({{ status: xhr.status, error: xhr.statusText }});
                        }}
                    }};
                    
                    xhr.onerror = () => reject({{ error: 'Network error' }});
                    xhr.send();
                }});
            }}
        """)
        
        print("XHR Result:", xhr_result)
        
        # Get cookies that we can use for future requests
        cookies = await context.cookies()
        print("\nCookies obtained:")
        for cookie in cookies:
            if 'PHPSESSID' in cookie.get('name', '') or 'PrestaShop' in cookie.get('name', '') or '_ga' in cookie.get('name', ''):
                print(f"  {cookie['name']}: {cookie['value'][:50]}...")
        
        await browser.close()

if __name__ == "__main__":
    asyncio.run(main())
