// mockingbird admin UI helpers: dialogs, toasts, theme, copy, bulk selection,
// row editors and body formatting. Everything else is server-rendered + htmx.
(function () {
  "use strict";

  const THEMES = ["system", "light", "dark"];

  function applyTheme(t) {
    if (t === "light" || t === "dark") document.documentElement.setAttribute("data-theme", t);
    else document.documentElement.removeAttribute("data-theme");
    document.querySelectorAll("[data-theme-toggle]").forEach(function (b) {
      b.setAttribute("aria-label", "Theme: " + t);
      b.setAttribute("title", "Theme: " + t);
      b.querySelectorAll("[data-theme-icon]").forEach(function (i) {
        i.hidden = i.getAttribute("data-theme-icon") !== t;
      });
    });
    document.querySelectorAll("input[data-theme-set]").forEach(function (i) { i.checked = i.value === t; });
  }

  function setTheme(t) {
    try { localStorage.setItem("mb-theme", t); } catch (e) { /* private mode */ }
    applyTheme(t);
  }

  function storedTheme() {
    try { return localStorage.getItem("mb-theme") || "system"; } catch (e) { return "system"; }
  }

  // ---- Toasts ----
  const ICONS = {
    success: '<path d="M20 6 9 17l-5-5"/>',
    error: '<circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/>',
    warning: '<circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/>',
    info: '<circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/>'
  };

  function toast(type, msg) {
    const root = document.getElementById("toasts");
    if (!root || !msg) return;
    type = ICONS[type] ? type : "info";
    const el = document.createElement("div");
    el.className = "toast toast--" + type;
    el.setAttribute("role", type === "error" ? "alert" : "status");
    el.innerHTML = '<svg class="icon" viewBox="0 0 24 24" aria-hidden="true">' + ICONS[type] + '</svg>' +
      '<div class="toast__msg"></div>' +
      '<button type="button" class="btn btn--ghost btn--sm btn--icon" data-toast-close aria-label="Dismiss">' +
      '<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg></button>';
    el.querySelector(".toast__msg").textContent = msg;
    root.appendChild(el);
    armToast(el);
  }

  function armToast(el) {
    if (el.dataset.armed) return;
    el.dataset.armed = "1";
    if (!el.classList.contains("toast--error")) setTimeout(function () { el.remove(); }, 5000);
  }

  // ---- Dialogs ----
  function openDialogs(root) {
    (root || document).querySelectorAll("#modal-root dialog.modal:not([open])").forEach(function (d) {
      if (!d.isConnected) return;
      d.showModal();
      const f = d.querySelector("[autofocus]") || d.querySelector("input:not([type=hidden]),select,textarea");
      if (f) f.focus();
      d.addEventListener("close", function () {
        const r = document.getElementById("modal-root");
        if (r && r.contains(d)) r.innerHTML = "";
      }, { once: true });
    });
  }

  function closeDialogs() {
    document.querySelectorAll("#modal-root dialog[open]").forEach(function (d) { d.close(); });
  }

  // ---- Body formatting ----
  function formatXML(src) {
    const doc = new DOMParser().parseFromString(src, "application/xml");
    if (doc.getElementsByTagName("parsererror").length) throw new Error("invalid XML");
    let out = "", depth = 0;
    const tokens = src.replace(/>\s*</g, "><").split(/(?=<)|(?<=>)/);
    tokens.forEach(function (t) {
      if (!t.trim()) return;
      if (/^<\//.test(t)) depth = Math.max(depth - 1, 0);
      const line = "  ".repeat(depth) + t.trim();
      if (/^<[^!?/][^>]*[^/]>$/.test(t) || /^<[^!?/>]>$/.test(t)) {
        out += line + "\n"; depth++;
      } else out += line + "\n";
    });
    // Collapse <a>text</a> back onto one line.
    return out.replace(/(<([\w:.-]+)[^>]*>)\n\s*([^<\n]+)\n\s*(<\/\2>)/g, "$1$3$4").trimEnd();
  }

  function formatBody(ta) {
    const v = ta.value.trim();
    if (!v) return;
    try {
      ta.value = v[0] === "<" ? formatXML(v) : JSON.stringify(JSON.parse(v), null, 2);
      ta.dispatchEvent(new Event("input", { bubbles: true }));
    } catch (e) {
      toast("warning", "Body is not valid " + (v[0] === "<" ? "XML" : "JSON"));
    }
  }

  function insertAtCursor(ta, text) {
    const s = ta.selectionStart, e = ta.selectionEnd;
    ta.setRangeText(text, s, e, "end");
    ta.focus();
    ta.dispatchEvent(new Event("input", { bubbles: true }));
  }

  // ---- Event wiring ----
  document.addEventListener("click", function (ev) {
    const t = ev.target.closest("[data-theme-toggle],[data-copy],[data-toast-close],[data-close-modal],[data-add-row],[data-remove-row],[data-format],[data-wrap],[data-insert],[data-tab],[data-select-all]");
    if (!t) {
      // Close open menus when clicking elsewhere.
      document.querySelectorAll("details.menu[open]").forEach(function (m) {
        if (!m.contains(ev.target)) m.removeAttribute("open");
      });
      return;
    }
    if (t.hasAttribute("data-theme-toggle")) {
      setTheme(THEMES[(THEMES.indexOf(storedTheme()) + 1) % THEMES.length]);
    } else if (t.hasAttribute("data-copy")) {
      const text = t.getAttribute("data-copy");
      if (navigator.clipboard) navigator.clipboard.writeText(text).then(function () {
        t.classList.add("is-copied");
        const prev = t.getAttribute("aria-label");
        t.setAttribute("aria-label", "Copied");
        setTimeout(function () { t.classList.remove("is-copied"); t.setAttribute("aria-label", prev || "Copy"); }, 1500);
        toast("success", "Copied to clipboard");
      });
    } else if (t.hasAttribute("data-toast-close")) {
      t.closest(".toast").remove();
    } else if (t.hasAttribute("data-close-modal")) {
      ev.preventDefault();
      const d = t.closest("dialog");
      if (d) d.close(); else history.back();
    } else if (t.hasAttribute("data-add-row")) {
      const tpl = document.getElementById(t.getAttribute("data-add-row"));
      const body = document.getElementById(t.getAttribute("data-target"));
      const next = parseInt(body.getAttribute("data-next") || "0", 10);
      body.setAttribute("data-next", String(next + 1));
      body.insertAdjacentHTML("beforeend", tpl.innerHTML.replace(/__i__/g, String(next)));
      const first = body.lastElementChild && body.lastElementChild.querySelector("input,select");
      if (first) first.focus();
      markDirty(t);
    } else if (t.hasAttribute("data-remove-row")) {
      markDirty(t);
      t.closest("tr").remove();
    } else if (t.hasAttribute("data-format")) {
      formatBody(document.getElementById(t.getAttribute("data-format")));
    } else if (t.hasAttribute("data-wrap")) {
      const ta = document.getElementById(t.getAttribute("data-wrap"));
      const on = ta.classList.toggle("code--wrap");
      ta.setAttribute("wrap", on ? "soft" : "off");
      t.setAttribute("aria-pressed", String(on));
    } else if (t.hasAttribute("data-insert")) {
      insertAtCursor(document.getElementById(t.getAttribute("data-target")), t.getAttribute("data-insert"));
    } else if (t.hasAttribute("data-tab")) {
      ev.preventDefault();
      const list = t.closest("[role=tablist]");
      list.querySelectorAll("[role=tab]").forEach(function (b) {
        const on = b === t;
        b.setAttribute("aria-selected", String(on));
        const p = document.getElementById(b.getAttribute("aria-controls"));
        if (p) p.hidden = !on;
      });
    } else if (t.hasAttribute("data-select-all")) {
      const form = t.closest("form");
      form.querySelectorAll('input[name="ids"]').forEach(function (c) { c.checked = t.checked; });
      updateBulk(form);
    }
  });

  function updateBulk(form) {
    if (!form) return;
    const boxes = form.querySelectorAll('input[name="ids"]');
    const n = Array.prototype.filter.call(boxes, function (c) { return c.checked; }).length;
    form.querySelectorAll("[data-bulk-count]").forEach(function (el) { el.textContent = n + " selected"; });
    const all = form.querySelector("[data-select-all]");
    if (all) { all.checked = n > 0 && n === boxes.length; all.indeterminate = n > 0 && n < boxes.length; }
  }

  function markDirty(el) {
    const form = el.closest("form[data-dirty-track]");
    if (!form) return;
    form.querySelectorAll("[data-dirty-state]").forEach(function (s) {
      s.setAttribute("data-dirty", "true");
      s.textContent = "Unsaved changes";
    });
  }

  document.addEventListener("change", function (ev) {
    if (ev.target.matches("input[data-theme-set]")) { setTheme(ev.target.value); return; }
    if (ev.target.matches('input[name="ids"]')) updateBulk(ev.target.closest("form"));
    if (ev.target.matches('input[type="file"][data-file-label]')) {
      const out = document.getElementById(ev.target.getAttribute("data-file-label"));
      const f = ev.target.files && ev.target.files[0];
      if (out) out.textContent = f ? f.name + " · " + Math.max(1, Math.round(f.size / 1024)) + " KB" : "";
    }
    markDirty(ev.target);
  });

  document.addEventListener("input", function (ev) {
    markDirty(ev.target);
    const m = ev.target.closest("[data-confirm-name]");
    if (m) {
      const btn = document.getElementById(m.getAttribute("data-confirm-button"));
      if (btn) btn.disabled = ev.target.value !== m.getAttribute("data-confirm-name");
    }
  });

  document.addEventListener("keydown", function (ev) {
    if (ev.key === "Escape") document.querySelectorAll("details.menu[open]").forEach(function (m) { m.removeAttribute("open"); });
  });

  // htmx integration.
  document.addEventListener("htmx:afterSwap", function (ev) {
    openDialogs(ev.target);
  });
  document.addEventListener("htmx:afterSettle", function () {
    document.querySelectorAll(".toast").forEach(armToast);
    applyTheme(storedTheme());
  });
  document.addEventListener("htmx:beforeRequest", function () {
    document.querySelectorAll("details.menu[open]").forEach(function (m) { m.removeAttribute("open"); });
  });
  document.addEventListener("closeModal", closeDialogs);
  document.addEventListener("toast", function (ev) {
    const d = ev.detail || {};
    const v = d.value && typeof d.value === "object" ? d.value : d;
    toast(v.type, v.msg);
  });
  document.addEventListener("htmx:responseError", function (ev) {
    const x = ev.detail.xhr;
    if (x && x.status === 401) return; // HX-Redirect to the login page takes over.
    if (x && x.getResponseHeader("HX-Trigger")) return; // the server sent its own toast.
    toast("error", "Request failed (" + (x ? x.status : "network") + "). Please retry.");
  });
  document.addEventListener("htmx:sendError", function () {
    toast("error", "Server unreachable. Please retry.");
  });

  document.addEventListener("DOMContentLoaded", function () {
    applyTheme(storedTheme());
    document.querySelectorAll(".toast").forEach(armToast);
    openDialogs(document);
  });

  window.mockingbird = { toast: toast };
})();
