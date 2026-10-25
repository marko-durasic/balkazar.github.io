"""Automated responsive UI smoke gate for the bilingual Balkazar static site.

Run against this checkout, before GitHub Pages artifact upload. Use Playwright's
installed Chromium; save representative screenshots for visual review.
"""
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Thread
from urllib.parse import quote
import os
import sys

from playwright.sync_api import sync_playwright

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / "qa-artifacts"
VIEWPORTS = (320, 390, 768, 1280)
EVENT = "events/2026-12-03-european-christmas-market/index.html"
IMPORTANT = {"index.html", "zh/index.html", EVENT, "zh/" + EVENT,
             "events/index.html", "zh/events/index.html"}

class QuietHandler(SimpleHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def copyfile(self, source, outputfile):
        try:
            super().copyfile(source, outputfile)
        except (BrokenPipeError, ConnectionResetError):
            pass  # A browser navigated away while a large image was loading.

def evaluate_layout(page):
    return page.evaluate("""() => {
      const root = document.documentElement;
      const width = window.innerWidth;
      const heading = document.querySelector("main h1");
      const mainImages = [...document.querySelectorAll("main img")];
      const visibleBroken = mainImages.filter(img => {
        const r = img.getBoundingClientRect();
        return r.bottom > 0 && r.top < innerHeight && img.complete && !img.naturalWidth;
      }).map(img => img.getAttribute("src"));
      const badOverflows = [...document.querySelectorAll("main *")]
        .filter(el => {
          let r = el.getBoundingClientRect();
          return r.width && (r.left < -2 || r.right > width + 2);
        }).slice(0, 3).map(el => el.tagName.toLowerCase() + '.' + (el.className || ''));
      let title = null;
      const market = document.querySelector(".event-title-zh__market");
      const date = document.querySelector(".event-title-zh__date");
      if (market && date) {
        const shape = (el) => {
          const range = document.createRange();
          range.selectNodeContents(el);
          const all = [...range.getClientRects()].filter(r => r.width > 0);
          const box = el.getBoundingClientRect();
          return {rectCount: all.length, right:box.right, width:box.width, height:box.height};
        };
        title = {market:shape(market), date:shape(date), maxRight:heading.getBoundingClientRect().right};
      }
      return {width, scrollWidth: root.scrollWidth, headingFont: heading && getComputedStyle(heading).fontSize,
              headingText: heading && heading.innerText, visibleBroken, badOverflows, title};
    }""")

def main():
    pages = sorted(p.relative_to(ROOT).as_posix() for p in ROOT.rglob("index.html")
                   if not set(p.parts).intersection({".git", "node_modules", "qa-artifacts", ".venv"}))
    OUT.mkdir(exist_ok=True)
    service = ThreadingHTTPServer(("127.0.0.1", 0), partial(QuietHandler, directory=str(ROOT)))
    Thread(target=service.serve_forever, daemon=True).start()
    errors, checks = [], 0
    try:
        with sync_playwright() as playwright:
            executable = os.getenv("BALKAZAR_CHROME") or None
            browser = playwright.chromium.launch(headless=True, executable_path=executable,
                        args=["--no-sandbox"])
            try:
                for file in pages:
                    route = file[:-10] if file.endswith("index.html") else file
                    url = f"http://127.0.0.1:{service.server_port}/{quote(route)}"
                    for viewport in VIEWPORTS:
                        page = browser.new_page(viewport={"width": viewport, "height": 850},device_scale_factor=1)
                        if viewport == 390 and "/2026-" in file and "events/" in file:
                            # Browser smoke simulation only; real phone testing remains separate.
                            page.add_init_script("Object.defineProperty(navigator, 'share', {configurable:true, value:async (payload) => { window.__balkazarTestShare = payload; }});")
                        response = page.goto(url,wait_until="domcontentloaded",timeout=20000)
                        if not response or response.status != 200:
                            errors.append(f"{file} @{viewport}: HTTP {response.status if response else 'unknown'}")
                            page.close()
                            continue
                        page.evaluate("() => document.fonts.ready")
                        page.wait_for_timeout(100)
                        data = evaluate_layout(page)
                        checks += 1
                        if data["scrollWidth"] > viewport + 1:
                            errors.append(f"{file} @{viewport}: horizontal document overflow {data['scrollWidth']}px"
                                         f" (elements: {data['badOverflows']})")
                        if data["visibleBroken"]:
                            errors.append(f"{file} @{viewport}: broken above-the-fold image {data['visibleBroken']}")
                        if data["title"]:
                            for part in ("market", "date"):
                                box = data["title"][part]
                                if box["rectCount"] != 1 or box["right"] > viewport + 1:
                                    errors.append(f"{file} @{viewport}: Traditional Chinese {part} wraps/overflows {box}")
                        if file in IMPORTANT and viewport in (320, 390):
                            lang = "zh" if file.startswith("zh/") else "en"
                            short = "home" if file in ("index.html", "zh/index.html") else (
                                "christmas" if "2026-12-03" in file else "events")
                            page.screenshot(path=str(OUT / f"{lang}-{short}-{viewport}.png"),full_page=False)
                        if viewport == 390 and file in ("index.html", "zh/index.html", EVENT, "zh/" + EVENT):
                            toggle = page.locator("button.nav-toggle")
                            if toggle.count():
                                before = toggle.get_attribute("aria-expanded")
                                toggle.click()
                                after = toggle.get_attribute("aria-expanded")
                                if before != "false" or after != "true":
                                    errors.append(f"{file} @390: mobile navigation toggle doesn't open accessibly")
                                expected_open_label = "關閉選單" if file.startswith("zh/") else "Close menu"
                                expected_closed_label = "開啟選單" if file.startswith("zh/") else "Open menu"
                                if toggle.get_attribute("aria-label") != expected_open_label:
                                    errors.append(f"{file} @390: mobile menu open label is not localized")
                                toggle.click()
                                if toggle.get_attribute("aria-expanded") != "false" or toggle.get_attribute("aria-label") != expected_closed_label:
                                    errors.append(f"{file} @390: mobile menu does not close with a localized label")
                        if viewport == 390 and "/2026-" in file and "events/" in file:
                            # Assert mobile native sharing and canonical social links in both languages.
                            canonical = page.locator('link[rel="canonical"]').get_attribute("href")
                            if page.locator('meta[property="og:url"]').get_attribute("content") != canonical:
                                errors.append(f"{file} @390: OG URL mismatch")
                            if page.locator('meta[name="twitter:card"]').get_attribute("content") != "summary_large_image":
                                errors.append(f"{file} @390: missing social-card metadata")
                            if page.locator(".share-tools").count() != 1:
                                errors.append(f"{file} @390: share panel missing")
                            else:
                                primary = page.locator(".share-primary")
                                secondary = page.locator(".share-secondary")
                                if primary.count() != 1 or secondary.count() != 1:
                                    errors.append(f"{file} @390: primary/secondary share groups missing")
                                else:
                                    native = primary.locator('button[data-share="native"]')
                                    copy = primary.locator('button[data-share="copy"]')
                                    if native.count() != 1 or copy.count() != 1:
                                        errors.append(f"{file} @390: native/copy share buttons missing")
                                    else:
                                        for button in (native, copy):
                                            bounds = button.bounding_box()
                                            if not bounds or bounds["height"] < 44:
                                                errors.append(f"{file} @390: primary share button missing/too small")
                                        if native.is_visible():
                                            native.click()
                                            shared = page.evaluate("window.__balkazarTestShare")
                                            if not shared or shared.get("url") != canonical:
                                                errors.append(f"{file} @390: native share missing canonical URL")
                                        else:
                                            errors.append(f"{file} @390: native share API did not reveal Share button")
                                        copy.click()
                                        try:
                                            page.locator(".share-feedback").wait_for(state="visible", timeout=2500)
                                            if not page.locator(".share-feedback").inner_text().strip():
                                                errors.append(f"{file} @390: missing copy status")
                                        except Exception:
                                            errors.append(f"{file} @390: copy feedback timed out")
                                    for provider in ("facebook", "line", "x", "email"):
                                        links = secondary.locator(f'a[data-share="{provider}"]')
                                        if links.count() != 1:
                                            errors.append(f"{file} @390: missing {provider} link")
                                        else:
                                            from urllib.parse import unquote
                                            link = links.get_attribute("href") or ""
                                            if not canonical or canonical not in unquote(link):
                                                errors.append(f"{file} @390: {provider} share URL not canonical")
                                            bounds = links.bounding_box()
                                            if not bounds or bounds["height"] < 44:
                                                errors.append(f"{file} @390: {provider} tap target too small")
                        page.close()
            finally:
                browser.close()
    finally:
        service.shutdown()
        service.server_close()
    print(f"Responsive QA: {checks} viewport checks across {len(pages)} pages (EN + zh-Hant)")
    if errors:
        print("\n".join("- " + msg for msg in errors))
        print(f"FAIL: {len(errors)} responsive issues; screenshots in qa-artifacts")
        return 1
    print("PASS: widths 320, 390, 768, 1280; no horizontal overflow, broken visible media or CJK heading line breaks")
    print("PASS: mobile navigation toggle for EN/ZH home + event pages")
    return 0

if __name__ == "__main__":
    sys.exit(main())
