# Balkazar website — bilingual release QA

A deployment must pass QA, not simply build successfully. GitHub Actions runs checks on every pull request to main and again before deployment from main. The deploy job depends on success.

## Automated checks

- Python qa/check_site.py: all English and Traditional Chinese HTML pages have appropriate language, responsive viewport, title, single H1, image alt attributes, unique IDs, and working internal links and media.
- Python qa/check_responsive.py: actual browser rendering at 320, 390, 768 and 1280 px for **all** website pages, checking for horizontal scrolling, visible broken images, mobile menu operation, and deliberate unbroken Chinese market name/date. Screenshots from representative pages are uploaded on CI failure.

Install Playwright version 1.47.0 and its Chromium browser, then run both Python scripts from repository root. Locally you can set BALKAZAR_CHROME=/usr/bin/brave-browser to use Brave instead of packaged Chromium.

## Visual and editorial review before merge

Automation catches regressions, not good design. Inspect EN **and zh-Hant** pages on narrow phones, tablets and desktops. Check:
- Chinese heading line breaks, punctuation, contrast, scalable type and no text clipping
- Working mobile menus, button targets, keyboard focus and screen-reader reading order
- Images/video crops and captions, non-fabricated event imagery, photo usage and minors' privacy
- Actual event date, venue, time zones and Facebook Event link, localized navigation, calendar/save links
- Media loading and overall look/feel. Use quick keyboard/screen reader spot checks; automated QA is not full accessibility certification.

Release order: feature branch → PR → automated QA → human bilingual visual/content review → merge → predeployment QA → live smoke check.

Never bypass a failing check by deleting tests or hiding page content. Fix the page or improve a demonstrated false positive in a reviewable PR.
