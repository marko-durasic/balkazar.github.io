// The private review build supplies local file paths. Empty paths serve the photograph.
(() => {
  const film = document.getElementById("hero-film");
  const toggle = document.getElementById("film-toggle");
  const reducedMotion = matchMedia("(prefers-reduced-motion: reduce)");
  const portrait = matchMedia("(max-aspect-ratio: 1/1)");
  const connection = navigator.connection;
  const chinese = document.documentElement.lang.startsWith("zh");
  let userPaused = false;

  function updateLabel() {
    toggle.textContent = film.paused
      ? (chinese ? "播放影片" : "Play video")
      : (chinese ? "暫停影片" : "Pause video");
  }

  function updateFilm() {
    const source = portrait.matches ? film.dataset.mobile : film.dataset.desktop;
    if (!source || reducedMotion.matches || connection?.saveData) {
      film.pause();
      film.hidden = toggle.hidden = true;
      if (film.hasAttribute("src")) {
        film.removeAttribute("src");
        film.load();
      }
      return;
    }
    if (film.getAttribute("src") !== source) {
      film.hidden = true;
      film.src = source;
    }
    if (!userPaused) film.play().catch(() => {});
  }

  film.addEventListener("loadeddata", () => {
    if (!film.hasAttribute("src")) return;
    toggle.hidden = false;
    updateLabel();
  });
  film.addEventListener("playing", () => {
    film.hidden = false;
    updateLabel();
  });
  film.addEventListener("pause", updateLabel);
  film.addEventListener("error", () => { film.hidden = toggle.hidden = true; });
  toggle.addEventListener("click", () => {
    userPaused = !film.paused;
    if (userPaused) film.pause();
    else film.play().catch(updateLabel);
  });
  reducedMotion.addEventListener("change", updateFilm);
  portrait.addEventListener("change", updateFilm);
  connection?.addEventListener("change", updateFilm);
  updateFilm();
})();

const featuredShow = document.querySelector(".show-feature");
if (Date.now() >= Date.parse(featuredShow.dataset.endsAt)) {
  document.getElementById("next-show-heading").textContent =
    document.documentElement.lang.startsWith("zh") ? "最近的演出" : "Recent show";
  featuredShow.querySelector(".show-calendar").remove();
}
