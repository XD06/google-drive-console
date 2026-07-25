/* Drive Backup Console — interactive UI demo (mock data only) */

const FS = {
  root: {
    id: "root",
    name: "My Drive",
    type: "folder",
    children: [
      {
        id: "f-backup",
        name: "Backups",
        type: "folder",
        modified: "2026-07-18 14:22",
        children: [
          {
            id: "file-pg",
            name: "postgres-2026-07-18.dump",
            type: "file",
            size: 2147483648,
            modified: "2026-07-18 14:22",
          },
          {
            id: "file-cfg",
            name: "nginx.conf.bak",
            type: "file",
            size: 4096,
            modified: "2026-07-12 09:01",
          },
          {
            id: "f-weekly",
            name: "weekly",
            type: "folder",
            modified: "2026-07-10 20:00",
            children: [
              {
                id: "file-w1",
                name: "home-2026-07-06.tar.gz",
                type: "file",
                size: 8589934592,
                modified: "2026-07-06 03:12",
              },
            ],
          },
        ],
      },
      {
        id: "f-photos",
        name: "Camera",
        type: "folder",
        modified: "2026-06-02 11:40",
        children: [],
      },
      {
        id: "file-readme",
        name: "README-backup.txt",
        type: "file",
        size: 820,
        modified: "2026-05-01 08:00",
      },
    ],
  },
};

const el = {
  gate: document.getElementById("gate"),
  shell: document.getElementById("shell"),
  crumbs: document.getElementById("crumbs"),
  tbody: document.getElementById("file-tbody"),
  empty: document.getElementById("empty-state"),
  tableWrap: document.getElementById("table-wrap"),
  rail: document.getElementById("rail"),
  railBody: document.getElementById("rail-body"),
  railEmpty: document.getElementById("rail-empty"),
  jobCount: document.getElementById("job-count"),
  drop: document.getElementById("drop-overlay"),
  main: document.getElementById("main-drop"),
  fileInput: document.getElementById("file-input"),
  footerSel: document.getElementById("footer-sel"),
  footerItems: document.getElementById("footer-items"),
  toastHost: document.getElementById("toast-host"),
  btnUp: document.getElementById("btn-up"),
  btnToggleRail: document.getElementById("btn-toggle-rail"),
};

/** @type {{id:string,name:string}[]} */
let path = [{ id: "root", name: "My Drive" }];
/** @type {Set<string>} */
let selected = new Set();
/** @type {Map<string, object>} */
const jobs = new Map();
let jobSeq = 0;

function findNode(id, node = FS.root) {
  if (node.id === id) return node;
  if (!node.children) return null;
  for (const c of node.children) {
    const hit = findNode(id, c);
    if (hit) return hit;
  }
  return null;
}

function currentFolder() {
  return findNode(path[path.length - 1].id);
}

function formatSize(n) {
  if (n == null) return "—";
  const u = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v < 10 && i > 0 ? v.toFixed(1) : Math.round(v)} ${u[i]}`;
}

function iconSvg(type) {
  if (type === "folder") {
    return `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true"><path d="M4 7h4l2-2h8a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V9a2 2 0 0 1 2-2z"/></svg>`;
  }
  return `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true"><path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5"/></svg>`;
}

const prefs = {
  banner: true,
  uploadToast: true,
  railDefault: true,
  compact: false,
};

