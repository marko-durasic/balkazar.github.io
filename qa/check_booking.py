"""Exercise the client-only booking handoff without sending any inquiry."""
from functools import partial
from http.server import ThreadingHTTPServer
from threading import Thread
from urllib.parse import urlparse, parse_qs

from playwright.sync_api import sync_playwright
from check_responsive import ROOT, OUT, QuietHandler


def main():
    server = ThreadingHTTPServer(("127.0.0.1", 0), partial(QuietHandler, directory=ROOT))
    Thread(target=server.serve_forever, daemon=True).start()
    origin = f"http://127.0.0.1:{server.server_port}"
    paths = ("/", "/zh/", "/zh/weiya/")
    try:
        with sync_playwright() as p:
            browser = p.chromium.launch()
            for width in (320, 390, 1280):
                for path in paths:
                    context = browser.new_context(viewport={"width": width, "height": 900})
                    # Block all third parties; the synthetic inquiry never leaves localhost.
                    context.route("**/*", lambda route: route.continue_() if route.request.url.startswith(origin) else route.abort())
                    context.add_init_script("""Object.defineProperty(navigator, 'clipboard', {value: {
                      writeText: async () => { throw new Error('Clipboard blocked'); }
                    }});""")
                    page = context.new_page()
                    page.goto(origin + path + "?utm_source=qa&personal=SECRET&EMAIL=qa@example.invalid&utm_campaign=bad%0AINJECT", wait_until="domcontentloaded")
                    page.locator('[name="name"]').fill("Synthetic QA 測試")
                    page.locator('[name="email"]').fill("qa@example.invalid")
                    page.locator('[name="venue"]').fill("<b>literal venue</b>")
                    page.locator('[name="details"]').fill("Synthetic only. Never send.")
                    page.locator('.booking-form button[type="submit"]').click()
                    preview = page.locator('[data-role="message"]')
                    assert preview.is_visible()
                    body = preview.input_value()
                    assert "Synthetic QA 測試" in body and "<b>literal venue</b>" in body
                    assert "utm_source: qa" in body
                    assert "SECRET" not in body and "INJECT" not in body
                    assert page.locator('.booking-confirm b').count() == 0
                    assert page.locator('[data-role="line"]').get_attribute('href') == "https://line.me/R/ti/p/@489cncyj"
                    for role in ("mailto", "gmail"):
                        query = parse_qs(urlparse(page.locator(f'[data-role="{role}"]').get_attribute('href')).query)
                        assert query['body'][0] in body
                    page.locator('[data-role="copy"]').click()
                    assert preview.evaluate('(el) => el.selectionEnd - el.selectionStart') == len(body)
                    assert page.evaluate('document.documentElement.scrollWidth <= innerWidth + 1')
                    assert page.url.startswith(origin + path + "?utm_source=qa&")
                    if width == 390:
                        OUT.mkdir(exist_ok=True)
                        page.screenshot(path=str(OUT / ("booking" + path.replace('/', '-') + ".png")), full_page=True)
                    # Also check the working clipboard path and that edits retain entered details.
                    page.evaluate("navigator.clipboard.writeText = async text => { window.copiedMessage = text; }")
                    page.locator('[data-role="copy"]').click()
                    assert page.evaluate('window.copiedMessage') == body
                    page.locator('[data-role="back"]').click()
                    assert page.locator('[name="name"]').input_value() == "Synthetic QA 測試"
                    context.close()
            nojs = browser.new_context(java_script_enabled=False)
            for path in paths:
                page = nojs.new_page()
                page.goto(origin + path, wait_until="domcontentloaded")
                assert not page.locator('.booking-form').is_visible()
                assert page.locator('noscript').is_visible()
                assert page.locator('a[href="mailto:ddjura87@gmail.com"]').count() > 0
                assert page.locator('a[href="https://line.me/R/ti/p/@489cncyj"]').count() > 0
                page.close()
            browser.close()
        print("PASS: 9 mobile/desktop booking handoffs, clipboard failure/success, privacy, and 3 no-JS fallbacks")
    finally:
        server.shutdown()


if __name__ == "__main__":
    main()
