import { useSyncExternalStore } from "react";
import { ApiError, listFiles, type FileItem } from "./api";

/**
 * File-list store — the "My Drive" listing subsystem lives outside React so the
 * god-component App.tsx can shrink to a thin shell and future pages can be added
 * as self-contained components. Mirrors the useSyncExternalStore pattern already
 * proven in uploadSession.ts.
 *
 * Behaviour is a verbatim port of the original App.tsx logic: AbortController
 * supersession, an LRU folder cache (stale-while-revalidate), a stale-folder
 * guard for pagination, in-place row patching that keeps the cache in sync, and
 * duplicate folder-open debouncing. Side effects that used to live in App
 * (toasts, sign-out, recent-folder tracking) are delegated through injected
 * sinks so the store stays UI-agnostic.
 */

export type Crumb = { id: string; name: string };

export type FileListSnapshot = {
  items: FileItem[];
  folderId: string | undefined;
  trail: Crumb[];
  nextToken: string | null;
  loading: boolean;
  loadingMore: boolean;
  error: string | null;
  /**
   * Monotonic counter bumped ONLY on a full-table replacement (cache-hit paint
   * and network-success). loadMoreFiles / patchItems / row inserts do NOT bump
   * it. The App subscribes to this to reproduce the original "navigation clears
   * the selection" semantics without moving selection into the store.
   */
  version: number;
};

export type FileStoreSinks = {
  /** Non-fatal error surfaced to the user (e.g. "load more" failure). */
  onError?: (msg: string) => void;
  /** A 401 was observed — the session is gone. */
  onUnauthorized?: () => void;
  /** A folder was opened via openFolder (drives the recent-folders list). */
  onFolderOpened?: (id: string, name: string) => void;
};

type CacheEntry = { items: FileItem[]; folderId: string; nextPageToken: string | null };

// --- module state -----------------------------------------------------------
let items: FileItem[] = [];
let folderId: string | undefined = undefined;
let trail: Crumb[] = [];
let nextToken: string | null = null;
let loading = false;
let loadingMore = false;
let error: string | null = null;
let version = 0;

let sinks: FileStoreSinks = {};
const listeners = new Set<() => void>();

// Simple in-memory cache for stale-while-revalidate (LRU capped at 30 entries).
const folderCache = new Map<string, CacheEntry>();
const FOLDER_CACHE_MAX = 30;

// AbortController for list requests — cancels stale fetches on rapid navigation.
let listAbort: AbortController | null = null;

// Guards against duplicate folder activations (double-tap on touch devices).
const lastFolderOpen: { id: string; at: number } = { id: "", at: 0 };

// --- snapshot plumbing ------------------------------------------------------
// useSyncExternalStore requires getSnapshot to return a STABLE reference between
// changes, so the snapshot object is rebuilt only inside emit().
let snapshot: FileListSnapshot = buildSnapshot();

function buildSnapshot(): FileListSnapshot {
  return { items, folderId, trail, nextToken, loading, loadingMore, error, version };
}

