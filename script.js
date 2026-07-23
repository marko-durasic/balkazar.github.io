const siteLanguage = document.body.dataset.siteLanguage;
const languageLinks = document.querySelectorAll("[data-language-select]");

languageLinks.forEach((link) => {
  link.addEventListener("click", () => {
    localStorage.setItem("balkazar-language", link.dataset.languageSelect);
  });
});

if (siteLanguage && window.location.protocol !== "file:") {
  const savedLanguage = localStorage.getItem("balkazar-language");
  const prefersTraditionalChinese = navigator.languages?.some(
    (language) => /^zh-(TW|Hant)/i.test(language)
  );
  if (siteLanguage === "en" && (savedLanguage === "zh" || (!savedLanguage && prefersTraditionalChinese))) {
    window.location.replace("zh/");
  } else if (siteLanguage === "zh" && savedLanguage === "en") {
    window.location.replace("../");
  }
}

const year = document.getElementById("year");
if (year) year.textContent = new Date().getFullYear();

const nav = document.querySelector(".nav");
const navToggle = document.querySelector(".nav-toggle");
const navLinks = document.getElementById("nav-links");

function closeNav() {
  nav?.classList.remove("open");
  navToggle?.setAttribute("aria-expanded", "false");
  navToggle?.setAttribute("aria-label", "Open menu");
}

navToggle?.addEventListener("click", () => {
  const isOpen = nav.classList.toggle("open");
  navToggle.setAttribute("aria-expanded", String(isOpen));
  navToggle.setAttribute("aria-label", isOpen ? "Close menu" : "Open menu");
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

document.querySelectorAll("[data-share]").forEach((link) => {
  const pageUrl = encodeURIComponent(window.location.href);
  const title = encodeURIComponent(document.title);
  if (link.dataset.share === "facebook") link.href = `https://www.facebook.com/sharer/sharer.php?u=${pageUrl}`;
  if (link.dataset.share === "line") link.href = `https://social-plugins.line.me/lineit/share?url=${pageUrl}`;
  if (link.dataset.share === "email") link.href = `mailto:?subject=${title}&body=${pageUrl}`;
});
navLinks?.querySelectorAll("a").forEach((link) => link.addEventListener("click", closeNav));
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") closeNav();
});

document.querySelectorAll(".booking-form").forEach((form) => {
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    const data = new FormData(form);
    const isChinese = form.dataset.language === "zh";
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
    window.location.href = `mailto:ddjura87@gmail.com?subject=${encodeURIComponent(subject)}&body=${encodeURIComponent(trackedBody)}`;
  });
});
