import htmx from "htmx.org";
import Alpine from "@alpinejs/csp";

window.htmx = htmx;

Alpine.data("toggle", () => ({
  open: false,
  init() { this.open = this.$el.hasAttribute("data-open"); },
  toggle() { this.open = !this.open; },
  close() { this.open = false; },
}));

// Copy buttons and the announcement close button only touch the DOM, so they use plain
// delegated listeners instead of Alpine components.
async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.setAttribute("readonly", "");
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    document.execCommand("copy");
    ta.remove();
  }
}

document.addEventListener("click", async (event) => {
  const button = event.target.closest("[data-copy-text]");
  if (button) {
    await copyText(button.dataset.copyText);
    const label = button.querySelector("[data-copy-label]");
    const original = label.textContent;
    button.querySelector("[data-copy-icon]").hidden = true;
    button.querySelector("[data-copy-done]").hidden = false;
    label.textContent = "Tersalin!";
    setTimeout(() => {
      button.querySelector("[data-copy-icon]").hidden = false;
      button.querySelector("[data-copy-done]").hidden = true;
      label.textContent = original;
    }, 2000);
    return;
  }
  const dismiss = event.target.closest("[data-dismiss-announcement]");
  if (dismiss) {
    document.cookie = `dismissed_announcement=${dismiss.dataset.dismissAnnouncement}; path=/; max-age=${60 * 60 * 24 * 180}; SameSite=Lax`;
    dismiss.closest(".announce").remove();
  }
});

// Hero slider: the first slide is server-rendered as the LCP image; rotation starts only after
// the page has loaded, stops when touched, and never runs for reduced-motion users.
Alpine.data("slider", () => ({
  index: 0,
  timer: null,
  init() {
    this.slides = [...this.$el.querySelectorAll("[data-slide]")];
    this.dots = [...this.$el.querySelectorAll("[data-dot]")];
    this.show(0);
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (this.slides.length < 2 || reduced) return;
    const start = () => { this.timer = setInterval(() => this.show((this.index + 1) % this.slides.length), 6000); };
    if (document.readyState === "complete") start(); else window.addEventListener("load", start, { once: true });
    const stop = () => { clearInterval(this.timer); this.timer = null; };
    this.$el.addEventListener("pointerdown", stop, { passive: true });
    this.$el.addEventListener("focusin", stop);
  },
  show(i) {
    this.index = i;
    this.slides.forEach((s, n) => { s.hidden = n !== i; });
    this.dots.forEach((d, n) => {
      d.classList.toggle("is-active", n === i);
      d.setAttribute("aria-current", n === i ? "true" : "false");
    });
  },
  go(event) {
    clearInterval(this.timer);
    this.timer = null;
    this.show(Number(event.currentTarget.dataset.dot));
  },
}));

document.addEventListener("submit", (event) => {
  const form = event.target;
  const message = form.dataset.confirm;
  if (message && !window.confirm(message)) {
    event.preventDefault();
    event.stopImmediatePropagation();
  }
}, true);

document.addEventListener("htmx:confirm", (event) => {
  const form = event.target.closest("form");
  const message = form && form.dataset.confirm;
  if (!message) return;
  event.preventDefault();
  if (window.confirm(message)) event.detail.issueRequest(true);
});

document.addEventListener("change", (event) => {
  if (event.target.matches("[data-autosubmit]")) event.target.form.requestSubmit();
});

document.addEventListener("htmx:responseError", (event) => {
  const text = event.detail.xhr.responseText;
  if (text && text.length < 300) window.alert(text);
});

window.Alpine = Alpine;
Alpine.start();
