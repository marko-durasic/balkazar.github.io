// A visitor's explicit language choice wins. The neutral homepage is the
// only automatically localized URL; direct translated/deep links stay intact.
function setupSiteLanguage(win, doc, navInfo) {
  const current = new URL(win.location.href);
  const currentLanguage = doc.documentElement.lang.toLowerCase().startsWith("zh") ? "zh" : "en";
  const valid = (value) => value === "en" || value === "zh";
  const key = "balkazar-language";
  const read = () => {
    try { return win.localStorage.getItem(key); } catch { return null; }
  };
  const save = (value) => {
    try { win.localStorage.setItem(key, value); return true; } catch { return false; }
  };
  const explicit = current.searchParams.get("lang");
  const isWeb = current.protocol === "https:" || current.protocol === "http:";
  const isNeutralHome = current.pathname === "/" || current.pathname === "/index.html";
  const other = currentLanguage === "zh" ? "en" : "zh";
  const alternate = doc.querySelector(other === "zh"
    ? 'link[rel="alternate"][hreflang^="zh"]'
    : 'link[rel="alternate"][hreflang="en"]');
  const languageLinkSelector = '[data-language-select], a.band-lang, a.hub-language, a.language';
  const header = doc.querySelector(".nav-inner");
  let link = header?.querySelector(languageLinkSelector);
  // Compact pages may have no existing switch. Use their actual hreflang
  // counterpart; the Chinese-only corporate page links to the English home.
  const target = alternate?.href || (current.pathname === "/zh/weiya/" ? "/?lang=en" : null);
  if (header && (link || target)) {
    if (!link) {
      link = doc.createElement("a");
      link.href = target;
      link.textContent = other === "zh" ? "中文" : "EN";
    }
    link.classList.add("header-language");
    link.dataset.languageSelect = other;
    link.lang = other === "zh" ? "zh-Hant" : "en";
    link.setAttribute("aria-label", other === "zh" ? "切換成繁體中文" : "Switch to English");
    const toggle = header.querySelector(".nav-toggle");
    if (toggle) header.insertBefore(link, toggle);
    else header.appendChild(link);
  }
  doc.querySelectorAll(languageLinkSelector).forEach((item) => {
    const language = item.dataset.languageSelect || other;
    item.dataset.languageSelect = language;
    if (isWeb) {
      const destination = new URL(item.href, current);
      // Keep campaign attribution and make modified clicks/new tabs work,
      // even when browser storage is blocked.
      current.searchParams.forEach((value, name) => {
        if (name !== "lang" && !destination.searchParams.has(name)) {
          destination.searchParams.set(name, value);
        }
      });
      destination.searchParams.set("lang", language);
      // Only copy the fragment on the homepages, where sections match.
      if (doc.body.dataset.siteLanguage && !destination.hash) destination.hash = current.hash;
      item.href = destination.href;
    }
    item.addEventListener("click", () => save(language));
  });
  if (!isWeb) return;
  if (valid(explicit) && explicit === currentLanguage) {
    // Keep the explicit marker if storage is unavailable, so reloading
    // English in Taiwan cannot force a return to Chinese.
    if (save(explicit)) {
      current.searchParams.delete("lang");
      try { win.history.replaceState(null, "", current.href); } catch { /* optional cleanup */ }
    }
    return;
  }
  if (!isNeutralHome) return;
  let zone;
  try { zone = Intl.DateTimeFormat().resolvedOptions().timeZone; } catch { /* locale fallback */ }
  const localChinese = (navInfo.languages || [navInfo.language]).some(
    (language) => /^zh(?:-Hant)?-TW$|^zh-Hant$/i.test(language || "")
  );
  const defaultLanguage = zone && zone !== "UTC" ? (zone === "Asia/Taipei" ? "zh" : "en")
    : (localChinese ? "zh" : "en");
  const saved = read();
  const desired = valid(explicit) ? explicit : valid(saved) ? saved : defaultLanguage;
  if (desired === "zh") {
    current.pathname = "/zh/";
    win.location.replace(current.href);
  }
}

setupSiteLanguage(window, document, navigator);

const year = document.getElementById("year");
if (year) year.textContent = new Date().getFullYear();

const nav = document.querySelector(".nav");
const navToggle = document.querySelector(".nav-toggle");
const navLinks = document.getElementById("nav-links");
const menuIsChinese = document.documentElement.lang.toLowerCase().startsWith("zh");
const menuOpenLabel = menuIsChinese ? "開啟選單" : "Open menu";
const menuCloseLabel = menuIsChinese ? "關閉選單" : "Close menu";

function closeNav() {
  nav?.classList.remove("open");
  navToggle?.setAttribute("aria-expanded", "false");
  navToggle?.setAttribute("aria-label", menuOpenLabel);
}

