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

// Rotation editing on game day: tap one player then another to swap them, or drag a name onto
// another name with a mouse. Native drag and drop does not fire on touch screens, which is why
// tapping is the primary interaction on phones.
const swap = { selected: null };

function swapPanel(el) { return el.closest("#rotasi"); }

function clearSwapSelection() {
  if (swap.selected) {
    swap.selected.classList.remove("is-selected");
    swap.selected.setAttribute("aria-pressed", "false");
  }
  swap.selected = null;
}

function announce(panel, text) {
  const status = panel && panel.querySelector("[data-swap-status]");
  if (status) status.textContent = text;
}

function submitSwap(a, b) {
  const panel = swapPanel(a);
  const form = panel && panel.querySelector("#swap-form");
  if (!form || a === b) return;
  form.elements.player_a.value = a.dataset.swapPlayer;
  form.elements.player_b.value = b.dataset.swapPlayer;
  panel.setAttribute("data-swapping", "");
  announce(panel, `Menukar ${a.dataset.name} dengan ${b.dataset.name}…`);
  swap.selected = null;
  form.requestSubmit();
}

document.addEventListener("click", (event) => {
  const chip = event.target.closest("[data-swap-player]");
  if (!chip) return;
  if (!swap.selected) {
    swap.selected = chip;
    chip.classList.add("is-selected");
    chip.setAttribute("aria-pressed", "true");
    announce(swapPanel(chip), `${chip.dataset.name} dipilih. Ketuk pemain lain untuk menukar.`);
  } else if (swap.selected === chip) {
    clearSwapSelection();
    announce(swapPanel(chip), "Pilihan dibatalkan.");
  } else {
    submitSwap(swap.selected, chip);
  }
});

document.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && swap.selected) clearSwapSelection();
});

document.addEventListener("dragstart", (event) => {
  const chip = event.target.closest && event.target.closest("[data-swap-player]");
  if (!chip) return;
  clearSwapSelection();
  swap.dragging = chip;
  chip.classList.add("is-dragging");
  event.dataTransfer.effectAllowed = "move";
  event.dataTransfer.setData("text/plain", chip.dataset.swapPlayer);
});

document.addEventListener("dragend", () => {
  if (swap.dragging) swap.dragging.classList.remove("is-dragging");
  document.querySelectorAll(".swap-chip.is-target").forEach((el) => el.classList.remove("is-target"));
  swap.dragging = null;
});

document.addEventListener("dragover", (event) => {
  const chip = event.target.closest && event.target.closest("[data-swap-player]");
  if (!chip || !swap.dragging || chip === swap.dragging) return;
  event.preventDefault();
  event.dataTransfer.dropEffect = "move";
  chip.classList.add("is-target");
});

document.addEventListener("dragleave", (event) => {
  const chip = event.target.closest && event.target.closest("[data-swap-player]");
  if (chip && !chip.contains(event.relatedTarget)) chip.classList.remove("is-target");
});

document.addEventListener("drop", (event) => {
  const chip = event.target.closest && event.target.closest("[data-swap-player]");
  if (!chip || !swap.dragging) return;
  event.preventDefault();
  chip.classList.remove("is-target");
  submitSwap(swap.dragging, chip);
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

// Confirmation toast: every action form or button with data-confirm asks "Ya / Tidak" first.
// The submitter is kept so buttons with name/value (e.g. draft vs publish) still send their value,
// and HTMX forms continue through their own submit handling once confirmed.
const confirmToast = {
  dialog: null,
  pending: null,
  open(message, danger, onYes) {
    const d = document.getElementById("confirm-toast");
    if (!d) { if (window.confirm(message)) onYes(); return; }
    d.querySelector("[data-confirm-text]").textContent = message;
    const yes = d.querySelector("[data-confirm-yes]");
    yes.classList.toggle("btn-danger", danger);
    yes.classList.toggle("btn-primary", !danger);
    this.dialog = d;
    this.pending = onYes;
    d.showModal();
    d.querySelector("[data-confirm-no]").focus();
  },
  answer(yes) {
    const run = this.pending;
    this.pending = null;
    if (this.dialog) this.dialog.close();
    if (yes && run) run();
  },
};

// Every POST form asks for confirmation unless it opts out with data-no-confirm (login, fast
// checklists). Without a custom message the question is built from the button label.
function defaultConfirm(form, submitter) {
  if ((form.getAttribute("method") || "get").toLowerCase() !== "post" || form.hasAttribute("data-no-confirm")) return "";
  const label = submitter ? (submitter.getAttribute("aria-label") || submitter.textContent).trim().replace(/\s+/g, " ") : "";
  return label ? `Yakin ${label.charAt(0).toLowerCase()}${label.slice(1)}?` : "Lanjutkan tindakan ini?";
}

document.addEventListener("submit", (event) => {
  const form = event.target;
  const submitter = event.submitter;
  const message = (submitter && submitter.dataset.confirm) || form.dataset.confirm || defaultConfirm(form, submitter);
  if (!message || form.dataset.confirmed === "1") {
    delete form.dataset.confirmed;
    return;
  }
  event.preventDefault();
  event.stopImmediatePropagation();
  const danger = (submitter && submitter.classList.contains("btn-danger")) || form.dataset.confirmTone === "danger";
  confirmToast.open(message, danger, () => {
    form.dataset.confirmed = "1";
    form.requestSubmit(submitter && submitter.form === form ? submitter : undefined);
  });
}, true);

document.addEventListener("click", (event) => {
  if (event.target.closest("[data-confirm-yes]")) confirmToast.answer(true);
  else if (event.target.closest("[data-confirm-no]")) confirmToast.answer(false);
  const thumb = event.target.closest("[data-lightbox-src]");
  if (thumb) {
    const box = document.getElementById("lightbox");
    const img = box.querySelector("[data-lightbox-img]");
    img.src = thumb.dataset.lightboxSrc;
    img.alt = thumb.dataset.lightboxAlt || "";
    box.showModal();
  }
  if (event.target.closest("[data-lightbox-close]") || event.target.id === "lightbox") {
    document.getElementById("lightbox").close();
  }
});

document.addEventListener("cancel", (event) => {
  if (event.target.id === "confirm-toast") confirmToast.answer(false);
}, true);

document.addEventListener("change", (event) => {
  if (event.target.matches("[data-autosubmit]")) event.target.form.requestSubmit();
});

document.addEventListener("htmx:responseError", (event) => {
  const text = event.detail.xhr.responseText;
  if (text && text.length < 300) window.alert(text);
});

window.Alpine = Alpine;
Alpine.start();