function toast(msg, isErr, title) {
  if (!prefs.banner && !isErr) return;
  const t = document.createElement("div");
  t.className = "toast" + (isErr ? " is-err" : "");
  t.setAttribute("role", isErr ? "alert" : "status");
  const iconOk =
    '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true"><path d="M20 6L9 17l-5-5"/></svg>';
  const iconErr =
    '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true"><path d="M12 8v5M12 17h.01"/><circle cx="12" cy="12" r="9"/></svg>';
  const heading = title || (isErr ? "Something went wrong" : "Notification");
  t.innerHTML = `
    <span class="toast-icon" aria-hidden="true">${isErr ? iconErr : iconOk}</span>
    <div class="toast-body">
      <span class="toast-title">${heading}</span>
      <span class="toast-msg"></span>
    </div>
    <button type="button" class="toast-close" aria-label="Dismiss">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true"><path d="M6 6l12 12M18 6L6 18"/></svg>
    </button>
  `;
  t.querySelector(".toast-msg").textContent = msg;
  const dismiss = () => {
    if (!t.isConnected) return;
    t.classList.remove("is-in");
    t.classList.add("is-out");
    setTimeout(() => t.remove(), 180);
  };
  t.querySelector(".toast-close").addEventListener("click", dismiss);
  el.toastHost.appendChild(t);
  requestAnimationFrame(() => {
    requestAnimationFrame(() => t.classList.add("is-in"));
  });
  setTimeout(dismiss, 3200);
}

function openSettings() {
  const sheet = document.getElementById("settings-sheet");
  const overlay = document.getElementById("settings-overlay");
  const btn = document.getElementById("btn-settings");
  if (!sheet || !overlay) return;
  sheet.hidden = false;
  overlay.hidden = false;
  requestAnimationFrame(() => {
    sheet.classList.add("is-on");
    overlay.classList.add("is-on");
  });
  document.body.classList.add("settings-open");
  btn?.setAttribute("aria-expanded", "true");
  document.getElementById("btn-settings-close")?.focus();
}

function closeSettings() {
  const sheet = document.getElementById("settings-sheet");
  const overlay = document.getElementById("settings-overlay");
  const btn = document.getElementById("btn-settings");
  if (!sheet || !overlay) return;
  sheet.classList.remove("is-on");
  overlay.classList.remove("is-on");
  document.body.classList.remove("settings-open");
  btn?.setAttribute("aria-expanded", "false");
  setTimeout(() => {
    if (!sheet.classList.contains("is-on")) {
      sheet.hidden = true;
      overlay.hidden = true;
    }
  }, 260);
  btn?.focus();
}


function renderCrumbs() {
  el.crumbs.innerHTML = "";
  path.forEach((p, i) => {
    if (i > 0) {
      const sep = document.createElement("span");
      sep.className = "crumb-sep";
      sep.setAttribute("aria-hidden", "true");
      sep.textContent = "/";
      el.crumbs.appendChild(sep);
    }
    const b = document.createElement("button");
    b.type = "button";
    b.className = "crumb" + (i === path.length - 1 ? " is-current" : "");
    b.textContent = p.name;
    if (i === path.length - 1) {
      b.setAttribute("aria-current", "page");
    } else {
      b.addEventListener("click", () => {
        path = path.slice(0, i + 1);
        selected.clear();
        render();
      });
    }
    el.crumbs.appendChild(b);
  });
  el.btnUp.disabled = path.length <= 1;
}

function renderTable() {
  const folder = currentFolder();
  const items = folder?.children ? [...folder.children] : [];
  items.sort((a, b) => {
    if (a.type !== b.type) return a.type === "folder" ? -1 : 1;
    return a.name.localeCompare(b.name);
  });

  el.tbody.innerHTML = "";
  el.empty.classList.toggle("is-on", items.length === 0);
  el.tableWrap.querySelector(".file-table").style.display = items.length ? "" : "none";

  for (const item of items) {
    const tr = document.createElement("tr");
    tr.dataset.id = item.id;
    if (item.type === "folder") tr.classList.add("is-folder");
    if (selected.has(item.id)) tr.classList.add("is-selected");
    tr.setAttribute("aria-selected", selected.has(item.id) ? "true" : "false");

    tr.innerHTML = `
      <td>
        <div class="name-cell">
          <span class="icon-box ${item.type}" aria-hidden="true">${iconSvg(item.type)}</span>
          <span title="${item.name}">${item.name}</span>
        </div>
      </td>
      <td class="num">${item.type === "folder" ? "—" : formatSize(item.size)}</td>
      <td class="num">${item.modified || "—"}</td>
      <td>
        <div class="row-actions">
          ${
            item.type === "file"
              ? `<button type="button" class="btn btn-ghost btn-sm act-dl">Download</button>`
              : `<button type="button" class="btn btn-ghost btn-sm act-open">Open</button>`
          }
        </div>
      </td>
    `;

    tr.addEventListener("click", (e) => {
      if (e.target.closest("button")) return;
      if (e.metaKey || e.ctrlKey) {
        if (selected.has(item.id)) selected.delete(item.id);
        else selected.add(item.id);
      } else {
        selected = new Set([item.id]);
      }
      renderTable();
      updateFooter(items.length);
    });

    tr.addEventListener("dblclick", () => {
      if (item.type === "folder") openFolder(item);
    });

    tr.querySelector(".act-open")?.addEventListener("click", (e) => {
      e.stopPropagation();
      openFolder(item);
    });
    tr.querySelector(".act-dl")?.addEventListener("click", (e) => {
      e.stopPropagation();
      toast(`Download started: ${item.name}`);
    });

    el.tbody.appendChild(tr);
  }

  updateFooter(items.length);
}