function emit() {
  snapshot = buildSnapshot();
  for (const l of listeners) l();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function getSnapshot(): FileListSnapshot {
  return snapshot;
}

/** Subscribe a React component to the file-list snapshot. */
export function useFileList(): FileListSnapshot {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

/** Imperative read of the current snapshot (tests / non-render call sites). */
export function getFileListSnapshot(): FileListSnapshot {
  return snapshot;
}

/** Wire the UI side effects. Call once when the app mounts. */
export function initFileStore(next: FileStoreSinks) {
  sinks = next;
}

// Normalize root: state may be undefined or "root" depending on the path.
function normFolder(id?: string) {
  return !id || id === "root" ? "root" : id;
}

function folderCacheSet(key: string, value: CacheEntry) {
  // Delete first to refresh insertion order (Map preserves order).
  folderCache.delete(key);
  folderCache.set(key, value);
  // Evict oldest entries if over the limit.
  while (folderCache.size > FOLDER_CACHE_MAX) {
    const oldest = folderCache.keys().next().value;
    if (oldest !== undefined) folderCache.delete(oldest);
    else break;
  }
}

// --- actions ----------------------------------------------------------------

export async function loadFiles(fid?: string): Promise<void> {
  // Cancel any in-flight list request.
  listAbort?.abort();
  const ac = new AbortController();
  listAbort = ac;

  const cacheKey = fid ?? "root";
  const cached = folderCache.get(cacheKey);
  if (cached) {
    // Show cached data immediately (no loading screen); revalidate silently.
    items = cached.items;
    folderId = cached.folderId;
    nextToken = cached.nextPageToken;
    error = null;
    version++; // full-table replacement
    emit();
  } else {
    loading = true;
    error = null;
    nextToken = null;
    emit();
  }

  try {
    const res = await listFiles(fid ? { folderId: fid } : {}, ac.signal);
    // If this request was superseded, discard the result.
    if (ac.signal.aborted) return;
    items = res.items;
    folderId = res.folderId;
    nextToken = res.nextPageToken ?? null;
    error = null;
    version++; // full-table replacement
    folderCacheSet(cacheKey, {
      items: res.items,
      folderId: res.folderId,
      nextPageToken: res.nextPageToken ?? null,
    });
  } catch (e) {
    if (ac.signal.aborted) return; // cancelled — ignore
    if (e instanceof ApiError && e.status === 401) {
      sinks.onUnauthorized?.();
      error = null;
    } else {
      error = e instanceof Error ? e.message : "Failed to list files";
    }
    if (!cached) {
      items = [];
      nextToken = null;
    }
  } finally {
    if (!ac.signal.aborted) {
      loading = false;
      emit();
    }
  }
}

/**
 * Patch rows in place (live list + folder cache) right after a mutation, so the
 * background revalidation triggered afterwards never re-shows stale rows. Does
 * NOT bump version (selection must survive an in-place edit).
 */
export function patchItems(updater: (items: FileItem[]) => FileItem[]) {
  items = updater(items);
  const key = folderId ?? "root";
  const cached = folderCache.get(key);
  if (cached) {
    folderCacheSet(key, { ...cached, items: updater(cached.items) });
  }
  emit();
}

export async function loadMoreFiles(): Promise<void> {
  if (!nextToken || loadingMore || loading) return;
  // Capture the folder at call time — a navigate-away while this page is in
  // flight must not merge page-2 of A into B's listing. Compare against the
  // live module folderId (always current), not a captured closure.
  const requestFolderId = folderId;
  const requestKey = normFolder(requestFolderId);
  const requestToken = nextToken;
  loadingMore = true;
  emit();
  try {
    const res = await listFiles({
      folderId: requestFolderId && requestFolderId !== "root" ? requestFolderId : undefined,
      pageToken: requestToken,
    });
    if (normFolder(folderId) !== requestKey) return;
    const newToken = res.nextPageToken ?? null;
    const seen = new Set(items.map((it) => it.id));
    const merged = [...items];
    for (const it of res.items) {
      if (!seen.has(it.id)) merged.push(it);
    }
    items = merged;
    nextToken = newToken;
    // Keep the folder cache in sync so a later silent revalidation doesn't drop
    // the extra page we just loaded.
    const cached = folderCache.get(requestKey);
    if (cached) {
      folderCacheSet(requestKey, { ...cached, items: merged, nextPageToken: newToken });
    }
  } catch (e) {
    if (normFolder(folderId) !== requestKey) return;
    if (e instanceof ApiError && e.status === 401) {
      sinks.onUnauthorized?.();
    } else {
      sinks.onError?.(e instanceof Error ? e.message : "Failed to load more");
    }
  } finally {
    loadingMore = false;
    emit();
  }
}

/**
 * Open a folder. Owns the breadcrumb trail, the duplicate-activation guard and
 * the recent-folder side effect. Returns true when navigation actually
 * happened, so the caller can gate its own UI resets (search mode etc.) exactly
 * like the original — a debounced duplicate is a no-op.
 */
export function openFolder(item: FileItem, opts: { fromSearch: boolean }): boolean {
  if (!item.isFolder) return false;
  const fromSearch = opts.fromSearch;
  if (!fromSearch) {
    // Already inside (or already entering) this folder — ignore the duplicate.
    if (folderId === item.id || trail[trail.length - 1]?.id === item.id) return false;
    const now = Date.now();
    if (lastFolderOpen.id === item.id && now - lastFolderOpen.at < 500) return false;
    lastFolderOpen.id = item.id;
    lastFolderOpen.at = now;
  }
  if (fromSearch) {
    trail = [{ id: item.id, name: item.name }];
  } else {
    // Backstop: never append the same folder twice in a row (duplicate keys).
    trail =
      trail[trail.length - 1]?.id === item.id
        ? trail
        : [...trail, { id: item.id, name: item.name }];
  }
  emit();
  sinks.onFolderOpened?.(item.id, item.name);
  void loadFiles(item.id);
  return true;
}

/** Navigate via a breadcrumb. index < 0 means "root". */
export function goCrumb(index: number): void {
  if (index < 0) {
    trail = [];
    emit();
    void loadFiles(undefined);
    return;
  }
  const next = trail.slice(0, index + 1);
  trail = next;
  emit();
  void loadFiles(next[next.length - 1]?.id);
}

/** Open a folder from the recent-folders list (resets the trail to a single crumb). */
export function openRecentFolder(id: string, name: string): void {
  trail = [{ id, name }];
  emit();
  void loadFiles(id);
}

/** Enter a folder from a search hit (resets the trail to a single crumb). */
export function enterFolderFromSearch(item: FileItem): void {
  trail = [{ id: item.id, name: item.name }];
  emit();
  void loadFiles(item.id);
}

/** Drop a cache entry (e.g. after copying into another folder). */
export function bustCache(key: string): void {
  folderCache.delete(key);
}

/**
 * Insert a completed resumable (large) upload into the open folder. Always
 * busts the destination cache; only mutates the visible list if the user is
 * still in that folder, and never clobbers an existing row (a revalidate may
 * already hold the real one).
 */
export function insertResumableRow(destKey: string, row: FileItem): void {
  folderCache.delete(destKey);
  if (normFolder(folderId) !== destKey) return;
  if (items.some((it) => it.id === row.id)) return;
  items = [row, ...items];
  emit();
}

/**
 * Insert a completed simple (small) upload. Always busts the destination cache;
 * only mutates the visible list if the user is still in that folder, replacing
 * any stale copy and hoisting the fresh server row to the top.
 */
export function insertSimpleRow(destKey: string, row: FileItem): void {
  folderCache.delete(destKey);
  if (normFolder(folderId) !== destKey) return;
  items = [row, ...items.filter((it) => it.id !== row.id)];
  emit();
}

/**
 * Background-revalidate a destination folder after uploads (Drive eventual
 * consistency / name-sorted lists). Busts the cache and only refetches if the
 * user is still viewing that folder.
 */
export function revalidateIfViewing(destKey: string): void {
  folderCache.delete(destKey);
  if (normFolder(folderId) !== destKey) return;
  void loadFiles(destKey === "root" ? undefined : destKey);
}

/** Restore persisted location on refresh (App then calls loadFiles). */
export function hydrate(next: { folderId?: string; trail: Crumb[] }): void {
  folderId = next.folderId;
  trail = next.trail;
  emit();
}

/** Clear everything on logout. */
export function resetOnLogout(): void {
  listAbort?.abort();
  listAbort = null;
  items = [];
  folderId = undefined;
  trail = [];
  nextToken = null;
  loading = false;
  loadingMore = false;
  error = null;
  folderCache.clear();
  lastFolderOpen.id = "";
  lastFolderOpen.at = 0;
  version++;
  emit();
}
