import {
  lazy,
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type DragEvent,
} from "react";
import {
  ApiError,
  cancelUpload,
  copyFile,
  fileDownloadUrl,
fileThumbnailUrl,
  fetchMe,
  formatBytes,
  listFiles,
  logout,
  uploadFile,
  createFolder,
  trashFile,
  renameFile,
  updateFile,
  moveFile,
  fetchFileContent,
  saveFileContent,
  isTextPreviewable,
  isImagePreviewable,
  isPdfPreviewable,
  isVideoPreviewable,
  fetchFileBlob,
  createFile,
  mimeFromFilename,
  searchFiles,
  simpleUpload,
  SIMPLE_UPLOAD_THRESHOLD,
  zipDownloadUrl,
  batchTrash,
  downloadMultiZip,
  type FileItem,
  type MeResponse,
  type SearchScope,
  fetchOverview,
  type OverviewResponse,
} from "./lib/api";
import { fileKind, fileKindLabel, type FileKind } from "./lib/fileKind";
import { FileTypeIcon, KindIcon, iconBoxClass } from "./lib/FileTypeIcon";
import {
  IconCheck,
  IconChevronDown,
  IconChevronLeft,
  IconChevronUp,
  IconClose,
  IconCopy,
  IconDownload,
  IconDrive,
  IconFilePlus,
  IconFilter,
  IconFolderPlus,
  IconHelp,
  IconHistory,
  IconMenu,
  IconMoon,
  IconMove,
  IconOpen,
  IconRefresh,
  IconRename,
  IconSearch,
  IconSettings,
  IconShare,
  IconSun,
  IconTrash,
  IconUpload,
} from "./lib/icons";
import { ConfirmDialog } from "./components/ConfirmDialog";
import { FileRow } from "./components/FileRow";
import { MobileNav } from "./components/MobileNav";
import { Sidebar } from "./components/Sidebar";
import { ToastHost } from "./components/ToastHost";
import { UploadToastHost } from "./components/UploadToastHost";
import { useLocalStorage } from "./hooks/useLocalStorage";
import { useTheme } from "./hooks/useTheme";
import { useWallpaper } from "./hooks/useWallpaper";

// Code-split heavy overlays: they load on first open, keeping the initial
// bundle (and thus first paint / folder list) lean.
const MediaLightbox = lazy(() =>
  import("./components/MediaLightbox").then((m) => ({ default: m.MediaLightbox })),
);
const SettingsSheet = lazy(() =>
  import("./components/SettingsSheet").then((m) => ({ default: m.SettingsSheet })),
);
const ShareDialog = lazy(() =>
  import("./components/ShareDialog").then((m) => ({ default: m.ShareDialog })),
);
const ShortcutHelp = lazy(() =>
  import("./components/ShortcutHelp").then((m) => ({ default: m.ShortcutHelp })),
);
const RevisionsPanel = lazy(() =>
  import("./components/RevisionsPanel").then((m) => ({ default: m.RevisionsPanel })),
);

type AuthState =
  | { status: "loading" }
  | { status: "signed_out" }
  | { status: "signed_in"; me: MeResponse };

type UploadJob = {
  id: string;
  name: string;
  received: number;
  total: number;
  status: string;
  error?: string;
  serverId?: string;
  cancelling?: boolean;
};

type SortKey = "name" | "size" | "modified";
type SortDir = "asc" | "desc";

type ConfirmState = {
  title: string;
  message: string;
  confirmLabel?: string;
  danger?: boolean;
  resolve: (ok: boolean) => void;
} | null;

type ToastItem = {
  id: string;
  title: string;
  msg: string;
  isErr: boolean;
  phase: "enter" | "in" | "out";
};

let uploadSeq = 0;
let toastSeq = 0;

// --- Navigation state persistence (survive page refresh) ---
type NavState = {
  v: 1;
  folderId?: string;
  trail: { id: string; name: string }[];
  view: "files" | "overview";
};
const NAV_KEY = "dbc.nav";

