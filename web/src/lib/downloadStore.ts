import { useSyncExternalStore } from "react";
import { ApiError, parseError } from "./api";

/**
 * Download store — manages download jobs via the /api/downloads endpoints.
 * Uses the same useSyncExternalStore pattern as fileStore.ts.
 *
 * The store polls active downloads every 2s to keep progress fresh even when
 * the user is away from the page. Completed/failed jobs are kept in the list
 * until the user clears them.
 */

// ---- Types -----------------------------------------------------------------

export type DownloadStatus =
  | "pending"
  | "resolving"
  | "downloading"
  | "uploading"
  | "completed"
  | "failed"
  | "cancelled";

export type DownloadJob = {
  id: string;
  url: string;
  parentId?: string | null;
  status: DownloadStatus;
  progress: number; // 0-100
  speed?: string | null;
  eta?: string | null;
  downloaded: number; // bytes
  total: number; // bytes (0 if unknown)
  title?: string | null;
  thumbnail?: string | null;
  extractor?: string | null;
  uploader?: string | null;
  duration?: number;
  ext?: string | null;
  fileName?: string | null;
  driveFileId?: string | null;
  error?: string | null;
  createdAt: string;
  updatedAt: string;
};

export type DownloadSnapshot = {
  jobs: DownloadJob[];
  loading: boolean;
  error: string | null;
};

export type DownloadSinks = {
  onError?: (msg: string) => void;
  onUnauthorized?: () => void;
};

// ---- Module state ----------------------------------------------------------

let jobs: DownloadJob[] = [];
let loading = false;
let error: string | null = null;
let sinks: DownloadSinks = {};
let pollTimer: ReturnType<typeof setInterval> | null = null;
let pollInflight = false;

/** IDs that the user has deleted locally — we never re-add them from server. */
const deletedIds = new Set<string>();

const listeners = new Set<() => void>();

let snapshot: DownloadSnapshot = buildSnapshot();

function buildSnapshot(): DownloadSnapshot {
  return { jobs, loading, error };
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

function getSnapshot(): DownloadSnapshot {
  return snapshot;
}

// ---- Public hooks ----------------------------------------------------------

/** Subscribe a React component to the download snapshot. */
export function useDownloads(): DownloadSnapshot {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

/** Imperative read of the current snapshot. */
export function getDownloadSnapshot(): DownloadSnapshot {
  return snapshot;
}

/** Wire UI side effects. Call once when the app mounts. */
export function initDownloadStore(next: DownloadSinks) {
  sinks = next;
}

// ---- API helpers -----------------------------------------------------------

async function fetchJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, { credentials: "include", ...init });
  if (!res.ok) throw await parseError(res, "download");
  return res.json() as Promise<T>;
}

// ---- Actions ---------------------------------------------------------------

/** Fetch all download jobs from the server. */
export async function refreshDownloads(): Promise<void> {
  try {
    const data = await fetchJSON<{ jobs: DownloadJob[] }>("/api/downloads");
    // Merge: keep any local-only jobs (just-created, not yet on server list).
    // Filter out jobs that were deleted by the user (deletedIds).
    const serverIds = new Set(data.jobs.map((j) => j.id));
    const localOnly = jobs.filter((j) => !serverIds.has(j.id) && !deletedIds.has(j.id));
    const serverJobs = data.jobs.filter((j) => !deletedIds.has(j.id));
    jobs = [...localOnly, ...serverJobs];
    error = null;
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      sinks.onUnauthorized?.();
      error = null;
    } else {
      error = e instanceof Error ? e.message : "Failed to fetch downloads";
      sinks.onError?.(error);
    }
  } finally {
    emit();
  }
}

/** Load downloads on mount. */
export async function loadDownloads(): Promise<void> {
  if (loading) return;
  loading = true;
  emit();
  await refreshDownloads();
  loading = false;
  emit();
}

/** Create a new download job. */
export async function createDownload(url: string, parentId?: string): Promise<DownloadJob | null> {
  try {
    const body: Record<string, string> = { url };
    if (parentId) body.parentId = parentId;
    const data = await fetchJSON<DownloadJob>("/api/downloads", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    // Optimistic: insert at top so the user sees it immediately.
    if (!jobs.some((j) => j.id === data.id)) {
      jobs = [data, ...jobs];
      emit();
    }
    // Start polling if not already active.
    ensurePolling();
    return data;
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      sinks.onUnauthorized?.();
    } else {
      const msg = e instanceof Error ? e.message : "Failed to create download";
      sinks.onError?.(msg);
    }
    return null;
  }
}

