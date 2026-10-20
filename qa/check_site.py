"""Offline site health gate for Balkazar EN and Traditional Chinese pages.

Checks tracked HTML for semantic essentials and resolves local page/assets links.
No third-party URLs or logins are requested.
"""
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import unquote, urlsplit
import sys

ROOT = Path(__file__).resolve().parent.parent

class PageParser(HTMLParser):
    def __init__(self):
        super().__init__()
        self.html_lang = ""
        self.viewport = False
        self.headings = 0
        self.title = False
        self.links = []
        self.errors = []
        self.ids = set()

    def handle_starttag(self, tag, attrs):
        a = dict(attrs)
        if tag == "html":
            self.html_lang = a.get("lang", "")
        if tag == "h1":
            self.headings += 1
        if tag == "title":
            self.title = True
        if tag == "meta" and a.get("name") == "viewport":
            self.viewport = "width=device-width" in a.get("content", "")
        if tag == "img":
            if "alt" not in a:
                self.errors.append("image missing alt attribute (empty alt allowed for decoration)")
            self.links.append(("image", a.get("src", "")))
        if tag == "script" and a.get("src"):
            self.links.append(("script", a["src"]))
        if tag == "link" and a.get("rel") in {"stylesheet", "icon", "manifest"}:
            self.links.append(("linked asset", a.get("href", "")))
        if tag == "a" and a.get("href"):
            self.links.append(("link", a["href"]))
        if a.get("id"):
            if a["id"] in self.ids:
                self.errors.append("duplicate element id " + a["id"])
            self.ids.add(a["id"])

def resolve_link(page, url):
    if not url or url.startswith(("mailto:", "tel:", "#", "data:", "javascript:", "sms:")):
        return None
    parsed = urlsplit(url)
    if parsed.scheme or parsed.netloc:
        return None
    path = unquote(parsed.path)
    if not path:
        return None
    location = (ROOT / path.lstrip("/")) if path.startswith("/") else (page.parent / path)
    location = location.resolve()
    if not location.is_relative_to(ROOT):
        return "escapes website directory"
    if location.is_dir():
        location = location / "index.html"
    elif not location.suffix:
        location = location / "index.html"
    if not location.is_file():
        return "missing local file " + str(location.relative_to(ROOT))
    return None

def main():
    pages = sorted(p for p in ROOT.rglob("*.html") if all(
        excluded not in p.parts for excluded in {".git", "node_modules", ".venv", "qa-artifacts"}
    ))
    problems = []
    english = chinese = 0
    for page in pages:
        doc = PageParser()
        doc.feed(page.read_text(encoding="utf-8"))
        rel = page.relative_to(ROOT)
        zh = rel.parts[0] == "zh"
        english += not zh
        chinese += zh
        expected = "zh" if zh else "en"
        if not doc.html_lang.lower().startswith(expected):
            problems.append(f"{rel}: HTML language should be {expected!r}, got {doc.html_lang!r}")
        if not doc.viewport:
            problems.append(f"{rel}: responsive viewport meta missing")
        if not doc.title:
            problems.append(f"{rel}: document title missing")
        if doc.headings != 1:
            problems.append(f"{rel}: expected one h1, got {doc.headings}")
        for kind, link in doc.links:
            result = resolve_link(page, link)
            if result:
                problems.append(f"{rel}: {kind} {link!r}: {result}")
        for error in doc.errors:
            problems.append(f"{rel}: {error}")
    print(f"Static QA: {len(pages)} pages ({english} EN, {chinese} zh-Hant)")
    if problems:
        print("\n".join("- " + issue for issue in problems))
        print(f"FAIL: {len(problems)} site problems")
        return 1
    print("PASS: page language, viewport, titles, H1, image alts and all local assets/links")
    return 0

if __name__ == "__main__":
    sys.exit(main())