function openFolder(item) {
  path.push({ id: item.id, name: item.name });
  selected.clear();
  render();
}

function updateFooter(n) {
  el.footerSel.textContent = `${selected.size} selected`;
  el.footerItems.textContent = `${n} items`;
}

function renderJobs() {
  const list = [...jobs.values()].reverse();
  el.jobCount.textContent = String(list.filter((j) => j.status === "running").length);
  el.railEmpty.style.display = list.length ? "none" : "";

  el.railBody.querySelectorAll(".job").forEach((n) => n.remove());
  for (const j of list) {
    const d = document.createElement("div");
    d.className = "job" + (j.status === "done" ? " is-done" : j.status === "error" ? " is-err" : "");
    d.dataset.jobId = j.id;
    d.innerHTML = `
      <div class="job-top">
        <span class="job-name" title="${j.name}">${j.name}</span>
        <span class="job-meta">${j.status === "done" ? "done" : j.status === "error" ? "error" : j.pct + "%"}</span>
      </div>
      <div class="bar" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${j.pct}" aria-label="${j.name}"><i style="width:${j.pct}%"></i></div>
    `;
    el.railBody.appendChild(d);
  }
}

function startMockUpload(files) {
  const folder = currentFolder();
  if (!folder.children) folder.children = [];

  if (el.rail.classList.contains("hidden")) {
    el.rail.classList.remove("hidden");
    document.getElementById("main-row")?.classList.remove("rail-off");
    el.btnToggleRail?.setAttribute("aria-expanded", "true");
  }

  for (const file of files) {
    const id = `job-${++jobSeq}`;
    const job = {
      id,
      name: file.name || "upload.bin",
      size: file.size || Math.floor(Math.random() * 5e8) + 1e6,
      pct: 0,
      status: "running",
    };
    jobs.set(id, job);
    renderJobs();

    const step = () => {
      const j = jobs.get(id);
      if (!j || j.status !== "running") return;
      j.pct = Math.min(100, j.pct + Math.floor(Math.random() * 12) + 4);
      if (j.pct >= 100) {
        j.pct = 100;
        j.status = "done";
        const now = new Date();
        const stamp = now.toISOString().slice(0, 16).replace("T", " ");
        folder.children.push({
          id: `up-${id}`,
          name: j.name,
          type: "file",
          size: j.size,
          modified: stamp,
        });
        if (prefs.uploadToast) toast(`Uploaded ${j.name}`, false, "Upload complete");
        renderTable();
      }
      renderJobs();
      if (j.status === "running") setTimeout(step, 280 + Math.random() * 200);
    };
    setTimeout(step, 200);
  }
}

function connect() {
  el.gate.style.display = "none";
  el.shell.classList.add("is-on");
  render();
  toast("Connected as xxt221673@gmail.com", false, "Signed in");
}

function disconnect() {
  el.shell.classList.remove("is-on");
  el.gate.style.display = "";
  path = [{ id: "root", name: "My Drive" }];
  selected.clear();
  toast("Session ended", false, "Disconnected");
}