/** Cancel a download job. */
export async function cancelDownload(id: string): Promise<void> {
  try {
    await fetchJSON<{ id: string; status: string }>(
      `/api/downloads/${encodeURIComponent(id)}/cancel`,
      { method: "POST" },
    );
    // Optimistic update
    jobs = jobs.map((j) => (j.id === id ? { ...j, status: "cancelled" as DownloadStatus } : j));
    emit();
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      sinks.onUnauthorized?.();
    } else {
      sinks.onError?.(e instanceof Error ? e.message : "Failed to cancel download");
    }
  }
}

/** Remove all completed/failed/cancelled jobs — calls server DELETE. */
export async function clearFinishedDownloads(): Promise<void> {
  const finishedIds = new Set(
    jobs
      .filter((j) => j.status === "completed" || j.status === "failed" || j.status === "cancelled")
      .map((j) => j.id),
  );
  // Optimistic: remove from local list immediately.
  jobs = jobs.filter((j) => !finishedIds.has(j.id));
  for (const id of finishedIds) deletedIds.add(id);
  emit();

  try {
    await fetchJSON<{ cleared: number }>("/api/downloads", { method: "DELETE" });
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      sinks.onUnauthorized?.();
    } else {
      sinks.onError?.(e instanceof Error ? e.message : "Failed to clear downloads");
    }
  }
}

/** Delete a single job from server and local state. */
export async function deleteDownload(id: string): Promise<void> {
  // Optimistic: remove from local list immediately.
  jobs = jobs.filter((j) => j.id !== id);
  deletedIds.add(id);
  emit();

  try {
    await fetchJSON<{ deleted: boolean }>(
      `/api/downloads/${encodeURIComponent(id)}`,
      { method: "DELETE" },
    );
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      sinks.onUnauthorized?.();
    } else {
      sinks.onError?.(e instanceof Error ? e.message : "Failed to delete download");
    }
  }
}

/** Retry just the upload phase for a failed job (no re-download). */
export async function retryUpload(id: string): Promise<void> {
  try {
    await fetchJSON<{ id: string; status: string }>(
      `/api/downloads/${encodeURIComponent(id)}/retry-upload`,
      { method: "POST" },
    );
    // Optimistic update
    jobs = jobs.map((j) =>
      j.id === id ? { ...j, status: "uploading" as DownloadStatus, error: null } : j,
    );
    emit();
    ensurePolling();
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      sinks.onUnauthorized?.();
    } else {
      sinks.onError?.(e instanceof Error ? e.message : "Failed to retry upload");
    }
  }
}

// ---- Polling ---------------------------------------------------------------

function hasActiveJobs(): boolean {
  return jobs.some(
    (j) =>
      j.status === "pending" ||
      j.status === "resolving" ||
      j.status === "downloading" ||
      j.status === "uploading",
  );
}

function ensurePolling() {
  if (pollTimer) return;
  pollTimer = setInterval(async () => {
    if (pollInflight) return;
    if (!hasActiveJobs()) {
      stopPolling();
      return;
    }
    pollInflight = true;
    await refreshDownloads();
    pollInflight = false;
    if (!hasActiveJobs()) stopPolling();
  }, 2000);
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

/** Reset on logout. */
export function resetDownloads(): void {
  stopPolling();
  jobs = [];
  loading = false;
  error = null;
  deletedIds.clear();
  emit();
}

// ---- Utilities -------------------------------------------------------------

export function formatDuration(s?: number): string {
  if (!s) return "";
  const m = Math.floor(s / 60);
  const sec = s % 60;
  return `${m}:${String(sec).padStart(2, "0")}`;
}

export function formatRelativeTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const now = Date.now();
  const diff = (now - d.getTime()) / 1000;
  if (diff < 60) return "just now";
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return d.toLocaleDateString();
}