function loadNavState(): NavState | null {
  try {
    const raw = localStorage.getItem(NAV_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as NavState;
    if (parsed?.v !== 1 || !Array.isArray(parsed.trail)) return null;
    return parsed;
  } catch {
    return null;
  }
}

function saveNavState(nav: NavState) {
  try {
    localStorage.setItem(NAV_KEY, JSON.stringify(nav));
  } catch {
    // Quota/private-mode errors are non-fatal.
  }
}

function clearNavState() {
  try {
    localStorage.removeItem(NAV_KEY);
  } catch {
    // ignore
  }
}

export default function App() {
  const [auth, setAuth] = useState<AuthState>({ status: "loading" });
  const [folderId, setFolderId] = useState<string | undefined>(undefined);
  const [trail, setTrail] = useState<{ id: string; name: string }[]>([]);
  const [items, setItems] = useState<FileItem[]>([]);
  const [listError, setListError] = useState<string | null>(null);
  const [listLoading, setListLoading] = useState(false);
  const [listNextToken, setListNextToken] = useState<string | null>(null);
  const [listLoadingMore, setListLoadingMore] = useState(false);
const [renderLimit, setRenderLimit] = useState(200); // F2: progressive rendering
  const [busy, setBusy] = useState(false);
  const [jobs, setJobs] = useState<UploadJob[]>([]);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [prefs, setPrefs] = useLocalStorage("dbc.prefs", { banner: true, uploadToast: true, compact: false, autoDismiss: true });
  const { mode: themeMode, setMode: setThemeMode } = useTheme();
  const { wallpaper, setWallpaper } = useWallpaper();
  const [view, setView] = useState<"files" | "overview">("files");
  const [navOpen, setNavOpen] = useState(false);
  const [rowMenuId, setRowMenuId] = useState<string | null>(null);
  const [overview, setOverview] = useState<OverviewResponse | null>(null);
  const [overviewLoading, setOverviewLoading] = useState(false);
  const [overviewError, setOverviewError] = useState<string | null>(null);
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const [dropOn, setDropOn] = useState(false);
  const [sortKey, setSortKey] = useLocalStorage<SortKey>("dbc.sortKey", "name");
  const [sortDir, setSortDir] = useLocalStorage<SortDir>("dbc.sortDir", "asc");
  const [confirm, setConfirm] = useState<ConfirmState>(null);
  // P0/P1 new state
  const [shortcutHelpOpen, setShortcutHelpOpen] = useState(false);
  const [shareItem, setShareItem] = useState<FileItem | null>(null);
  const [revisionsItem, setRevisionsItem] = useState<FileItem | null>(null);
  const [typeFilter, setTypeFilter] = useState<string>("all");
  const [recentFolders, setRecentFolders] = useLocalStorage<{ id: string; name: string }[]>("dbc.recentFolders", []);
  const [dragItemId, setDragItemId] = useState<string | null>(null);
  const [dragOverFolder, setDragOverFolder] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const prefsRef = useRef(prefs);
  const dragDepthRef = useRef(0);
  const uploadAbortRef = useRef<Map<string, AbortController>>(new Map());
  prefsRef.current = prefs;

  type EditorState = {
    id: string | null;
    name: string;
    mimeType: string;
    content: string;
    original: string;
    loading: boolean;
    saving: boolean;
    error: string | null;
  } | null;
  const [editor, setEditor] = useState<EditorState>(null);

  const [saveAsOpen, setSaveAsOpen] = useState(false);
  const [saveAsName, setSaveAsName] = useState("");
  const [saveAsBusy, setSaveAsBusy] = useState(false);

  const [mkdirOpen, setMkdirOpen] = useState(false);
  const [mkdirName, setMkdirName] = useState("");
  const [mkdirBusy, setMkdirBusy] = useState(false);
  const [ctxMenu, setCtxMenu] = useState<null | { x: number; y: number; item: FileItem | null }>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  const [renameOpen, setRenameOpen] = useState(false);
  const [renameName, setRenameName] = useState("");
  const [renameItem, setRenameItem] = useState<FileItem | null>(null);
  const [renameBusy, setRenameBusy] = useState(false);
  const [descOpen, setDescOpen] = useState(false);
  const [descValue, setDescValue] = useState("");
  const [descItem, setDescItem] = useState<FileItem | null>(null);
  const [descBusy, setDescBusy] = useState(false);
  const [moveOpen, setMoveOpen] = useState(false);
  const [moveItem, setMoveItem] = useState<FileItem | null>(null);
  const [moveTarget, setMoveTarget] = useState("root");
  const [moveBusy, setMoveBusy] = useState(false);
  // Copy-to dialog (same pattern as Move)
  const [copyOpen, setCopyOpen] = useState(false);
  const [copyItemState, setCopyItemState] = useState<FileItem | null>(null);
  const [copyTarget, setCopyTarget] = useState("root");
  const [copyBusy, setCopyBusy] = useState(false);

  type ImagePreviewState = {
    id: string;
    name: string;
    mimeType?: string;
    url: string | null;
    loading: boolean;
    error: string | null;
  } | null;
  const [imagePreview, setImagePreview] = useState<ImagePreviewState>(null);
  const imageUrlRef = useRef<string | null>(null);

  const [searchQ, setSearchQ] = useState("");
  const [searchScope, setSearchScope] = useState<SearchScope>("drive");
  const [searchModalOpen, setSearchModalOpen] = useState(false);
  const [searchLoading, setSearchLoading] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);
  const [searchSuggest, setSearchSuggest] = useState<FileItem[]>([]);
  const [searchMode, setSearchMode] = useState(false);
  const [searchResults, setSearchResults] = useState<FileItem[]>([]);
  const [searchNextToken, setSearchNextToken] = useState<string | null>(null);
  const [searchActiveQuery, setSearchActiveQuery] = useState("");
  const searchAbortRef = useRef<AbortController | null>(null);
  const searchDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  function patchJob(id: string, patch: Partial<UploadJob>) {
    setJobs((prev) => prev.map((j) => (j.id === id ? { ...j, ...patch } : j)));
  }

  function askConfirm(opts: {
    title: string;
    message: string;
    confirmLabel?: string;
    danger?: boolean;
  }): Promise<boolean> {
    return new Promise((resolve) => {
      setConfirm({
        title: opts.title,
        message: opts.message,
        confirmLabel: opts.confirmLabel,
        danger: opts.danger,
        resolve,
      });
    });
  }

  function closeConfirm(ok: boolean) {
    setConfirm((cur) => {
      cur?.resolve(ok);
      return null;
    });
  }

  function toggleSort(key: SortKey) {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
      return;
    }
    setSortKey(key);
    setSortDir(key === "modified" ? "desc" : "asc");
  }

  function sortItems(list: FileItem[]): FileItem[] {
    const dir = sortDir === "asc" ? 1 : -1;
    return [...list].sort((a, b) => {
      if (a.isFolder !== b.isFolder) return a.isFolder ? -1 : 1;
      if (sortKey === "name") {
        return a.name.localeCompare(b.name, undefined, { sensitivity: "base" }) * dir;
      }
      if (sortKey === "size") {
        const as = a.isFolder ? -1 : a.size ?? 0;
        const bs = b.isFolder ? -1 : b.size ?? 0;
        if (as === bs) {
          return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
        }
        return (as - bs) * dir;
      }
      const at = Date.parse(a.modifiedTime || "") || 0;
      const bt = Date.parse(b.modifiedTime || "") || 0;
      if (at === bt) {
        return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
      }
      return (at - bt) * dir;
    });
  }

  // Upload history persisted to localStorage for Overview uploads card
  type UploadHistoryItem = { name: string; status: string; at: string; error?: string };
  const [uploadHistory, setUploadHistory] = useState<UploadHistoryItem[]>([]);
  const UPLOAD_HISTORY_KEY = "dbc_upload_history";

  function loadUploadHistory() {
    try {
      const raw = localStorage.getItem(UPLOAD_HISTORY_KEY);
      if (!raw) return [] as UploadHistoryItem[];
      const parsed = JSON.parse(raw) as UploadHistoryItem[];
      return Array.isArray(parsed) ? parsed : [];
    } catch {
      return [] as UploadHistoryItem[];
    }
  }
  function saveUploadHistory(h: UploadHistoryItem[]) {
    try {
      localStorage.setItem(UPLOAD_HISTORY_KEY, JSON.stringify(h.slice(0, 50)));
    } catch {
      // ignore
    }
  }
  function appendUploadHistory(it: UploadHistoryItem) {
    setUploadHistory((prev) => {
      const next = [it, ...prev].slice(0, 50);
      saveUploadHistory(next);
      return next;
    });
  }

  function dismissToast(id: string) {
    setToasts((prev) =>
      prev.map((t) => (t.id === id ? { ...t, phase: "out" } : t)),
    );
    window.setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, 180);
  }

  function showToast(msg: string, isErr = false, title?: string) {
    const p = prefsRef.current;
    if (!p.banner && !isErr) return;
    const id = `toast-${++toastSeq}`;
    const item: ToastItem = {
      id,
      msg,
      isErr,
      title: title || (isErr ? "Something went wrong" : "Notification"),
      phase: "enter",
    };
    setToasts((prev) => [...prev, item]);
    // setTimeout (not rAF): rAF is throttled in background tabs / automation
    window.setTimeout(() => {
      setToasts((prev) =>
        prev.map((t) => (t.id === id && t.phase === "enter" ? { ...t, phase: "in" } : t)),
      );
    }, 20);
    window.setTimeout(() => dismissToast(id), 3200);
  }

  const refreshAuth = useCallback(async () => {
    try {
      const me = await fetchMe();
      setAuth({ status: "signed_in", me });
      try {
        sessionStorage.setItem("dbc.me", JSON.stringify(me));
      } catch {
        /* ignore */
      }
    } catch {
      try {
        sessionStorage.removeItem("dbc.me");
      } catch {
        /* ignore */
      }
      setAuth({ status: "signed_out" });
    }
  }, []);

  useEffect(() => {
    if (!navOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setNavOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [navOpen]);

  useEffect(() => {
    if (!rowMenuId) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setRowMenuId(null);
    };
    const onPointer = (e: PointerEvent) => {
      const t = e.target as HTMLElement | null;
      if (t?.closest?.(".row-more")) return;
      setRowMenuId(null);
    };
    window.addEventListener("keydown", onKey);
    window.addEventListener("pointerdown", onPointer);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("pointerdown", onPointer);
    };
  }, [rowMenuId]);

  useEffect(() => {
    const onResize = () => {
      if (window.innerWidth > 900) setNavOpen(false);
    };
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  useEffect(() => {
    // Avoid full-page gate flash on refresh when session was already valid
    try {
      const raw = sessionStorage.getItem("dbc.me");
      if (raw) {
        const me = JSON.parse(raw) as MeResponse;
        if (me?.connected && me?.email) {
          setAuth({ status: "signed_in", me });
        }
      }
    } catch {
      /* ignore */
    }
    // load persisted upload history
    setUploadHistory(loadUploadHistory());
    void refreshAuth();
  }, [refreshAuth]);

  // Simple in-memory cache for stale-while-revalidate (LRU capped at 30 entries)
  const folderCacheRef = useRef<Map<string, { items: FileItem[]; folderId: string; nextPageToken: string | null }>>(new Map());
  const FOLDER_CACHE_MAX = 30;
  function folderCacheSet(key: string, value: { items: FileItem[]; folderId: string; nextPageToken: string | null }) {
    const cache = folderCacheRef.current;
    // Delete first to refresh insertion order (Map preserves order)
    cache.delete(key);
    cache.set(key, value);
    // Evict oldest entries if over limit
    while (cache.size > FOLDER_CACHE_MAX) {
      const oldest = cache.keys().next().value;
      if (oldest !== undefined) cache.delete(oldest);
      else break;
    }
  }

  // AbortController for list requests — cancels stale fetches on rapid navigation
  const listAbortRef = useRef<AbortController | null>(null);

  const loadFiles = useCallback(async (fid?: string) => {
    // Cancel any in-flight list request
    listAbortRef.current?.abort();
    const ac = new AbortController();
    listAbortRef.current = ac;

    const cacheKey = fid ?? "root";
    const cached = folderCacheRef.current.get(cacheKey);
    if (cached) {
      // Show cached data immediately (no loading screen)
      setItems(cached.items);
      setFolderId(cached.folderId);
      setListNextToken(cached.nextPageToken);
      setSelectedIds(new Set());
      setSelectedId(null);
      setListError(null);
      setRenderLimit(200); // F2: reset progressive render limit
      // Don't set loading=true; fetch silently in background
    } else {
      setListLoading(true);
      setListError(null);
      setListNextToken(null);
    }
    try {
      const res = await listFiles(fid ? { folderId: fid } : {}, ac.signal);
      // If this request was superseded, discard the result
      if (ac.signal.aborted) return;
      setItems(res.items);
      setFolderId(res.folderId);
      setListNextToken(res.nextPageToken ?? null);
      setSelectedIds(new Set());
      setSelectedId(null);
      // Cache the result (LRU)
      folderCacheSet(cacheKey, { items: res.items, folderId: res.folderId, nextPageToken: res.nextPageToken ?? null });
    } catch (e) {
      if (ac.signal.aborted) return; // cancelled — ignore
      if (e instanceof ApiError && e.status === 401) {
        setAuth({ status: "signed_out" });
        setListError(null);
      } else {
        setListError(e instanceof Error ? e.message : "Failed to list files");
      }
      if (!cached) {
        setItems([]);
        setListNextToken(null);
      }
    } finally {
      if (!ac.signal.aborted) setListLoading(false);
    }
  }, []);

// Patch rows in place (live list + folder cache) right after a mutation, so the
// background revalidation triggered afterwards never re-shows stale rows. This
// fixes the visible "flash" where e.g. a renamed row briefly reverted to its
// old name (shown from the stale cache) before the refetch landed.
function patchItems(updater: (items: FileItem[]) => FileItem[]) {
setItems((prev) => updater(prev));
const key = folderId ?? "root";
const cached = folderCacheRef.current.get(key);
if (cached) {
  folderCacheSet(key, { ...cached, items: updater(cached.items) });
}
}

const loadMoreFiles = useCallback(async () => {
    if (!listNextToken || listLoadingMore || listLoading) return;
    setListLoadingMore(true);
    try {
      const res = await listFiles({
        folderId: folderId && folderId !== "root" ? folderId : undefined,
        pageToken: listNextToken,
      });
      setItems((prev) => {
        const seen = new Set(prev.map((it) => it.id));
        const merged = [...prev];
        for (const it of res.items) {
          if (!seen.has(it.id)) merged.push(it);
        }
        return merged;
      });
      setListNextToken(res.nextPageToken ?? null);
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        setAuth({ status: "signed_out" });
      } else {
        showToast(e instanceof Error ? e.message : "Failed to load more", true);
      }
    } finally {
      setListLoadingMore(false);
    }
  }, [folderId, listLoading, listLoadingMore, listNextToken]);

useEffect(() => {
if (auth.status !== "signed_in") return;
// Restore where the user was before the refresh instead of always going to root.
const nav = loadNavState();
if (nav && (nav.folderId || nav.trail.length > 0)) {
  const fid = nav.folderId && nav.folderId !== "root" ? nav.folderId : undefined;
  setTrail(nav.trail);
  setView(nav.view === "overview" ? "overview" : "files");
  void loadFiles(fid);
} else {
  void loadFiles(undefined);
  setTrail([]);
}
}, [auth.status, loadFiles]);

// Keep the persisted location in sync as the user navigates.
useEffect(() => {
if (auth.status !== "signed_in") return;
saveNavState({ v: 1, folderId, trail, view });
}, [auth.status, folderId, trail, view]);

  const loadOverview = useCallback(async () => {
    setOverviewLoading(true);
    setOverviewError(null);
    try {
      const data = await fetchOverview();
      setOverview(data);
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        setAuth({ status: "signed_out" });
        return;
      }
      setOverviewError(e instanceof Error ? e.message : "Failed to load overview");
    } finally {
      setOverviewLoading(false);
    }
  }, []);

  useEffect(() => {
    if (auth.status === "signed_in" && view === "overview") {
      void loadOverview();
    }
  }, [auth.status, view, loadOverview]);

  useEffect(() => {
    document.body.classList.toggle("settings-open", settingsOpen);
    document.body.classList.toggle("is-compact", prefs.compact);
    return () => {
      document.body.classList.remove("settings-open");
    };
  }, [settingsOpen, prefs.compact]);

  useEffect(() => {
    if (!settingsOpen) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setSettingsOpen(false);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [settingsOpen]);

  useEffect(() => {
    function onDocClick() {
      setCtxMenu(null);
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setCtxMenu(null);
    }
    document.addEventListener("click", onDocClick);
    window.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("click", onDocClick);
      window.removeEventListener("keydown", onKey);
    };
  }, []);

  async function onLogout() {
    setBusy(true);
    try {
await logout(false);
setAuth({ status: "signed_out" });
setItems([]);
setTrail([]);
setFolderId(undefined);
setJobs([]);
clearNavState();
      showToast("Session ended", false, "Disconnected");
    } catch (e) {
      setListError(e instanceof Error ? e.message : "Logout failed");
      showToast(e instanceof Error ? e.message : "Logout failed", true);
    } finally {
      setBusy(false);
    }
  }

  // Guards against duplicate folder activations. On touch devices a double-tap
  // (or a second tap while the list is still loading) fires onActivate more
  // than once for the same folder; without a guard each call appends a crumb
  // and refetches, producing duplicated breadcrumbs ("X / X").
  const lastFolderOpenRef = useRef<{ id: string; at: number }>({ id: "", at: 0 });

  function openFolder(item: FileItem) {
    if (!item.isFolder) return;
    const fromSearch = searchMode;
    if (!fromSearch) {
      // Already inside (or already entering) this folder — ignore the duplicate.
      if (folderId === item.id || trail[trail.length - 1]?.id === item.id) return;
      const now = Date.now();
      const last = lastFolderOpenRef.current;
      if (last.id === item.id && now - last.at < 500) return;
      lastFolderOpenRef.current = { id: item.id, at: now };
    }
    setSearchMode(false);
    setSearchModalOpen(false);
    if (fromSearch) {
      setTrail([{ id: item.id, name: item.name }]);
    } else {
      // Backstop: never append the same folder twice in a row (duplicate keys).
      setTrail((t) =>
        t[t.length - 1]?.id === item.id ? t : [...t, { id: item.id, name: item.name }],
      );
    }
    addRecentFolder(item.id, item.name);
    void loadFiles(item.id);
  }

  function openItem(item: FileItem) {
    if (item.isFolder) {
      openFolder(item);
      return;
    }
    if (isImagePreviewable(item) || isPdfPreviewable(item) || isVideoPreviewable(item)) {
      void openImageFile(item);
      return;
    }
    if (isTextPreviewable(item)) {
      void openTextFile(item);
      return;
    }
    window.open(fileDownloadUrl(item.id, item), "_blank", "noopener,noreferrer");
  }

  async function copyItemName(item: FileItem) {
    try {
      await navigator.clipboard.writeText(item.name);
      showToast(`Copied “${item.name}”`, false, "Clipboard");
    } catch {
      showToast("Could not copy name", true);
    }
  }

  function clearSearchDebounce() {
    if (searchDebounceRef.current) {
      clearTimeout(searchDebounceRef.current);
      searchDebounceRef.current = null;
    }
  }

  function abortSearch() {
    if (searchAbortRef.current) {
      searchAbortRef.current.abort();
      searchAbortRef.current = null;
    }
  }

  function exitSearchMode() {
    clearSearchDebounce();
    abortSearch();
    setSearchMode(false);
    setSearchModalOpen(false);
    setSearchResults([]);
    setSearchSuggest([]);
    setSearchNextToken(null);
    setSearchError(null);
    setSearchActiveQuery("");
    setSearchLoading(false);
  }

  async function runSearch(opts: {
    q: string;
    scope: SearchScope;
    pageToken?: string;
    suggest?: boolean;
    append?: boolean;
  }) {
    const q = opts.q.trim();
    if ([...q].length < 2) {
      setSearchSuggest([]);
      setSearchError(null);
      setSearchLoading(false);
      return;
    }
    abortSearch();
    const ac = new AbortController();
    searchAbortRef.current = ac;
    setSearchLoading(true);
    setSearchError(null);
    try {
      const res = await searchFiles({
        q,
        scope: opts.scope,
        folderId: opts.scope === "folder" ? folderId : undefined,
        pageToken: opts.pageToken,
        pageSize: opts.suggest ? 8 : 50,
        signal: ac.signal,
      });
      if (ac.signal.aborted) return;
      if (opts.suggest) {
        setSearchSuggest(res.items.slice(0, 8));
      } else if (opts.append) {
        setSearchResults((prev) => [...prev, ...res.items]);
        setSearchNextToken(res.nextPageToken);
      } else {
        setSearchResults(res.items);
        setSearchNextToken(res.nextPageToken);
        setSearchActiveQuery(res.query || q);
        setSearchMode(true);
        setSearchModalOpen(false);
        setView("files");
      }
    } catch (e) {
      if (e instanceof DOMException && e.name === "AbortError") return;
      if (e instanceof ApiError && e.status === 401) {
        setAuth({ status: "signed_out" });
        return;
      }
      const msg = e instanceof Error ? e.message : "Search failed";
      if (opts.suggest) {
        setSearchSuggest([]);
        setSearchError(msg);
      } else {
        setSearchError(msg);
        showToast(msg, true);
      }
    } finally {
      if (searchAbortRef.current === ac) {
        searchAbortRef.current = null;
        setSearchLoading(false);
      }
    }
  }

  function scheduleSuggest(q: string, scope: SearchScope) {
    clearSearchDebounce();
    const trimmed = q.trim();
    if ([...trimmed].length < 2) {
      abortSearch();
      setSearchSuggest([]);
      setSearchError(null);
      setSearchLoading(false);
      return;
    }
    searchDebounceRef.current = setTimeout(() => {
      void runSearch({ q: trimmed, scope, suggest: true });
    }, 250);
  }

  function submitFullSearch() {
    const q = searchQ.trim();
    if ([...q].length < 2) {
      showToast("Type at least 2 characters", true);
      return;
    }
    clearSearchDebounce();
    void runSearch({ q, scope: searchScope, suggest: false });
  }

  function openSearchHit(item: FileItem) {
    setSearchModalOpen(false);
    if (item.isFolder) {
      setSearchMode(false);
      setTrail([{ id: item.id, name: item.name }]);
      void loadFiles(item.id);
      return;
    }
    if (isImagePreviewable(item) || isVideoPreviewable(item) || isPdfPreviewable(item)) void openImageFile(item);
    else if (isTextPreviewable(item)) void openTextFile(item);
    else window.open(fileDownloadUrl(item.id, item), "_blank", "noopener,noreferrer");
  }

  async function doMkdir() {
    if (!mkdirName.trim()) {
      showToast("Folder name required", true);
      return;
    }
setMkdirBusy(true);
try {
  const parentId = folderId && folderId !== "root" ? folderId : undefined;
  const created = await createFolder(mkdirName.trim(), parentId);
  setMkdirOpen(false);
  const createdName = mkdirName;
  setMkdirName("");
  showToast(`Folder "${createdName}" created`);
  patchItems((list) => [created, ...list]);
  void loadFiles(folderId);
    } catch (e) {
      showToast(e instanceof Error ? e.message : "Create folder failed", true);
    } finally {
      setMkdirBusy(false);
    }
  }

  async function doTrash(item: FileItem) {
    const ok = await askConfirm({
      title: "Move to trash",
      message: `Move “${item.name}” to trash? You can restore it later in Google Drive.`,
      confirmLabel: "Move to trash",
      danger: true,
    });
    if (!ok) return;
    try {
  await trashFile(item.id);
  showToast(`Moved "${item.name}" to trash`);
  patchItems((list) => list.filter((it) => it.id !== item.id));
  void loadFiles(folderId);
    } catch (e) {
      showToast(e instanceof Error ? e.message : "Delete failed", true);
    }
  }

  function openRename(item: FileItem) {
    setRenameItem(item);
    setRenameName(item.name);
    setRenameOpen(true);
    setCtxMenu(null);
  }

  async function doRename() {
    if (!renameItem) return;
    const name = renameName.trim();
    if (!name) {
      showToast("Name required", true);
      return;
    }
    if (name === renameItem.name) {
      setRenameOpen(false);
      return;
    }
    setRenameBusy(true);
    try {
  await renameFile(renameItem.id, name);
  showToast(`Renamed to "${name}"`);
  setRenameOpen(false);
  setRenameItem(null);
  setRenameName("");
  patchItems((list) => list.map((it) => (it.id === renameItem.id ? { ...it, name } : it)));
  void loadFiles(folderId);
    } catch (e) {
      showToast(e instanceof Error ? e.message : "Rename failed", true);
    } finally {
      setRenameBusy(false);
    }
  }

  function openEditDesc(item: FileItem) {
    setDescItem(item);
    setDescValue(item.description ?? "");
    setDescOpen(true);
    setCtxMenu(null);
  }

  async function doSaveDesc() {
    if (!descItem) return;
    const next = descValue.trim();
    if (next === (descItem.description ?? "").trim()) {
      setDescOpen(false);
      return;
    }
    setDescBusy(true);
    try {
  await updateFile(descItem.id, { description: next });
  showToast(next ? "Description updated" : "Description cleared");
  setDescOpen(false);
  setDescItem(null);
  setDescValue("");
  patchItems((list) => list.map((it) => (it.id === descItem.id ? { ...it, description: next } : it)));
  void loadFiles(folderId);
    } catch (e) {
      showToast(e instanceof Error ? e.message : "Update failed", true);
    } finally {
      setDescBusy(false);
    }
  }

  function openMove(item: FileItem) {
    const current = folderId && folderId !== "root" ? folderId : "root";
    // Prefer parent of current folder (go up one), else root
    const defaultTarget =
      trail.length >= 2
        ? trail[trail.length - 2].id
        : trail.length === 1
          ? "root"
          : current === "root"
            ? "root"
            : "root";
    setMoveItem(item);
    setMoveTarget(defaultTarget);
    setMoveOpen(true);
    setCtxMenu(null);
  }

  async function doMove() {
    if (!moveItem) return;
    const parentId = moveTarget.trim() || "root";
    setMoveBusy(true);
    try {
await moveFile(moveItem.id, parentId, folderId);
showToast(`Moved "${moveItem.name}"`);
setMoveOpen(false);
setMoveItem(null);
patchItems((list) => list.filter((it) => it.id !== moveItem.id));
void loadFiles(folderId);
    } catch (e) {
      showToast(e instanceof Error ? e.message : "Move failed", true);
    } finally {
      setMoveBusy(false);
    }
  }

  function doCopy(item: FileItem) {
    setCopyItemState(item);
    setCopyTarget("root");
    setCopyOpen(true);
    setCtxMenu(null);
    setRowMenuId(null);
  }

  async function doCopyTo() {
    if (!copyItemState) return;
    const parentId = copyTarget.trim() || "root";
    setCopyBusy(true);
    try {
      await copyFile(copyItemState.id, parentId);
      showToast(`Copied "${copyItemState.name}"`);
      setCopyOpen(false);
      setCopyItemState(null);
      folderCacheRef.current.delete(parentId);
    } catch (e) {
      showToast(e instanceof Error ? e.message : "Copy failed", true);
    } finally {
      setCopyBusy(false);
    }
  }

  function doShare(item: FileItem) {
    setShareItem(item);
    setCtxMenu(null);
    setRowMenuId(null);
  }

  function doRevisions(item: FileItem) {
    setRevisionsItem(item);
    setCtxMenu(null);
    setRowMenuId(null);
  }

  function doZipDownload(item: FileItem) {
    const a = document.createElement("a");
    a.href = zipDownloadUrl(item.id);
    a.rel = "noopener noreferrer";
    a.download = `${item.name}.zip`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    showToast(`Downloading "${item.name}.zip"`);
  }

  function addRecentFolder(id: string, name: string) {
    if (!id || id === "root") return;
    setRecentFolders((prev) => {
      const filtered = prev.filter((f) => f.id !== id);
      return [{ id, name }, ...filtered].slice(0, 5);
    });
  }

  function openRecentFolder(id: string, name: string) {
    setTrail([{ id, name }]);
    void loadFiles(id);
    setNavOpen(false);
  }

  // Type filter: returns filtered items based on typeFilter
  const filteredItems = useMemo(() => {
    if (typeFilter === "all") return items;
    return items.filter((it) => {
      const kind = fileKind(it);
      if (typeFilter === "images") return kind === "image";
      if (typeFilter === "docs") return kind === "text" || kind === "doc" || kind === "sheet" || kind === "slides";
      if (typeFilter === "videos") return kind === "video";
      if (typeFilter === "archives") return kind === "archive";
      if (typeFilter === "folders") return it.isFolder;
      return true;
    });
  }, [items, typeFilter]);

  // Drag-to-move handlers
  function onRowDragStart(e: DragEvent<HTMLTableRowElement>, item: FileItem) {
    setDragItemId(item.id);
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", item.id);
  }

  function onRowDragEnd() {
    setDragItemId(null);
    setDragOverFolder(null);
  }

  function onFolderDrop(e: DragEvent<HTMLTableRowElement>, target: FileItem) {
    e.preventDefault();
    e.stopPropagation();
    setDragOverFolder(null);
    if (!dragItemId || !target.isFolder || dragItemId === target.id) return;
    moveFile(dragItemId, target.id, folderId)
      .then(() => { showToast(`Moved to "${target.name}"`); void loadFiles(folderId); })
      .catch((err) => showToast(err instanceof Error ? err.message : "Move failed", true));
    setDragItemId(null);
  }

  function toggleSelect(id: string, multi: boolean) {
    setSelectedIds((prev) => {
      const next = multi ? new Set(prev) : new Set<string>();
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
    setSelectedId(id);
  }

  function toggleSelectAllVisible(visible: FileItem[]) {
    const ids = visible.map((it) => it.id);
    setSelectedIds((prev) => {
      const allOn = ids.length > 0 && ids.every((id) => prev.has(id));
      if (allOn) return new Set();
      return new Set(ids);
    });
  }

  async function doBulkTrash() {
    const ids = [...selectedIds];
    if (ids.length === 0) return;
    const ok = await askConfirm({
      title: "Move to trash",
      message: `Move ${ids.length} item${ids.length === 1 ? "" : "s"} to trash? You can restore later in Google Drive.`,
      confirmLabel: "Move to trash",
      danger: true,
    });
    if (!ok) return;
    try {
      // L2: Use batch API instead of serial loop
const result = await batchTrash(ids);
if (result.succeeded) showToast(`Moved ${result.succeeded} item${result.succeeded === 1 ? "" : "s"} to trash`);
if (result.failed) showToast(`${result.failed} failed`, true);
if (!result.failed) {
  const gone = new Set(ids);
  patchItems((list) => list.filter((it) => !gone.has(it.id)));
}
} catch (e) {
showToast(e instanceof Error ? e.message : "Batch delete failed", true);
}
setSelectedIds(new Set());
void loadFiles(folderId);
  }

  function doBulkDownload(visible: FileItem[]) {
    const files = visible.filter((it) => selectedIds.has(it.id) && !it.isFolder);
    if (files.length === 0) {
      showToast("Select files to download (folders skipped)", true);
      return;
    }
    // Stagger downloads with 200ms interval to avoid browser blocking
    files.forEach((f, i) => {
      setTimeout(() => {
        const a = document.createElement("a");
        a.href = fileDownloadUrl(f.id, f);
        a.rel = "noopener noreferrer";
        a.download = f.name;
        document.body.appendChild(a);
        a.click();
        a.remove();
      }, i * 200);
    });
    showToast(`Downloading ${files.length} file${files.length === 1 ? "" : "s"}`);
  }

  async function doBulkZip(visible: FileItem[]) {
    const files = visible.filter((it) => selectedIds.has(it.id) && !it.isFolder);
    if (files.length === 0) {
      showToast("Select files to zip (folders skipped)", true);
      return;
    }
    showToast(`Creating ZIP with ${files.length} file${files.length === 1 ? "" : "s"}...`);
    try {
      await downloadMultiZip(files.map((f) => ({ id: f.id, name: f.name })));
      showToast(`Downloaded ${files.length} files as ZIP`);
    } catch (e) {
      showToast(e instanceof Error ? e.message : "ZIP download failed", true);
    }
  }

  const moveTargets = useMemo(() => {
    const opts: { id: string; label: string }[] = [{ id: "root", label: "My Drive (root)" }];
    for (const c of trail) {
      if (!opts.some((o) => o.id === c.id)) opts.push({ id: c.id, label: c.name });
    }
    if (folderId && folderId !== "root" && !opts.some((o) => o.id === folderId)) {
      opts.push({ id: folderId, label: "Current folder" });
    }
    // Sibling folders in the current listing (common drive UX)
    for (const it of items) {
      if (!it.isFolder) continue;
      if (moveItem && it.id === moveItem.id) continue;
      if (opts.some((o) => o.id === it.id)) continue;
      opts.push({ id: it.id, label: it.name });
    }
    return opts;
  }, [trail, folderId, items, moveItem]);

  // Text editor modal handlers
  const editorRef = useRef<HTMLTextAreaElement | null>(null);

  // Memoized lightbox item avoids items.find() on every App render while preview is open
  const lightboxItem = useMemo(() => {
    if (!imagePreview) return null;
    const found = items.find((it) => it.id === imagePreview.id);
    if (found) return found;
    return { id: imagePreview.id, name: imagePreview.name, mimeType: "", size: null, modifiedTime: "", isFolder: false } as FileItem;
  }, [imagePreview, items]);

  // Prev/next navigation cycles only within the same media kind as the open
  // file (images ↔ images, videos ↔ videos, PDFs ↔ PDFs) — same order user sees.
  const previewableList = useMemo(() => {
    if (!lightboxItem) return [];
    const kindOf = (it: FileItem) =>
      isImagePreviewable(it) ? "image" : isVideoPreviewable(it) ? "video" : isPdfPreviewable(it) ? "pdf" : null;
    const kind = kindOf(lightboxItem);
    if (!kind) return [];
    const source = searchMode ? searchResults : filteredItems;
    return sortItems(source).filter((it) => !it.isFolder && kindOf(it) === kind);
  }, [lightboxItem, searchMode, searchResults, filteredItems, sortKey, sortDir]);

  const lightboxIndex = useMemo(() => {
    if (!imagePreview) return -1;
    return previewableList.findIndex((it) => it.id === imagePreview.id);
  }, [imagePreview, previewableList]);

  const lightboxHasPrev = lightboxIndex > 0;
  const lightboxHasNext = lightboxIndex >= 0 && lightboxIndex < previewableList.length - 1;

  function lightboxGoPrev() {
    if (!lightboxHasPrev) return;
    void openImageFile(previewableList[lightboxIndex - 1]);
  }
  function lightboxGoNext() {
    if (!lightboxHasNext) return;
    void openImageFile(previewableList[lightboxIndex + 1]);
  }

  function closeImagePreview() {
    if (imageUrlRef.current) {
      URL.revokeObjectURL(imageUrlRef.current);
      imageUrlRef.current = null;
    }
    setImagePreview(null);
  }

  async function openImageFile(item: FileItem) {
    if (imageUrlRef.current) {
      URL.revokeObjectURL(imageUrlRef.current);
      imageUrlRef.current = null;
    }

    // Video & PDF: use streaming URL directly (server supports Range requests)
    // — no need to download the entire file into memory first.
    if (isVideoPreviewable(item) || isPdfPreviewable(item)) {
      setImagePreview({
        id: item.id,
        name: item.name,
        mimeType: item.mimeType,
        url: fileDownloadUrl(item.id, item),
        loading: false,
        error: null,
      });
      return;
    }

    // Images: show thumbnail instantly, then load full-res in background
    setImagePreview({
      id: item.id,
      name: item.name,
      url: fileThumbnailUrl(item.id, item.thumbnailUrl),
      loading: true,
      error: null,
    });
    try {
      const blob = await fetchFileBlob(item.id, item);
      const url = URL.createObjectURL(blob);
      imageUrlRef.current = url;
      setImagePreview({
        id: item.id,
        name: item.name,
        url,
        loading: false,
        error: null,
      });
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Failed to open image";
      showToast(msg, true);
      setImagePreview(null);
    }
  }

  async function openTextFile(item: FileItem) {
    setEditor({
      id: item.id,
      name: item.name,
      mimeType: item.mimeType,
      content: "",
      original: "",
      loading: true,
      saving: false,
      error: null,
    });
    try {
      const data = await fetchFileContent(item.id);
      setEditor({
        id: item.id,
        name: data.name || item.name,
        mimeType: data.mimeType,
        content: data.content,
        original: data.content,
        loading: false,
        saving: false,
        error: null,
      });
      // focus moved to effect
    } catch (e) {
      showToast(e instanceof Error ? e.message : "Failed to open file", true);
      setEditor(null);
    }
  }

  async function saveEditor() {
    if (!editor) return;
    if (editor.content === editor.original) return;
    // If this is a draft new file (id === null), open Save As dialog
    if (editor.id === null) {
      setSaveAsName("untitled.md");
      setSaveAsOpen(true);
      return;
    }
    setEditor({ ...editor, saving: true });
    try {
      await saveFileContent(editor.id, editor.content);
      setEditor({ ...editor, original: editor.content, saving: false, error: null });
      showToast(`Saved ${editor.name}`);
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Save failed";
      setEditor({ ...editor, saving: false, error: msg });
      showToast(msg, true);
    }
  }

  async function doCreateFromEditor() {
    if (!editor) return;
    const name = (saveAsName || "").trim() || "untitled.md";
    setSaveAsBusy(true);
    try {
      const parentId = folderId && folderId !== "root" ? folderId : undefined;
      const mime = mimeFromFilename(name) || "text/plain";
      const item = await createFile({ name, parentId, mimeType: mime, content: editor.content });
      // update editor to point to new file
      setEditor({ id: item.id, name: item.name, mimeType: item.mimeType, content: editor.content, original: editor.content, loading: false, saving: false, error: null });
      setSaveAsOpen(false);
      showToast(`Created ${item.name}`);
      void loadFiles(folderId);
    } catch (e) {
      showToast(e instanceof Error ? e.message : "Create file failed", true);
    } finally {
      setSaveAsBusy(false);
    }
  }

  async function closeEditor(force = false) {
    if (!editor) {
      setEditor(null);
      return;
    }
    if (!force && editor.content !== editor.original) {
      const ok = await askConfirm({
        title: "Discard changes?",
        message: "You have unsaved edits. Close without saving?",
        confirmLabel: "Discard",
        danger: true,
      });
      if (!ok) return;
    }
    setEditor(null);
  }

  const editorDirty = !!editor && editor.content !== editor.original;

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        if (imagePreview) {
          closeImagePreview();
          return;
        }
        if (confirm) {
          closeConfirm(false);
          return;
        }
        if (editor) void closeEditor();
      }
    }
    if (editor || imagePreview || confirm) window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [editor, imagePreview, confirm]);

  useEffect(() => {
    if (editor && !editor.loading) {
      // focus textarea
      setTimeout(() => {
        editorRef.current?.focus();
      }, 30);
    }
  }, [editor?.loading]);

  useEffect(() => {
    return () => {
      if (imageUrlRef.current) {
        URL.revokeObjectURL(imageUrlRef.current);
        imageUrlRef.current = null;
      }
      clearSearchDebounce();
      abortSearch();
    };
  }, []);

  // Ctrl/Cmd+K toggles the search modal (Spotlight-style)
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setSearchModalOpen((v) => !v);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  function goCrumb(index: number) {
    setSearchMode(false);
    setSearchModalOpen(false);
    if (index < 0) {
      setTrail([]);
      void loadFiles(undefined);
      return;
    }
    const next = trail.slice(0, index + 1);
    setTrail(next);
    void loadFiles(next[next.length - 1]?.id);
  }

  async function runUpload(file: File, parentId?: string) {
    const id = `job-${++uploadSeq}`;
    const ac = new AbortController();
    uploadAbortRef.current.set(id, ac);
    const draft: UploadJob = {
      id,
      name: file.name,
      received: 0,
      total: file.size,
      status: "starting",
    };
    setJobs((prev) => [draft, ...prev]);
    try {
      const job = await uploadFile(file, {
        parentId,
        signal: ac.signal,
        onCreated: (serverId) => patchJob(id, { serverId }),
        onProgress: (received, total, status) => {
          patchJob(id, { received, total, status });
        },
      });
      const cancelled = job.status === "cancelled" || ac.signal.aborted;
      const ok = job.status === "completed";
      patchJob(id, {
        received: job.bytesReceived ?? file.size,
        total: file.size,
        status: cancelled ? "cancelled" : ok ? "completed" : job.status,
        error: cancelled ? undefined : job.error ?? undefined,
        serverId: job.uploadId,
      });
      appendUploadHistory({
        name: file.name,
        status: cancelled ? "cancelled" : ok ? "completed" : job.status || "failed",
        at: new Date().toISOString(),
        error: cancelled ? undefined : job.error ?? undefined,
      });
      if (cancelled) {
        // Progress toast already shows cancelled state; auto-dismiss shortly.
        window.setTimeout(() => dismissUploadJob(id), 2800);
        return false;
      }
      if (ok) {
        window.setTimeout(() => dismissUploadJob(id), 3200);
      }
      // Errors stay until user dismisses the floating progress card.
      return ok;
    } catch (e) {
      const aborted =
        ac.signal.aborted ||
        (e instanceof ApiError && e.code === "upload_aborted");
      if (aborted) {
        patchJob(id, { status: "cancelled", cancelling: false, error: undefined });
        appendUploadHistory({
          name: file.name,
          status: "cancelled",
          at: new Date().toISOString(),
        });
        window.setTimeout(() => dismissUploadJob(id), 2800);
        return false;
      }
      const err = e instanceof Error ? e.message : "Upload failed";
      patchJob(id, {
        status: "failed",
        error: err,
        cancelling: false,
      });
      appendUploadHistory({ name: file.name, status: "failed", at: new Date().toISOString(), error: err });
      return false;
    } finally {
      uploadAbortRef.current.delete(id);
    }
  }

  async function cancelJob(job: UploadJob) {
    if (
      job.cancelling ||
      job.status === "completed" ||
      job.status === "done" ||
      job.status === "failed" ||
      job.status === "cancelled"
    ) {
      return;
    }
    patchJob(job.id, { cancelling: true, status: "cancelling" });
    uploadAbortRef.current.get(job.id)?.abort();
    if (job.serverId) {
      try {
        await cancelUpload(job.serverId);
      } catch {
        /* best-effort; abort still stops client loop */
      }
    }
  }

  function dismissUploadJob(id: string) {
    setJobs((prev) => prev.filter((j) => j.id !== id));
  }

  function onMainDragEnter(e: DragEvent<HTMLElement>) {
    e.preventDefault();
    e.stopPropagation();
    dragDepthRef.current += 1;
    setDropOn(true);
  }

  function onMainDragLeave(e: DragEvent<HTMLElement>) {
    e.preventDefault();
    e.stopPropagation();
    dragDepthRef.current = Math.max(0, dragDepthRef.current - 1);
    if (dragDepthRef.current === 0) setDropOn(false);
  }

  function onMainDragOver(e: DragEvent<HTMLElement>) {
    e.preventDefault();
    e.stopPropagation();
  }

  function onMainDrop(e: DragEvent<HTMLElement>) {
    e.preventDefault();
    e.stopPropagation();
    dragDepthRef.current = 0;
    setDropOn(false);
    void onPickFile(e.dataTransfer?.files ?? null);
  }

  async function onPickFile(fileList: FileList | null) {
    if (!fileList?.length || auth.status !== "signed_in") return;
    const files = Array.from(fileList);
    const parentId = folderId && folderId !== "root" ? folderId : undefined;
    setBusy(true);
    try {
      // Concurrent upload: max 4 parallel, rest queued (F1: 2→4)
      const CONCURRENCY = 4;
      let anyOk = false;
      let idx = 0;
      const runNext = async (): Promise<void> => {
        if (idx >= files.length) return;
        const file = files[idx++];
        // Small files: use simpleUpload (one request, no resumable init)
        if (file.size < SIMPLE_UPLOAD_THRESHOLD) {
          try {
            const item = await simpleUpload(file, parentId);
            // Incrementally insert at top of list — no full reload needed
            setItems((prev) => [item, ...prev]);
            folderCacheRef.current.delete(parentId ?? "root");
            anyOk = true;
            showToast(`Uploaded "${file.name}"`);
          } catch (e) {
            showToast(e instanceof Error ? e.message : `Upload failed: ${file.name}`, true);
          }
        } else {
          const ok = await runUpload(file, parentId);
          if (ok) anyOk = true;
        }
        return runNext();
      };
      const workers = Array.from({ length: Math.min(CONCURRENCY, files.length) }, () => runNext());
      await Promise.all(workers);
      // Only reload if large files were uploaded (small files already inserted incrementally)
      if (anyOk) {
        folderCacheRef.current.delete(parentId ?? "root");
        // Don't reload if only small files were uploaded — they're already in the list
        const hasLargeFiles = files.some((f) => f.size >= SIMPLE_UPLOAD_THRESHOLD);
        if (hasLargeFiles) await loadFiles(folderId);
      }
    } finally {
      setBusy(false);
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  }

  const sortedItems = useMemo(() => sortItems(filteredItems), [filteredItems, sortKey, sortDir]);
  const sortedSearchResults = useMemo(
    () => sortItems(searchResults),
    [searchResults, sortKey, sortDir],
  );

  // Stable, memo-friendly row callbacks. FileRow is wrapped in React.memo, so a
  // row only re-renders when its OWN props change — not on every upload-progress
  // or toast tick. To preserve that, the callbacks handed to each row must keep a
  // constant identity. We delegate through rowCbRef, whose .current is refreshed
  // each render (just before return) with the latest closures, so there is no
  // stale-closure risk while the handler identities stay stable across renders.
  const rowCbRef = useRef<{
    dragStart: (e: DragEvent<HTMLTableRowElement>, item: FileItem) => void;
    dragEnd: () => void;
    drop: (e: DragEvent<HTMLTableRowElement>, item: FileItem) => void;
    activate: (item: FileItem) => void;
    toggleSelect: (id: string) => void;
    dragOver: (item: FileItem) => void;
    dragLeave: () => void;
    more: (item: FileItem, rect: DOMRect) => void;
  } | null>(null);
  const rowHandlers = useMemo(
    () => ({
      onDragStart: (e: DragEvent<HTMLTableRowElement>, item: FileItem) =>
        rowCbRef.current?.dragStart(e, item),
      onDragEnd: () => rowCbRef.current?.dragEnd(),
      onDrop: (e: DragEvent<HTMLTableRowElement>, item: FileItem) =>
        rowCbRef.current?.drop(e, item),
      onActivate: (item: FileItem) => rowCbRef.current?.activate(item),
      onToggleSelect: (id: string) => rowCbRef.current?.toggleSelect(id),
      onDragOverFolder: (item: FileItem) => rowCbRef.current?.dragOver(item),
      onDragLeaveFolder: () => rowCbRef.current?.dragLeave(),
      onMore: (item: FileItem, rect: DOMRect) => rowCbRef.current?.more(item, rect),
    }),
    [],
  );

  // Enter opens selection; Delete/Backspace moves to trash (files view only).
  // Uses ref pattern: a single stable listener reads the latest state from a ref,
  // avoiding frequent addEventListener/removeEventListener churn on every selection
  // or sort change (the previous 17-dep effect re-bound on ~every interaction).
  const keyStateRef = useRef({
    view, searchMode, sortedItems, sortedSearchResults,
    selectedId, selectedIds, folderId,
    settingsOpen, mkdirOpen, renameOpen, descOpen, moveOpen, copyOpen,
    saveAsOpen, editor, imagePreview, confirm, ctxMenu, rowMenuId,
  });
  keyStateRef.current = {
    view, searchMode, sortedItems, sortedSearchResults,
    selectedId, selectedIds, folderId,
    settingsOpen, mkdirOpen, renameOpen, descOpen, moveOpen, copyOpen,
    saveAsOpen, editor, imagePreview, confirm, ctxMenu, rowMenuId,
  };

  // Also store action functions in a ref so the single-mount keydown handler
  // always calls the latest closure (avoids stale folderId/selectedIds bugs).
  const keyActionsRef = useRef({ openItem, doBulkTrash, doTrash, copyItemName, toggleSelectAllVisible });
  keyActionsRef.current = { openItem, doBulkTrash, doTrash, copyItemName, toggleSelectAllVisible };

  useEffect(() => {
    if (auth.status !== "signed_in") return;

    function isTypingTarget(t: EventTarget | null) {
      if (!(t instanceof HTMLElement)) return false;
      const tag = t.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return true;
      if (t.isContentEditable) return true;
      return !!t.closest("input, textarea, select, [contenteditable='true']");
    }

    function onKey(e: KeyboardEvent) {
      const s = keyStateRef.current;
      const modalBlocks = !!(s.settingsOpen || s.mkdirOpen || s.renameOpen || s.descOpen ||
        s.moveOpen || s.copyOpen || s.saveAsOpen || s.editor || s.imagePreview ||
        s.confirm || s.ctxMenu || s.rowMenuId);

      // Handle Ctrl/Cmd shortcuts even when typing
      if (e.key === "?" && !e.metaKey && !e.ctrlKey && !e.altKey && !isTypingTarget(e.target)) {
        e.preventDefault();
        setShortcutHelpOpen((v) => !v);
        return;
      }
      if ((e.metaKey || e.ctrlKey) && e.key === "a" && s.view === "files" && !modalBlocks) {
        e.preventDefault();
        keyActionsRef.current.toggleSelectAllVisible(s.searchMode ? s.sortedSearchResults : s.sortedItems);
        return;
      }
      if ((e.metaKey || e.ctrlKey) && e.key === "c" && s.view === "files" && !modalBlocks) {
        const id = s.selectedId || (s.selectedIds.size === 1 ? [...s.selectedIds][0] : null);
        if (!id) return;
        const list = s.searchMode ? s.sortedSearchResults : s.sortedItems;
        const item = list.find((it) => it.id === id);
        if (item) {
          e.preventDefault();
          void keyActionsRef.current.copyItemName(item);
        }
        return;
      }
      if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey) return;
      if (s.view !== "files") return;
      if (isTypingTarget(e.target)) return;
      if (modalBlocks) return;

      const list = s.searchMode ? s.sortedSearchResults : s.sortedItems;

      // Arrow key navigation
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        e.preventDefault();
        const dir = e.key === "ArrowDown" ? 1 : -1;
        const curIdx = s.selectedId ? list.findIndex((it) => it.id === s.selectedId) : -1;
        const nextIdx = curIdx < 0 ? 0 : Math.max(0, Math.min(list.length - 1, curIdx + dir));
        if (list[nextIdx]) {
          setSelectedId(list[nextIdx].id);
        }
        return;
      }

      if (e.key === "Enter") {
        const id =
          s.selectedId && list.some((it) => it.id === s.selectedId)
            ? s.selectedId
            : s.selectedIds.size === 1
              ? [...s.selectedIds][0]
              : null;
        if (!id) return;
        const item = list.find((it) => it.id === id);
        if (!item) return;
        e.preventDefault();
        keyActionsRef.current.openItem(item);
        return;
      }

      if (e.key === "Delete" || e.key === "Backspace") {
        if (s.selectedIds.size > 1) {
          e.preventDefault();
          void keyActionsRef.current.doBulkTrash();
          return;
        }
        const id =
          s.selectedIds.size === 1
            ? [...s.selectedIds][0]
            : s.selectedId && list.some((it) => it.id === s.selectedId)
              ? s.selectedId
              : null;
        if (!id) return;
        const item = list.find((it) => it.id === id);
        if (!item) return;
        e.preventDefault();
        void keyActionsRef.current.doTrash(item);
      }
    }

    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [auth.status]);

  // --- Memoized overview computations (MUST be before conditional returns to
  //     respect the Rules of Hooks — hooks cannot live after an early return). ---
  const typeCounts = useMemo(() => {
    const map = new Map<FileKind, number>();
    for (const it of items) {
      const k = fileKind(it);
      map.set(k, (map.get(k) ?? 0) + 1);
    }
    return Array.from(map.entries())
      .map(([kind, count]) => ({ kind, count, label: fileKindLabel(kind) }))
      .sort((a, b) => b.count - a.count);
  }, [items]);
  const histOk = useMemo(() => uploadHistory.filter((h) => h.status === "completed" || h.status === "done").length, [uploadHistory]);
  const histFail = useMemo(() => uploadHistory.filter((h) => h.status === "failed" || !!h.error).length, [uploadHistory]);

  if (auth.status === "loading") {
    return (
      <div className="boot" role="status" aria-live="polite" aria-label="加载中">
        <div>
          <div className="boot-dot" aria-hidden="true" />
          Loading…
        </div>
      </div>
    );
  }

  if (auth.status === "signed_out") {
    return (
      <section className="gate" aria-label="登录">
        <div className="gate-card">
          <div className="gate-mark" aria-hidden="true">
            <IconDrive size={20} />
          </div>
          <h1>Drive Backup Console</h1>
          <p>
            自建控制台：浏览个人 Google Drive、上传备份、查看进度。授权后会回到本页。
          </p>
          <a
            href="/oauth2/login"
            className="btn btn-primary"
            style={{ width: "100%", height: 40, display: "inline-flex", alignItems: "center", justifyContent: "center", textDecoration: "none" }}
          >
            Connect Google
          </a>
          <p className="gate-hint">使用 localhost:5174（勿用 127.0.0.1）</p>
        </div>
      </section>
    );
  }

  const email = auth.me.email;
  const avatar = (email?.[0] ?? "?").toUpperCase();

  const typeTotal = items.length || 1;
  const storageLimit = overview?.storage.limit ?? 0;
  const storageUsage = overview?.storage.usage ?? 0;
  const storagePct =
    storageLimit > 0 ? Math.min(100, Math.round((storageUsage / storageLimit) * 1000) / 10) : 0;

  // Refresh the row-callback closures every render (see rowCbRef above). These
  // capture current state/handlers, so rowHandlers always invoke the latest logic
  // while keeping a stable identity for React.memo.
  rowCbRef.current = {
    dragStart: onRowDragStart,
    dragEnd: onRowDragEnd,
    drop: onFolderDrop,
    activate: (item) => {
      if (item.isFolder) openFolder(item);
      else if (isImagePreviewable(item) || isPdfPreviewable(item) || isVideoPreviewable(item))
        void openImageFile(item);
      else if (isTextPreviewable(item)) void openTextFile(item);
    },
    toggleSelect: (id) => toggleSelect(id, true),
    dragOver: (item) => setDragOverFolder(item.id),
    dragLeave: () => setDragOverFolder(null),
    more: (item, rect) => setCtxMenu({ x: rect.right, y: rect.bottom, item }),
  };

  return (
    <div className={`shell is-on${navOpen ? " nav-open" : ""}${selectedIds.size > 0 ? " is-selecting" : ""}`} aria-label="文件控制台">
      <button
        type="button"
        className={`nav-scrim${navOpen ? " is-on" : ""}`}
        aria-label="Close menu"
        tabIndex={navOpen ? 0 : -1}
        onClick={() => setNavOpen(false)}
      />
      <Sidebar
        view={view}
        email={email || ""}
        avatar={avatar}
        busy={busy}
        recentFolders={recentFolders}
        onOpenFiles={() => {
          setView("files");
          exitSearchMode();
          goCrumb(-1);
          setNavOpen(false);
        }}
        onOpenOverview={() => {
          setView("overview");
          setNavOpen(false);
        }}
        onOpenRecent={openRecentFolder}
        onLogout={() => void onLogout()}
        isDark={themeMode === "dark"}
        connected
        refreshBusy={(searchMode ? searchLoading : listLoading) || busy}
        onToggleTheme={() => setThemeMode(themeMode === "dark" ? "light" : "dark")}
        onRefresh={() => {
          if (view === "overview") {
            void loadOverview();
          } else if (searchMode && searchActiveQuery) {
            void runSearch({ q: searchActiveQuery, scope: searchScope, suggest: false });
          } else {
            void loadFiles(folderId);
          }
          setNavOpen(false);
        }}
        onOpenSettings={() => {
          setSettingsOpen(true);
          setNavOpen(false);
        }}
        onOpenShortcuts={() => {
          setShortcutHelpOpen(true);
          setNavOpen(false);
        }}
      />

      <div className="workspace">
        <header className="topbar">
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-icon menu-btn"
            title="Menu"
            aria-label="Open menu"
            aria-expanded={navOpen}
            onClick={() => setNavOpen((v) => !v)}
          >
            <IconMenu size={18} />
          </button>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-icon"
            title="上级目录"
            aria-label="上级目录"
            disabled={trail.length === 0 || listLoading}
            onClick={() => {
              if (trail.length === 0) return;
              goCrumb(trail.length - 2);
            }}
          >
            <IconChevronLeft size={16} />
          </button>
          <nav className="crumbs" aria-label="路径">
            <button
              type="button"
              className={`crumb${trail.length === 0 && !searchMode ? " is-current" : ""}`}
              aria-current={trail.length === 0 && !searchMode ? "page" : undefined}
              onClick={() => goCrumb(-1)}
            >
              My Drive
            </button>
            {trail.map((c, i) => (
              <span key={c.id} style={{ display: "contents" }}>
                <span className="crumb-sep" aria-hidden="true">
                  /
                </span>
                <button
                  type="button"
                  className={`crumb${i === trail.length - 1 && !searchMode ? " is-current" : ""}`}
                  aria-current={i === trail.length - 1 && !searchMode ? "page" : undefined}
                  onClick={() => goCrumb(i)}
                >
                  {c.name}
                </button>
              </span>
            ))}
            {searchMode && (
              <>
                <span className="crumb-sep" aria-hidden="true">
                  /
                </span>
                <span className="crumb is-current" aria-current="page">
                  Search
                </span>
              </>
            )}
          </nav>
          <div className="search-box">
            <button
              type="button"
              className={`search-trigger${searchMode ? " is-active" : ""}`}
              onClick={() => setSearchModalOpen(true)}
              aria-label="Search files"
              aria-haspopup="dialog"
            >
              <IconSearch size={15} />
              <span className="search-trigger-label">
                {searchMode ? `“${searchActiveQuery}”` : "Search"}
              </span>
              <kbd className="search-trigger-kbd" aria-hidden="true">Ctrl K</kbd>
            </button>
            {searchMode && (
              <button
                type="button"
                className="search-clear"
                title="Clear search"
                aria-label="Clear search"
                onClick={() => {
                  setSearchQ("");
                  exitSearchMode();
                  void loadFiles(folderId);
                }}
              >
                ×
              </button>
            )}
          </div>
          <div className="top-meta">
            <span className="pill is-ok pill-status" title="Connected">
              <span className="dot" aria-hidden="true" />
              <span className="pill-label">Connected</span>
            </span>
            <button
              type="button"
              className="btn btn-ghost btn-sm btn-icon"
              title={themeMode === "dark" ? "Light mode" : themeMode === "light" ? "Dark mode" : "Toggle theme"}
              aria-label="Toggle theme"
              onClick={() => setThemeMode(themeMode === "dark" ? "light" : "dark")}
            >
              {themeMode === "dark" ? <IconSun size={16} /> : <IconMoon size={16} />}
            </button>
            <button
              type="button"
              className="btn btn-ghost btn-sm btn-icon"
              title="Keyboard shortcuts"
              aria-label="Keyboard shortcuts"
              onClick={() => setShortcutHelpOpen(true)}
            >
              <IconHelp size={16} />
            </button>
            <button
              type="button"
              className="btn btn-ghost btn-sm btn-icon"
              title="Settings"
              aria-label="Settings"
              aria-haspopup="dialog"
              aria-controls="settings-sheet"
              aria-expanded={settingsOpen}
              onClick={() => setSettingsOpen(true)}
            >
              <IconSettings size={16} />
            </button>
            <button
              type="button"
              className="btn btn-ghost btn-sm btn-icon"
              title="刷新"
              aria-label="刷新"
              disabled={(searchMode ? searchLoading : listLoading) || busy}
              onClick={() => {
                if (searchMode && searchActiveQuery) {
                  void runSearch({ q: searchActiveQuery, scope: searchScope, suggest: false });
                } else {
                  void loadFiles(folderId);
                }
              }}
            >
              <IconRefresh size={16} />
            </button>
          </div>
        </header>

        {view === "files" ? (
        <div className="toolbar" role="toolbar" aria-label="文件操作">
          {searchMode ? (
            <>
              <strong className="toolbar-title">
                Search · “{searchActiveQuery}” · {searchScope === "folder" ? "this folder" : "whole Drive"}
              </strong>
              <span className="num">{searchResults.length} result{searchResults.length === 1 ? "" : "s"}</span>
              <div className="spacer" />
              <button
                type="button"
                className="btn btn-ghost btn-sm"
                onClick={() => {
                  exitSearchMode();
                  void loadFiles(folderId);
                }}
              >
                Back to folder
              </button>
            </>
          ) : (
            <>
          <input
            ref={fileInputRef}
            type="file"
            id="file-upload"
            multiple
            hidden
            onChange={(e) => void onPickFile(e.target.files)}
          />
          <button
            type="button"
            className="btn btn-primary toolbar-action"
            disabled={busy}
            title="Upload"
            aria-label="Upload"
            onClick={() => fileInputRef.current?.click()}
          >
            <IconUpload size={16} />
            <span className="btn-label">Upload</span>
          </button>
          <button
            type="button"
            className="btn toolbar-action"
            disabled={busy}
            title="New Folder"
            aria-label="New Folder"
            onClick={() => setMkdirOpen(true)}
          >
            <IconFolderPlus size={15} />
            <span className="btn-label">New Folder</span>
          </button>
          <button
            type="button"
            className="btn toolbar-action"
            disabled={busy}
            title="New File"
            aria-label="New File"
            onClick={() => {
              // start a draft new file in editor
              setEditor({
                id: null,
                name: "Untitled",
                mimeType: "text/plain",
                content: "",
                original: "",
                loading: false,
                saving: false,
                error: null,
              });
            }}
          >
            <IconFilePlus size={15} />
            <span className="btn-label">New File</span>
          </button>
          <div className="type-filter-row" style={{ display: "inline-flex", height: 28, marginLeft: 4 }}>
            <button type="button" className={`type-chip${typeFilter === "all" ? " is-active" : ""}`} onClick={() => setTypeFilter("all")}>
              <IconFilter size={11} /> All
            </button>
            <button type="button" className={`type-chip${typeFilter === "images" ? " is-active" : ""}`} onClick={() => setTypeFilter("images")}>Images</button>
            <button type="button" className={`type-chip${typeFilter === "docs" ? " is-active" : ""}`} onClick={() => setTypeFilter("docs")}>Docs</button>
            <button type="button" className={`type-chip${typeFilter === "videos" ? " is-active" : ""}`} onClick={() => setTypeFilter("videos")}>Videos</button>
            <button type="button" className={`type-chip${typeFilter === "archives" ? " is-active" : ""}`} onClick={() => setTypeFilter("archives")}>Archives</button>
            <button type="button" className={`type-chip${typeFilter === "folders" ? " is-active" : ""}`} onClick={() => setTypeFilter("folders")}>Folders</button>
          </div>
          <div className="spacer" />
            </>
          )}
        </div>
        ) : (
        <div className="toolbar toolbar-overview" role="toolbar" aria-label="概览">
          <strong className="toolbar-title">Overview</strong>
          <div className="spacer" />
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            disabled={overviewLoading}
            onClick={() => void loadOverview()}
          >
            <IconRefresh size={15} />
            Refresh
          </button>
        </div>
        )}

        <div className="main-row rail-off">
          <main
            className="main panel"
            id="main-drop"
            tabIndex={-1}
            onDragEnter={view === "files" ? onMainDragEnter : undefined}
            onDragLeave={view === "files" ? onMainDragLeave : undefined}
            onDragOver={view === "files" ? onMainDragOver : undefined}
            onDrop={view === "files" ? onMainDrop : undefined}
          >
            {view === "overview" ? (
              <div className="overview-page">
                {overviewError && (
                  <div className="ov-banner ov-banner-error" role="alert">
                    <div>
                      <strong>Couldn&apos;t load Drive storage</strong>
                      <p>{overviewError}</p>
                    </div>
                    <button type="button" className="btn btn-sm" onClick={() => void loadOverview()}>
                      Retry
                    </button>
                  </div>
                )}

                <section className="ov-hero" aria-label="Storage">
                  <div className="ov-hero-top">
                    <div className="ov-hero-identity">
                      <div className="ov-hero-avatar" aria-hidden="true">
                        {(overview?.user?.displayName || overview?.user?.email || email || "?")
                          .slice(0, 1)
                          .toUpperCase()}
                      </div>
                      <div className="ov-hero-meta">
                        <h2>Google Drive storage</h2>
                        <p className="ov-muted">
                          {overviewLoading && !overview
                            ? "Loading quota…"
                            : overview?.user?.email || email || "Signed in"}
                          {overview?.user?.displayName ? ` · ${overview.user.displayName}` : ""}
                        </p>
                      </div>
                    </div>
                    <div className="ov-hero-kpis">
                      <div className="ov-kpi">
                        <span className="ov-kpi-label">Used</span>
                        <strong className="ov-kpi-value">
                          {overview ? formatBytes(storageUsage) : "—"}
                        </strong>
                      </div>
                      <div className="ov-kpi">
                        <span className="ov-kpi-label">Limit</span>
                        <strong className="ov-kpi-value">
                          {overview && storageLimit > 0 ? formatBytes(storageLimit) : "—"}
                        </strong>
                      </div>
                      <div className="ov-kpi">
                        <span className="ov-kpi-label">Free</span>
                        <strong className="ov-kpi-value">
                          {overview && storageLimit > 0
                            ? formatBytes(Math.max(0, storageLimit - storageUsage))
                            : "—"}
                        </strong>
                      </div>
                    </div>
                  </div>

                  <div className="ov-hero-bar-wrap">
                    <div
                      className={`ov-bar ov-bar-lg${storagePct >= 90 ? " is-critical" : storagePct >= 75 ? " is-warn" : ""}`}
                      role="progressbar"
                      aria-valuenow={storagePct}
                      aria-valuemin={0}
                      aria-valuemax={100}
                      aria-label="Storage used"
                    >
                      <i style={{ width: overview ? `${Math.max(storagePct, storagePct > 0 ? 1.5 : 0)}%` : "0%" }} />
                    </div>
                    <div className="ov-hero-bar-meta">
                      <span>
                        {overview ? `${storagePct}% full` : overviewLoading ? "…" : "No data"}
                      </span>
                      <span className="ov-muted">
                        Drive files {overview ? formatBytes(overview.storage.usageInDrive) : "—"}
                      </span>
                    </div>
                  </div>
                </section>

                <div className="overview-grid">
                  <section className="ov-card" aria-label="Uploads">
                    <div className="ov-card-head">
                      <div>
                        <h3>Uploads</h3>
                        <p className="ov-card-desc">Local console history (this browser)</p>
                      </div>
                      <span className="ov-pill">{uploadHistory.length} total</span>
                    </div>
                    <div className="ov-stat-row">
                      <div className="ov-stat is-ok">
                        <strong>{histOk}</strong>
                        <span>Success</span>
                      </div>
                      <div className="ov-stat is-bad">
                        <strong>{histFail}</strong>
                        <span>Failed</span>
                      </div>
                      <div className="ov-stat">
                        <strong>{jobs.length}</strong>
                        <span>Session</span>
                      </div>
                    </div>
                    <ul className="ov-hist">
                      {uploadHistory.slice(0, 10).length === 0 && (
                        <li className="ov-empty">
                          <span>No uploads yet</span>
                          <span className="ov-muted">Upload from Files to see history here</span>
                        </li>
                      )}
                      {uploadHistory.slice(0, 10).map((h, i) => (
                        <li key={`${h.at}-${i}`} className={h.status === "failed" || h.error ? "is-err" : "is-ok"}>
                          <span className="ov-hist-dot" aria-hidden="true" />
                          <span className="ov-hist-name" title={h.name}>{h.name}</span>
                          <span className="ov-hist-meta">{h.status}</span>
                        </li>
                      ))}
                    </ul>
                  </section>

                  <section className="ov-card" aria-label="File types">
                    <div className="ov-card-head">
                      <div>
                        <h3>Types in view</h3>
                        <p className="ov-card-desc">
                          {items.length} item{items.length === 1 ? "" : "s"} in current folder listing
                        </p>
                      </div>
                      <span className="ov-pill">{typeCounts.length} kinds</span>
                    </div>
                    {items.length === 0 ? (
                      <div className="ov-empty">
                        <span>No files loaded</span>
                        <span className="ov-muted">Open Files to browse a folder first</span>
                      </div>
                    ) : (
                      <ul className="ov-types">
                        {typeCounts.map((t) => {
                          const pct = Math.max(4, (t.count / typeTotal) * 100);
                          return (
                            <li key={t.kind} className="ov-type-row">
                              <span className={iconBoxClass(t.kind)} aria-hidden="true">
                                <KindIcon kind={t.kind} />
                              </span>
                              <div className="ov-type-copy">
                                <span className="ov-type-label">{t.label}</span>
                                <span className={`ov-type-bar kind-bar-${t.kind}`}>
                                  <i style={{ width: `${pct}%` }} />
                                </span>
                              </div>
                              <span className="ov-type-count">{t.count}</span>
                            </li>
                          );
                        })}
                      </ul>
                    )}
                  </section>

                  <section className="ov-card ov-card-tips" aria-label="Tips">
                    <div className="ov-card-head">
                      <div>
                        <h3>Quick tips</h3>
                        <p className="ov-card-desc">How this console works</p>
                      </div>
                    </div>
                    <ul className="ov-tips">
                      <li className="tip-desktop">Double-click a folder to open it; double-click text/code to edit.</li>
                      <li className="tip-mobile">Tap a folder to open it; tap text/code to edit. Long-press to select.</li>
                      <li className="tip-desktop">Drag files onto the Files view to upload into the current folder.</li>
                      <li className="tip-mobile">Tap the + button to upload files or create folders.</li>
                      <li>Type stats cover the current listing only — not the whole Drive.</li>
                      <li>Use localhost:5174 so the session cookie matches the API host.</li>
                    </ul>
                  </section>
                </div>
              </div>
            ) : (
            <>
            <div
              className={`drop-overlay${dropOn ? " is-on" : ""}`}
              id="drop-overlay"
              aria-hidden={!dropOn}
            >
              Drop files to upload
            </div>
            {(searchMode ? searchError : listError) && (
              <p className="list-error">{searchMode ? searchError : listError}</p>
            )}
            <div className="table-wrap">
              {(() => {
                const tableItems = searchMode ? sortedSearchResults : sortedItems;
                const tableLoading = searchMode ? searchLoading : listLoading;
                const tableError = searchMode ? searchError : listError;
                const sortAria = (key: SortKey) =>
                  sortKey === key ? (sortDir === "asc" ? "ascending" : "descending") : "none";
                const SortIcon = ({ col }: { col: SortKey }) =>
                  sortKey !== col ? null : sortDir === "asc" ? (
                    <IconChevronUp size={12} />
                  ) : (
                    <IconChevronDown size={12} />
                  );
                return (
              <div style={{ position: "relative" }}>
              {selectedIds.size > 0 && (
                <div className="selection-bar" role="status">
                  <span className="selection-count">{selectedIds.size} selected</span>
<button type="button" className="btn btn-ghost btn-sm" onClick={() => doBulkDownload(tableItems)}>
<IconDownload size={14} /> Download
</button>
<button type="button" className="btn btn-ghost btn-sm" onClick={() => void doBulkZip(tableItems)}>
<IconDownload size={14} /> ZIP
</button>
                  <button type="button" className="btn btn-ghost btn-sm btn-danger" onClick={() => void doBulkTrash()}>
                    <IconTrash size={14} /> Trash
                  </button>
                  <button type="button" className="btn btn-ghost btn-sm" onClick={() => setSelectedIds(new Set())}>
                    Clear
                  </button>
                </div>
              )}
              {(tableLoading || tableItems.length > 0 || tableError) && (
                <table className="file-table">
                  <thead>
                    <tr>
                      <th className="col-check">
                        <input
                          type="checkbox"
                          className="row-check"
                          aria-label="Select all"
                          checked={tableItems.length > 0 && tableItems.every((it) => selectedIds.has(it.id))}
                          onChange={() => toggleSelectAllVisible(tableItems)}
                        />
                      </th>
                      <th aria-sort={sortAria("name")}>
                        <button type="button" className="th-sort" onClick={() => toggleSort("name")}>
                          Name <SortIcon col="name" />
                        </button>
                      </th>
                      <th aria-sort={sortAria("size")}>
                        <button type="button" className="th-sort" onClick={() => toggleSort("size")}>
                          Size <SortIcon col="size" />
                        </button>
                      </th>
                      <th aria-sort={sortAria("modified")}>
                        <button type="button" className="th-sort" onClick={() => toggleSort("modified")}>
                          Modified <SortIcon col="modified" />
                        </button>
                      </th>
                      <th />
                    </tr>
                  </thead>
                  <tbody
                    onContextMenu={(e) => {
                      e.preventDefault();
                      const target = e.target as HTMLElement;
                      const tr = target.closest("tr");
                      const id = tr?.getAttribute("data-id");
                      const item = id ? tableItems.find((it) => it.id === id) ?? null : null;
                      setCtxMenu({ x: e.clientX, y: e.clientY, item });
                    }}
                  >
                    {tableLoading && tableItems.length === 0 && (
                      Array.from({ length: 5 }).map((_, i) => (
                        <tr key={`skel-${i}`} className="skel-row-tr">
                          <td colSpan={5} style={{ padding: 0 }}>
                            <div className="skel-row">
                              <span className="skel-bar skel-icon" style={{ width: 24, height: 24 }} />
                              <span className="skel-bar skel-name" style={{ width: `${60 + Math.random() * 30}%` }} />
                              <span className="skel-bar skel-size" />
                              <span className="skel-bar skel-date" />
                            </div>
                          </td>
                        </tr>
                      ))
                    )}
                    {tableItems.slice(0, renderLimit).map((item) => (
                      <FileRow
                        key={item.id}
                        item={item}
                        isActive={selectedId === item.id}
                        isMulti={selectedIds.has(item.id)}
                        isDragging={dragItemId === item.id}
                        isDropTarget={dragOverFolder === item.id}
                        dragActive={dragItemId !== null}
                        onDragStart={rowHandlers.onDragStart}
                        onDragEnd={rowHandlers.onDragEnd}
                        onDrop={rowHandlers.onDrop}
                        onDragOverFolder={rowHandlers.onDragOverFolder}
                        onDragLeaveFolder={rowHandlers.onDragLeaveFolder}
                        onActivate={rowHandlers.onActivate}
                        onToggleSelect={rowHandlers.onToggleSelect}
                        onMore={rowHandlers.onMore}
                      />
                    ))}
                    {tableItems.length > renderLimit && (
                      <tr>
                        <td colSpan={5} style={{ textAlign: "center", padding: "12px" }}>
                          <button
                            type="button"
                            className="btn btn-ghost btn-sm"
                            onClick={() => setRenderLimit((prev) => prev + 200)}
                          >
                            Show more ({tableItems.length - renderLimit} remaining)
                          </button>
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              )}
              {searchMode && searchNextToken && !tableLoading && (
                <div className="search-load-more">
                  <button
                    type="button"
                    className="btn"
                    disabled={searchLoading}
                    onClick={() =>
                      void runSearch({
                        q: searchActiveQuery,
                        scope: searchScope,
                        pageToken: searchNextToken,
                        append: true,
                      })
                    }
                  >
                    Load more
                  </button>
                </div>
              )}
              {!searchMode && listNextToken && !tableLoading && (
                <div className="search-load-more">
                  <button
                    type="button"
                    className="btn"
                    disabled={listLoadingMore}
                    onClick={() => void loadMoreFiles()}
                  >
                    {listLoadingMore ? "Loading…" : "Load more"}
                  </button>
                </div>
              )}
              <div
                className={`empty${!tableLoading && tableItems.length === 0 && !tableError ? " is-on" : ""}`}
                id="empty-state"
              >
                {searchMode ? (
                  <>
                    <strong>No matches</strong>
                    <span>Try another name or switch scope (Folder / Drive).</span>
                    <button
                      type="button"
                      className="btn"
                      style={{ marginTop: 12 }}
                      onClick={() => {
                        exitSearchMode();
                        void loadFiles(folderId);
                      }}
                    >
                      Back to folder
                    </button>
                  </>
                ) : (
                  <>
                <strong>This folder is empty</strong>
                <span>Upload a backup or create a folder.</span>
                <button
                  type="button"
                  className="btn btn-primary"
                  style={{ marginTop: 12 }}
                  disabled={busy}
                  onClick={() => fileInputRef.current?.click()}
                >
                  Upload
                </button>
                  </>
                )}
              </div>
              </div>
                );
              })()}
            </div>
            </>
            )}
          </main>

        </div>

        <footer className="footer" role="status">
          {view === "overview"
            ? overviewLoading
              ? "Loading overview…"
              : overviewError
                ? "Overview error"
                : "Overview"
            : listLoading
              ? "Loading…"
              : selectedIds.size > 0
                ? `${selectedIds.size} selected · ${formatBytes(sortedItems.filter((it) => selectedIds.has(it.id) && !it.isFolder).reduce((sum, it) => sum + (it.size ?? 0), 0))}`
                : `${items.length} item${items.length === 1 ? "" : "s"}`}
        </footer>
      </div>

      <MobileNav
        view={view}
        busy={busy}
        onOpenFiles={() => {
          setView("files");
          exitSearchMode();
          goCrumb(-1);
        }}
        onOpenOverview={() => setView("overview")}
        onUpload={() => fileInputRef.current?.click()}
        onNewFolder={() => setMkdirOpen(true)}
        onNewFile={() =>
          setEditor({
            id: null,
            name: "Untitled",
            mimeType: "text/plain",
            content: "",
            original: "",
            loading: false,
            saving: false,
            error: null,
          })
        }
      />

      <ConfirmDialog
        open={!!confirm}
        title={confirm?.title || ""}
        message={confirm?.message || ""}
        confirmLabel={confirm?.confirmLabel}
        danger={confirm?.danger}
        onCancel={() => closeConfirm(false)}
        onConfirm={() => closeConfirm(true)}
      />

      {/* Mkdir modal */}
      <div
        className={`mkdir-overlay${mkdirOpen ? " is-on" : ""}`}
        hidden={!mkdirOpen}
        onClick={() => setMkdirOpen(false)}
      />
      <div
        className={`mkdir-sheet${mkdirOpen ? " is-on" : ""}`}
        role="dialog"
        aria-modal="true"
        hidden={!mkdirOpen}
      >
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
          <h3 style={{ margin: 0 }}>Create folder</h3>
          <button type="button" className="btn btn-ghost btn-sm btn-icon" aria-label="Close" onClick={() => setMkdirOpen(false)}>
            <IconClose size={16} />
          </button>
        </div>
        <div style={{ display: "flex", gap: 8 }}>
          <input
            autoFocus
            value={mkdirName}
            onChange={(e) => setMkdirName(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") void doMkdir(); if (e.key === "Escape") setMkdirOpen(false); }}
            placeholder="Folder name"
            style={{ flex: 1, padding: "8px 10px", borderRadius: 6, border: "1px solid var(--border)" }}
          />
          <button type="button" className="btn btn-ghost" onClick={() => setMkdirOpen(false)}>Cancel</button>
          <button type="button" className="btn btn-primary" disabled={mkdirBusy} onClick={() => void doMkdir()}>Create</button>
        </div>
      </div>

      {/* Rename modal */}
      <div
        className={`mkdir-overlay${renameOpen ? " is-on" : ""}`}
        hidden={!renameOpen}
        onClick={() => !renameBusy && setRenameOpen(false)}
      />
      <div
        className={`mkdir-sheet${renameOpen ? " is-on" : ""}`}
        role="dialog"
        aria-modal="true"
        hidden={!renameOpen}
      >
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
          <h3 style={{ margin: 0 }}>Rename</h3>
          <button type="button" className="btn btn-ghost btn-sm btn-icon" aria-label="Close" disabled={renameBusy} onClick={() => setRenameOpen(false)}>
            <IconClose size={16} />
          </button>
        </div>
        <div style={{ display: "flex", gap: 8 }}>
          <input
            autoFocus
            value={renameName}
            onChange={(e) => setRenameName(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") void doRename(); if (e.key === "Escape" && !renameBusy) setRenameOpen(false); }}
            placeholder="New name"
            disabled={renameBusy}
            style={{ flex: 1, padding: "8px 10px", borderRadius: 6, border: "1px solid var(--border)" }}
          />
          <button type="button" className="btn btn-ghost" disabled={renameBusy} onClick={() => setRenameOpen(false)}>Cancel</button>
          <button type="button" className="btn btn-primary" disabled={renameBusy || !renameName.trim()} onClick={() => void doRename()}>
            {renameBusy ? "Saving…" : "Save"}
          </button>
        </div>
      </div>

      {/* Description modal (folders) */}
      <div
        className={`mkdir-overlay${descOpen ? " is-on" : ""}`}
        hidden={!descOpen}
        onClick={() => !descBusy && setDescOpen(false)}
      />
      <div
        className={`mkdir-sheet${descOpen ? " is-on" : ""}`}
        role="dialog"
        aria-modal="true"
        hidden={!descOpen}
      >
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
          <h3 style={{ margin: 0 }}>Description{descItem ? ` “${descItem.name}”` : ""}</h3>
          <button type="button" className="btn btn-ghost btn-sm btn-icon" aria-label="Close" disabled={descBusy} onClick={() => setDescOpen(false)}>
            <IconClose size={16} />
          </button>
        </div>
        <textarea
          autoFocus
          value={descValue}
          onChange={(e) => setDescValue(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Escape" && !descBusy) setDescOpen(false); }}
          placeholder="What is this folder for? (shown to AI agents when listing)"
          disabled={descBusy}
          rows={3}
          style={{ width: "100%", padding: "8px 10px", borderRadius: 6, border: "1px solid var(--border)", resize: "vertical", boxSizing: "border-box", fontFamily: "inherit", fontSize: 14 }}
        />
        <div style={{ display: "flex", justifyContent: "flex-end", gap: 8, marginTop: 8 }}>
          <button type="button" className="btn btn-ghost" disabled={descBusy} onClick={() => setDescOpen(false)}>Cancel</button>
          <button type="button" className="btn btn-primary" disabled={descBusy} onClick={() => void doSaveDesc()}>
            {descBusy ? "Saving…" : "Save"}
          </button>
        </div>
      </div>

      {/* Move modal */}
      <div
        className={`mkdir-overlay${moveOpen ? " is-on" : ""}`}
        hidden={!moveOpen}
        onClick={() => !moveBusy && setMoveOpen(false)}
      />
      <div
        className={`mkdir-sheet${moveOpen ? " is-on" : ""}`}
        role="dialog"
        aria-modal="true"
        hidden={!moveOpen}
      >
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
          <h3 style={{ margin: 0 }}>Move{moveItem ? ` “${moveItem.name}”` : ""}</h3>
          <button type="button" className="btn btn-ghost btn-sm btn-icon" aria-label="Close" disabled={moveBusy} onClick={() => setMoveOpen(false)}>
            <IconClose size={16} />
          </button>
        </div>
        <label style={{ display: "block", fontSize: 12, color: "var(--muted)", marginBottom: 6 }}>Destination</label>
        <select
          value={moveTarget}
          onChange={(e) => setMoveTarget(e.target.value)}
          disabled={moveBusy}
          style={{ width: "100%", padding: "8px 10px", borderRadius: 6, border: "1px solid var(--border)", marginBottom: 12, background: "var(--surface)", color: "var(--text)" }}
        >
          {moveTargets.map((t) => (
            <option key={t.id} value={t.id}>{t.label}</option>
          ))}
        </select>
        <div style={{ display: "flex", justifyContent: "flex-end", gap: 8 }}>
          <button type="button" className="btn btn-ghost" disabled={moveBusy} onClick={() => setMoveOpen(false)}>Cancel</button>
          <button type="button" className="btn btn-primary" disabled={moveBusy} onClick={() => void doMove()}>
            {moveBusy ? "Moving…" : "Move"}
          </button>
        </div>
      </div>

      {/* Copy-to dialog */}
      <div
        className={`mkdir-overlay${copyOpen ? " is-on" : ""}`}
        hidden={!copyOpen}
        onClick={() => !copyBusy && setCopyOpen(false)}
      />
      <div
        className={`mkdir-sheet${copyOpen ? " is-on" : ""}`}
        role="dialog"
        aria-modal="true"
        hidden={!copyOpen}
      >
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
          <h3 style={{ margin: 0 }}>Copy{copyItemState ? ` “${copyItemState.name}”` : ""}</h3>
          <button type="button" className="btn btn-ghost btn-sm btn-icon" aria-label="Close" disabled={copyBusy} onClick={() => setCopyOpen(false)}>
            <IconClose size={16} />
          </button>
        </div>
        <label style={{ display: "block", fontSize: 12, color: "var(--muted)", marginBottom: 6 }}>Destination folder</label>
        <select
          value={copyTarget}
          onChange={(e) => setCopyTarget(e.target.value)}
          disabled={copyBusy}
          style={{ width: "100%", padding: "8px 10px", borderRadius: 6, border: "1px solid var(--border)", marginBottom: 12, background: "var(--surface)", color: "var(--text)" }}
        >
          {moveTargets.map((t) => (
            <option key={t.id} value={t.id}>{t.label}</option>
          ))}
        </select>
        <div style={{ display: "flex", justifyContent: "flex-end", gap: 8 }}>
          <button type="button" className="btn btn-ghost" disabled={copyBusy} onClick={() => setCopyOpen(false)}>Cancel</button>
          <button type="button" className="btn btn-primary" disabled={copyBusy} onClick={() => void doCopyTo()}>
            {copyBusy ? "Copying…" : "Copy here"}
          </button>
        </div>
      </div>

      {/* Save As modal */}
      <div
        className={`saveas-overlay${saveAsOpen ? " is-on" : ""}`}
        hidden={!saveAsOpen}
        onClick={() => setSaveAsOpen(false)}
      />
      <div
        className={`saveas-sheet${saveAsOpen ? " is-on" : ""}`}
        role="dialog"
        aria-modal="true"
        hidden={!saveAsOpen}
      >
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
          <h3 style={{ margin: 0 }}>Save file</h3>
          <button type="button" className="btn btn-ghost btn-sm btn-icon" aria-label="Close" onClick={() => setSaveAsOpen(false)}>
            <IconClose size={16} />
          </button>
        </div>
        <div style={{ display: "flex", gap: 8 }}>
          <input
            autoFocus
            value={saveAsName}
            onChange={(e) => setSaveAsName(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") void doCreateFromEditor(); if (e.key === "Escape") setSaveAsOpen(false); }}
            placeholder="Filename (e.g. notes.md)"
            style={{ flex: 1, padding: "8px 10px", borderRadius: 6, border: "1px solid var(--border)" }}
          />
          <button type="button" className="btn btn-ghost" onClick={() => setSaveAsOpen(false)}>Cancel</button>
          <button type="button" className="btn btn-primary" disabled={saveAsBusy} onClick={() => void doCreateFromEditor()}>Create</button>
        </div>
      </div>

      {/* Editor modal — only mount when open so display:flex cannot leave a stuck empty sheet */}
      {editor && (
        <>
          <div className="editor-overlay is-on" onClick={() => void closeEditor()} />
          <div
            className="editor-sheet is-on"
            role="dialog"
            aria-modal="true"
            aria-label={editor.name || "Text editor"}
            onClick={(e) => e.stopPropagation()}
          >
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
              <h3 style={{ margin: 0 }}>{editor.name || "Untitled"}</h3>
              <button type="button" className="btn btn-ghost btn-sm btn-icon" aria-label="Close" onClick={() => void closeEditor()}>
                <IconClose size={16} />
              </button>
            </div>
            <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
              {editor.loading ? (
                <div>Loading…</div>
              ) : editor.error ? (
                <div style={{ color: "var(--danger)" }}>{editor.error}</div>
              ) : (
                <textarea
                  ref={editorRef}
                  className="editor-textarea"
                  value={editor.content}
                  onChange={(e) => setEditor({ ...editor, content: e.target.value })}
                  spellCheck={false}
                />
              )}
              <div style={{ display: "flex", justifyContent: "flex-end", gap: 8 }}>
                <button type="button" className="btn" onClick={() => void closeEditor()}>Cancel</button>
                <button type="button" className="btn btn-primary" disabled={!editorDirty || editor.saving || editor.loading} onClick={() => void saveEditor()}>
                  {editor.saving ? "Saving…" : "Save"}
                </button>
              </div>
            </div>
          </div>
        </>
      )}

      {searchModalOpen && (
        <>
          <div className="search-modal-overlay" onClick={() => setSearchModalOpen(false)} />
          <div className="search-modal" role="dialog" aria-modal="true" aria-label="Search files">
            <div className="search-modal-head">
              <IconSearch size={18} />
              <input
                type="search"
                className="search-modal-input"
                placeholder={searchScope === "folder" ? "Search this folder…" : "Search Drive…"}
                value={searchQ}
                autoFocus
                aria-label="Search files"
                onChange={(e) => {
                  const v = e.target.value;
                  setSearchQ(v);
                  scheduleSuggest(v, searchScope);
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    submitFullSearch();
                  } else if (e.key === "Escape") {
                    setSearchModalOpen(false);
                  }
                }}
              />
              {searchLoading && <span className="search-spinner" aria-hidden="true" />}
              {searchQ && (
                <button
                  type="button"
                  className="search-clear"
                  title="Clear"
                  aria-label="Clear"
                  onClick={() => {
                    setSearchQ("");
                    setSearchSuggest([]);
                    setSearchError(null);
                  }}
                >
                  ×
                </button>
              )}
              <button
                type="button"
                className="search-modal-cancel"
                onClick={() => setSearchModalOpen(false)}
              >
                Cancel
              </button>
            </div>
            <div className="search-modal-scope" role="group" aria-label="Search scope">
              <button
                type="button"
                className={searchScope === "drive" ? "is-active" : ""}
                onClick={() => {
                  setSearchScope("drive");
                  if (searchQ.trim().length >= 2) scheduleSuggest(searchQ, "drive");
                }}
              >
                Whole Drive
              </button>
              <button
                type="button"
                className={searchScope === "folder" ? "is-active" : ""}
                onClick={() => {
                  setSearchScope("folder");
                  if (searchQ.trim().length >= 2) scheduleSuggest(searchQ, "folder");
                }}
              >
                This folder
              </button>
            </div>
            <div className="search-modal-results" role="listbox" aria-label="Search results">
              {searchQ.trim().length < 2 && !searchError && (
                <div className="search-empty">Type at least 2 characters to search…</div>
              )}
              {searchError && <div className="search-empty">{searchError}</div>}
              {!searchError && searchLoading && searchSuggest.length === 0 && searchQ.trim().length >= 2 && (
                <div className="search-empty">Searching…</div>
              )}
              {!searchError && !searchLoading && searchSuggest.length === 0 && searchQ.trim().length >= 2 && (
                <div className="search-empty">No matches</div>
              )}
              {searchSuggest.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  className="search-hit"
                  role="option"
                  onClick={() => openSearchHit(item)}
                >
                  <span className={iconBoxClass(fileKind(item))} aria-hidden="true">
                    <FileTypeIcon item={item} />
                  </span>
                  <span className="search-hit-meta">
                    <span className="search-hit-name">{item.name}</span>
                    <span className="search-hit-kind">
                      {fileKindLabel(fileKind(item))}
                      {item.size != null && !item.isFolder ? ` · ${formatBytes(item.size)}` : ""}
                    </span>
                  </span>
                </button>
              ))}
              {searchQ.trim().length >= 2 && searchSuggest.length > 0 && (
                <button type="button" className="search-more" onClick={() => submitFullSearch()}>
                  View all results for “{searchQ.trim()}”
                </button>
              )}
            </div>
          </div>
        </>
      )}

      {imagePreview && (
        <Suspense fallback={null}>
          <MediaLightbox
            preview={imagePreview}
            item={lightboxItem}
            onClose={closeImagePreview}
            onPrev={lightboxGoPrev}
            onNext={lightboxGoNext}
            hasPrev={lightboxHasPrev}
            hasNext={lightboxHasNext}
          />
        </Suspense>
      )}

      {settingsOpen && (
        <Suspense fallback={null}>
          <SettingsSheet
            open={settingsOpen}
            prefs={prefs}
            email={email || ""}
            busy={busy}
            themeMode={themeMode}
            wallpaper={wallpaper}
            onClose={() => setSettingsOpen(false)}
            onPrefsChange={setPrefs}
            onThemeChange={setThemeMode}
            onWallpaperChange={setWallpaper}
            onLogout={() => void onLogout()}
            onBannerEnabled={() => {
              setTimeout(() => showToast("Banner alerts on", false, "Settings"), 0);
            }}
          />
        </Suspense>
      )}

      {ctxMenu && (
        <div
          className="ctx-menu"
          role="menu"
          style={{
            // Keep the menu on-screen: clamp its left so the right edge can't
            // overflow the viewport (the ⋯ trigger sits near the right edge,
            // which pushed the 188px menu off-screen on narrow phones).
            left: Math.max(8, Math.min(ctxMenu.x, window.innerWidth - 200)),
            top: ctxMenu.y,
          }}
          onContextMenu={(e) => e.preventDefault()}
        >
          {/* Empty space: only New folder / Upload / Refresh */}
          {!ctxMenu.item && (
            <>
              <button type="button" role="menuitem" onClick={(e) => { e.stopPropagation(); setCtxMenu(null); setMkdirOpen(true); }}>
                <IconFolderPlus size={15} /> New folder
              </button>
              <button type="button" role="menuitem" onClick={(e) => { e.stopPropagation(); setCtxMenu(null); fileInputRef.current?.click(); }}>
                <IconUpload size={15} /> Upload files
              </button>
              <div className="ctx-separator" />
              <button type="button" role="menuitem" onClick={(e) => { e.stopPropagation(); setCtxMenu(null); void loadFiles(folderId); }}>
                <IconRefresh size={15} /> Refresh
              </button>
            </>
          )}

          {/* File or folder: full menu */}
          {ctxMenu.item && (
            <>
              {/* Open */}
              <button type="button" role="menuitem" onClick={(e) => { e.stopPropagation(); setCtxMenu(null); openItem(ctxMenu.item!); }}>
                <IconOpen size={15} /> Open
              </button>
              <button type="button" role="menuitem" onClick={(e) => { e.stopPropagation(); setCtxMenu(null); toggleSelect(ctxMenu.item!.id, true); }}>
                <IconCheck size={15} /> {selectedIds.has(ctxMenu.item.id) ? "Deselect" : "Select"}
              </button>

              <div className="ctx-separator" />

              {/* Download / Share */}
              {ctxMenu.item.isFolder ? (
                <button type="button" role="menuitem" onClick={(e) => { e.stopPropagation(); setCtxMenu(null); doZipDownload(ctxMenu.item!); }}>
                  <IconDownload size={15} /> Download as ZIP
                </button>
              ) : (
                <a role="menuitem" href={fileDownloadUrl(ctxMenu.item.id, ctxMenu.item)} onClick={(e) => { e.stopPropagation(); setCtxMenu(null); }}>
                  <IconDownload size={15} /> Download
                </a>
              )}
              <button type="button" role="menuitem" onClick={(e) => { e.stopPropagation(); doShare(ctxMenu.item!); }}>
                <IconShare size={15} /> Share
              </button>

              <div className="ctx-separator" />

              {/* Edit: Rename / Move / Duplicate */}
              <button type="button" role="menuitem" onClick={(e) => { e.stopPropagation(); setCtxMenu(null); openRename(ctxMenu.item!); }}>
                <IconRename size={15} /> Rename
              </button>
              {ctxMenu.item.isFolder && (
              <button type="button" role="menuitem" onClick={(e) => { e.stopPropagation(); setCtxMenu(null); openEditDesc(ctxMenu.item!); }}>
                <IconRename size={15} /> {ctxMenu.item.description ? "Edit description" : "Add description"}
              </button>
              )}
              <button type="button" role="menuitem" onClick={(e) => { e.stopPropagation(); setCtxMenu(null); openMove(ctxMenu.item!); }}>
                <IconMove size={15} /> Move to…
              </button>
              {!ctxMenu.item.isFolder && (
              <button type="button" role="menuitem" onClick={(e) => { e.stopPropagation(); setCtxMenu(null); doCopy(ctxMenu.item!); }}>
                <IconCopy size={15} /> Copy to…
              </button>
              )}

              {/* Version history (files only) */}
              {!ctxMenu.item.isFolder && (
                <>
                  <div className="ctx-separator" />
                  <button type="button" role="menuitem" onClick={(e) => { e.stopPropagation(); doRevisions(ctxMenu.item!); }}>
                    <IconHistory size={15} /> Version history
                  </button>
                </>
              )}

              <div className="ctx-separator" />

              {/* Danger: Trash */}
              <button type="button" role="menuitem" className="is-danger" onClick={(e) => { e.stopPropagation(); setCtxMenu(null); void doTrash(ctxMenu.item!); }}>
                <IconTrash size={15} /> Move to trash
              </button>
            </>
          )}
        </div>
      )}

      <ToastHost toasts={toasts} onDismiss={dismissToast} />
      <UploadToastHost
        jobs={jobs}
        onCancel={(job) => void cancelJob(job as UploadJob)}
        onDismiss={dismissUploadJob}
      />

{shortcutHelpOpen && (
  <Suspense fallback={null}>
    <ShortcutHelp open={shortcutHelpOpen} onClose={() => setShortcutHelpOpen(false)} />
  </Suspense>
)}
{shareItem && (
  <Suspense fallback={null}>
    <ShareDialog item={shareItem} onClose={() => setShareItem(null)} onShared={() => showToast("Share link created")} />
  </Suspense>
)}
{revisionsItem && (
  <Suspense fallback={null}>
    <RevisionsPanel item={revisionsItem} onClose={() => setRevisionsItem(null)} />
  </Suspense>
)}
    </div>
  );
}