function render() {
  renderCrumbs();
  renderTable();
  renderJobs();
}

// —— events ——
document.getElementById("btn-connect").addEventListener("click", connect);
document.getElementById("btn-disconnect").addEventListener("click", disconnect);
document.getElementById("btn-settings")?.addEventListener("click", openSettings);
document.getElementById("btn-settings-close")?.addEventListener("click", closeSettings);
document.getElementById("settings-overlay")?.addEventListener("click", closeSettings);
document.getElementById("btn-settings-disconnect")?.addEventListener("click", () => {
  closeSettings();
  disconnect();
});
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && document.body.classList.contains("settings-open")) {
    closeSettings();
  }
});
const setBanner = document.getElementById("set-banner");
const setUpload = document.getElementById("set-upload-toast");
const setRail = document.getElementById("set-rail-default");
const setCompact = document.getElementById("set-compact");
setBanner?.addEventListener("change", () => {
  prefs.banner = setBanner.checked;
  toast(prefs.banner ? "Banner alerts on" : "Banner alerts off", false, "Settings");
});
setUpload?.addEventListener("change", () => {
  prefs.uploadToast = setUpload.checked;
});
setRail?.addEventListener("change", () => {
  prefs.railDefault = setRail.checked;
  const off = !prefs.railDefault;
  el.rail.classList.toggle("hidden", off);
  document.getElementById("main-row")?.classList.toggle("rail-off", off);
  el.btnToggleRail?.setAttribute("aria-expanded", off ? "false" : "true");
});
setCompact?.addEventListener("change", () => {
  prefs.compact = setCompact.checked;
  document.body.classList.toggle("is-compact", prefs.compact);
});
el.btnUp.addEventListener("click", () => {
  if (path.length > 1) {
    path.pop();
    selected.clear();
    render();
  }
});
document.getElementById("btn-refresh").addEventListener("click", () => {
  render();
  toast("Folder list updated", false, "Refreshed");
});
document.getElementById("btn-upload").addEventListener("click", () => el.fileInput.click());
document.getElementById("btn-empty-upload").addEventListener("click", () => el.fileInput.click());
el.fileInput.addEventListener("change", () => {
  if (el.fileInput.files?.length) {
    startMockUpload([...el.fileInput.files]);
    el.fileInput.value = "";
  }
});
document.getElementById("btn-new-folder").addEventListener("click", () => {
  const name = prompt("Folder name", "New folder");
  if (!name) return;
  const folder = currentFolder();
  if (!folder.children) folder.children = [];
  folder.children.push({
    id: `nf-${Date.now()}`,
    name: name.trim(),
    type: "folder",
    modified: new Date().toISOString().slice(0, 16).replace("T", " "),
    children: [],
  });
  renderTable();
  toast(`Created ${name.trim()}`, false, "New folder");
});
const mainRow = document.getElementById("main-row");
el.btnToggleRail?.addEventListener("click", () => {
  el.rail.classList.toggle("hidden");
  const off = el.rail.classList.contains("hidden");
  mainRow?.classList.toggle("rail-off", off);
  el.btnToggleRail.setAttribute("aria-expanded", off ? "false" : "true");
});
document.getElementById("nav-drive")?.addEventListener("click", () => {
  path = [{ id: "root", name: "My Drive" }];
  selected.clear();
  render();
});

// drag-drop
let dragDepth = 0;
el.main.addEventListener("dragenter", (e) => {
  e.preventDefault();
  dragDepth++;
  el.drop.classList.add("is-on");
});
el.main.addEventListener("dragleave", (e) => {
  e.preventDefault();
  dragDepth = Math.max(0, dragDepth - 1);
  if (dragDepth === 0) el.drop.classList.remove("is-on");
});
el.main.addEventListener("dragover", (e) => e.preventDefault());
el.main.addEventListener("drop", (e) => {
  e.preventDefault();
  dragDepth = 0;
  el.drop.classList.remove("is-on");
  if (e.dataTransfer?.files?.length) startMockUpload([...e.dataTransfer.files]);
});