// ---- Type icons (inline SVG strings) for fallback thumbnails ----
// Each extractor / file type gets a branded gradient background + SVG glyph.
export const TYPE_SVG_ICONS: Record<string, string> = {
  youtube:
    '<svg viewBox="0 0 24 24" width="28" height="28" fill="currentColor"><path d="M23.5 6.2a3 3 0 0 0-2.1-2.1C19.6 3.5 12 3.5 12 3.5s-7.6 0-9.4.6A3 3 0 0 0 .5 6.2C0 8 0 12 0 12s0 4 .5 5.8a3 3 0 0 0 2.1 2.1c1.8.6 9.4.6 9.4.6s7.6 0 9.4-.6a3 3 0 0 0 2.1-2.1c.5-1.8.5-5.8.5-5.8s0-4-.5-5.8zM9.6 15.6V8.4l6.2 3.6-6.2 3.6z"/></svg>',
  BiliBili:
    '<svg viewBox="0 0 24 24" width="26" height="26" fill="currentColor"><path d="M17.8 3.2c.4-.4.4-1 0-1.4-.4-.4-1-.4-1.4 0L14 4.2V3a1 1 0 1 0-2 0v2.4l-2-2c-.4-.4-1-.4-1.4 0-.4.4-.4 1 0 1.4l1.3 1.3A4 4 0 0 0 4 9.5v7a4 4 0 0 0 4 4h8a4 4 0 0 0 4-4v-7a4 4 0 0 0-3.5-3.9l1.3-1.4zM16 11a1 1 0 0 1 1 1v2a1 1 0 1 1-2 0v-2a1 1 0 0 1 1-1zm-8 0a1 1 0 0 1 1 1v2a1 1 0 1 1-2 0v-2a1 1 0 0 1 1-1z"/></svg>',
  Douyin:
    '<svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor"><path d="M19.6 6.3a4.8 4.8 0 0 1-3.8-2.3V15a5.6 5.6 0 1 1-5.6-5.6c.3 0 .6 0 .9.1v3a2.6 2.6 0 1 0 1.8 2.5V2h2.9a4.8 4.8 0 0 0 3.8 4.3v2z"/></svg>',
  TikTok:
    '<svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor"><path d="M19.6 6.3a4.8 4.8 0 0 1-3.8-2.3V15a5.6 5.6 0 1 1-5.6-5.6c.3 0 .6 0 .9.1v3a2.6 2.6 0 1 0 1.8 2.5V2h2.9a4.8 4.8 0 0 0 3.8 4.3v2z"/></svg>',
  Xiaohongshu:
    '<svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor"><path d="M12 2C6.5 2 2 6.5 2 12s4.5 10 10 10 10-4.5 10-10S17.5 2 12 2zm-2 14.5v-9l7 4.5-7 4.5z"/></svg>',
  generic:
    '<svg viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5"/></svg>',
};

/** Determine the CSS class for the fallback thumbnail based on extractor or ext. */
export function getThumbClass(extractor?: string | null, ext?: string | null): string {
  const ex = (extractor || "").toLowerCase();
  if (ex === "youtube") return "thumb-youtube";
  if (ex === "bilibili") return "thumb-bilibili";
  if (ex === "douyin") return "thumb-douyin";
  if (ex === "tiktok") return "thumb-tiktok";
  if (ex === "xiaohongshu") return "thumb-xiaohongshu";
  const e = (ext || "").toLowerCase();
  if (["jpg", "jpeg", "png", "gif", "webp", "bmp", "svg"].includes(e)) return "thumb-image";
  if (["mp3", "m4a", "wav", "flac", "aac", "ogg"].includes(e)) return "thumb-audio";
  if (["mp4", "webm", "mkv", "avi", "mov", "flv"].includes(e)) return "thumb-video";
  if (["pdf", "doc", "docx", "txt", "epub", "mobi"].includes(e)) return "thumb-document";
  return "thumb-generic";
}

/** Get the SVG icon string for a given extractor. */
export function getTypeIcon(extractor?: string | null): string {
  return TYPE_SVG_ICONS[extractor || ""] || TYPE_SVG_ICONS.generic;
}

export const STATUS_LABELS: Record<DownloadStatus, string> = {
  pending: "Pending",
  resolving: "Resolving",
  downloading: "Downloading",
  uploading: "Uploading",
  completed: "Completed",
  failed: "Failed",
  cancelled: "Cancelled",
};

export const ACTIVE_STATUSES: DownloadStatus[] = ["pending", "resolving", "downloading", "uploading"];