navToggle?.addEventListener("click", () => {
  const isOpen = nav.classList.toggle("open");
  navToggle.setAttribute("aria-expanded", String(isOpen));
  navToggle.setAttribute("aria-label", isOpen ? menuCloseLabel : menuOpenLabel);
});

document.querySelectorAll("[data-copy]").forEach((button) => {
  button.addEventListener("click", async () => {
    const source = document.getElementById(button.dataset.copy);
    if (!source) return;
    const originalLabel = button.textContent;
    try {
      await navigator.clipboard.writeText(source.textContent.trim());
      button.textContent = button.dataset.copiedLabel || "Copied";
      setTimeout(() => { button.textContent = originalLabel; }, 1800);
    } catch {
      window.getSelection()?.selectAllChildren(source);
    }
  });
});

// Use the canonical URL: tracking parameters and anchors do not belong in shared links.
const shareUrl = document.querySelector('link[rel="canonical"]')?.href
  || window.location.href.split("#")[0].split("?")[0];
const shareTitle = document.querySelector('meta[property="og:title"]')?.content || document.title;
const shareIsChinese = document.documentElement.lang.toLowerCase().startsWith("zh");

async function copyEventLink() {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(shareUrl);
      return true;
    } catch {
      // Continue to the older-browser copy fallback below.
    }
  }
  const field = document.createElement("textarea");
  field.value = shareUrl;
  field.setAttribute("readonly", "");
  field.style.position = "fixed";
  field.style.opacity = "0";
  document.body.appendChild(field);
  field.select();
  let copied = false;
  try { copied = document.execCommand("copy"); } catch { /* no clipboard support */ }
  field.remove();
  return copied;
}

document.querySelectorAll(".share-tools").forEach((tools) => {
  const status = tools.querySelector(".share-feedback");
  const copy = tools.querySelector('[data-share="copy"]');
  const native = tools.querySelector('[data-share="native"]');
  const message = (success) => {
    status.textContent = success
      ? (shareIsChinese ? "連結已複製，可以貼到訊息或 Instagram 限動。" : "Link copied. Paste it into a message or Instagram Story.")
      : (shareIsChinese ? "無法自動複製，請從瀏覽器網址列複製連結。" : "Couldn't copy automatically. Please copy the URL from your browser.");
  };

  copy?.addEventListener("click", async () => message(await copyEventLink()));

  if (native && typeof navigator.share === "function") {
    native.hidden = false;
    native.addEventListener("click", async () => {
      try {
        await navigator.share({ title: shareTitle, url: shareUrl });
        status.textContent = "";
      } catch (error) {
        if (error.name !== "AbortError") message(await copyEventLink());
      }
    });
  }

  const encodedUrl = encodeURIComponent(shareUrl);
  const encodedTitle = encodeURIComponent(shareTitle);
  const targets = {
    facebook: `https://www.facebook.com/sharer/sharer.php?u=${encodedUrl}`,
    line: `https://social-plugins.line.me/lineit/share?url=${encodedUrl}`,
    x: `https://twitter.com/intent/tweet?text=${encodedTitle}&url=${encodedUrl}`,
    email: `mailto:?subject=${encodedTitle}&body=${encodeURIComponent(shareTitle + "\n" + shareUrl)}`,
  };
  tools.querySelectorAll("a[data-share]").forEach((link) => {
    if (targets[link.dataset.share]) link.href = targets[link.dataset.share];
  });
});
navLinks?.querySelectorAll("a").forEach((link) => link.addEventListener("click", closeNav));
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") closeNav();
});

document.querySelectorAll(".booking-form").forEach((form) => {
  const isChinese = form.dataset.language === "zh";

  const confirmPanel = document.createElement("div");
  confirmPanel.className = "booking-confirm";
  confirmPanel.hidden = true;
  confirmPanel.innerHTML = isChinese
    ? `<p class="eyebrow">內容已備妥</p>
       <h3>選擇送出方式</h3>
       <p class="confirm-note">選擇 Email、Gmail，或直接複製內容。</p>
       <div class="tool-row">
         <a class="btn" data-role="mailto">開啟郵件軟體 <span>→</span></a>
         <a class="tool-button" data-role="gmail" target="_blank" rel="noopener">用 Gmail 開啟 ↗</a>
         <button class="tool-button" data-role="copy" type="button">複製內容</button>
       </div>
       <button class="text-link confirm-back" data-role="back" type="button">← 返回修改</button>`
    : `<p class="eyebrow">Ready to send</p>
       <h3>Choose how to send it</h3>
       <p class="confirm-note">Choose email, Gmail, or copy the message.</p>
       <div class="tool-row">
         <a class="btn" data-role="mailto">Open email app <span>→</span></a>
         <a class="tool-button" data-role="gmail" target="_blank" rel="noopener">Open in Gmail ↗</a>
         <button class="tool-button" data-role="copy" type="button">Copy message</button>
       </div>
       <button class="text-link confirm-back" data-role="back" type="button">← Edit details</button>`;
  form.insertAdjacentElement("afterend", confirmPanel);

  const mailtoLink = confirmPanel.querySelector('[data-role="mailto"]');
  const gmailLink = confirmPanel.querySelector('[data-role="gmail"]');
  const copyButton = confirmPanel.querySelector('[data-role="copy"]');
  const backButton = confirmPanel.querySelector('[data-role="back"]');
  const copyButtonLabel = copyButton.textContent;

  backButton.addEventListener("click", () => {
    confirmPanel.hidden = true;
    form.hidden = false;
  });

  form.addEventListener("submit", (event) => {
    event.preventDefault();
    const data = new FormData(form);
    const subject = isChinese
      ? `Balkazar 演出邀約｜${data.get("date") || "日期待定"}｜${data.get("venue") || "場地待定"}`
      : `Balkazar booking inquiry | ${data.get("date") || "date TBD"} | ${data.get("venue") || "venue TBD"}`;
    const tracking = new URLSearchParams(window.location.search);
    const campaign = ["utm_source", "utm_medium", "utm_campaign", "utm_content"]
      .filter((key) => tracking.get(key))
      .map((key) => `${key}: ${tracking.get(key)}`)
      .join("\n");
    const body = isChinese
      ? `姓名：${data.get("name")}\nEmail：${data.get("email")}\n日期：${data.get("date") || "待定"}\n城市／場地：${data.get("venue") || "待定"}\n活動類型：${data.get("type")}\n\n預算與其他資訊：\n${data.get("details") || "尚未提供"}`
      : `Name: ${data.get("name")}\nEmail: ${data.get("email")}\nDate: ${data.get("date") || "TBD"}\nCity / venue: ${data.get("venue") || "TBD"}\nEvent type: ${data.get("type")}\n\nBudget and details:\n${data.get("details") || "Not provided yet"}`;
    const trackedBody = campaign ? `${body}\n\nCampaign:\n${campaign}` : body;

    mailtoLink.href = `mailto:booking@balkazar.org?subject=${encodeURIComponent(subject)}&body=${encodeURIComponent(trackedBody)}`;
    gmailLink.href = `https://mail.google.com/mail/?view=cm&fs=1&to=booking@balkazar.org&su=${encodeURIComponent(subject)}&body=${encodeURIComponent(trackedBody)}`;
    const fullText = `${subject}\n\n${trackedBody}`;
    copyButton.onclick = async () => {
      try {
        await navigator.clipboard.writeText(fullText);
        copyButton.textContent = isChinese ? "已複製" : "Copied";
      } catch {
        copyButton.textContent = isChinese ? "請手動選取複製" : "Select the text manually";
      }
      setTimeout(() => { copyButton.textContent = copyButtonLabel; }, 1800);
    };

    form.hidden = true;
    confirmPanel.hidden = false;
    confirmPanel.scrollIntoView({ behavior: "smooth", block: "start" });
  });
});

const masonryImages = Array.from(document.querySelectorAll(".photo-masonry img"));
if (masonryImages.length) {
  const lightbox = document.createElement("div");
  lightbox.className = "lightbox";
  lightbox.innerHTML = `
    <button class="lightbox-close" aria-label="Close">&times;</button>
    <button class="lightbox-prev" aria-label="Previous photo">&larr;</button>
    <img alt="">
    <button class="lightbox-next" aria-label="Next photo">&rarr;</button>`;
  document.body.appendChild(lightbox);
  const lightboxImg = lightbox.querySelector("img");
  let currentIndex = 0;

  function showPhoto(index) {
    currentIndex = (index + masonryImages.length) % masonryImages.length;
    lightboxImg.src = masonryImages[currentIndex].src;
    lightboxImg.alt = masonryImages[currentIndex].alt;
    lightbox.classList.add("open");
  }
  function closeLightbox() { lightbox.classList.remove("open"); }

  masonryImages.forEach((img, index) => img.addEventListener("click", () => showPhoto(index)));
  lightbox.querySelector(".lightbox-close").addEventListener("click", closeLightbox);
  lightbox.querySelector(".lightbox-prev").addEventListener("click", () => showPhoto(currentIndex - 1));
  lightbox.querySelector(".lightbox-next").addEventListener("click", () => showPhoto(currentIndex + 1));
  lightbox.addEventListener("click", (event) => { if (event.target === lightbox) closeLightbox(); });
  document.addEventListener("keydown", (event) => {
    if (!lightbox.classList.contains("open")) return;
    if (event.key === "Escape") closeLightbox();
    if (event.key === "ArrowLeft") showPhoto(currentIndex - 1);
    if (event.key === "ArrowRight") showPhoto(currentIndex + 1);
  });
}
