import { useSyncExternalStore } from "react";
import { ApiError, cancelUpload, uploadFile } from "./api";

/**
 * Upload session store — lives outside React so progress ticks (≈80ms) only
 * re-render subscribers (UploadToastHost / job-count badge), not the whole App.
 */

export type UploadJob = {
  id: string;
  name: string;
  received: number;
  total: number;
  status: string;
  error?: string;
  serverId?: string;
  cancelling?: boolean;
};

export type UploadHistorySink = (it: {
  name: string;
  status: string;
  at: string;
  error?: string;
}) => void;

/** Fired when a resumable upload reaches a terminal state. */
export type UploadTerminalSink = (ev: {
  file: File;
  parentId?: string;
  ok: boolean;
  cancelled: boolean;
  fileId?: string | null;
  error?: string;
}) => void;

// Keep success cards visible longer — 3s felt like the upload "vanished" before
// the file list refreshed, leaving users unsure whether anything was uploaded.
const DISMISS_OK_MS = 12_000;
const DISMISS_CANCEL_MS = 4_000;

let jobs: UploadJob[] = [];
let seq = 0;
const listeners = new Set<() => void>();
const aborts = new Map<string, AbortController>();

function emit() {
  for (const l of listeners) l();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function getJobsSnapshot() {
  return jobs;
}

function getJobCountSnapshot() {
  return jobs.length;
}

function setJobs(next: UploadJob[] | ((prev: UploadJob[]) => UploadJob[])) {
  jobs = typeof next === "function" ? next(jobs) : next;
  emit();
}

function patchJob(id: string, patch: Partial<UploadJob>) {
  setJobs((prev) => prev.map((j) => (j.id === id ? { ...j, ...patch } : j)));
}

/** Full job list — for the floating progress host. */
export function useUploadJobs(): UploadJob[] {
  return useSyncExternalStore(subscribe, getJobsSnapshot, getJobsSnapshot);
}

/** Job count — length is stable during byte progress, so badges stay quiet mid-upload. */
export function useUploadJobCount(): number {
  return useSyncExternalStore(subscribe, getJobCountSnapshot, getJobCountSnapshot);
}

export function dismissUploadJob(id: string) {
  setJobs((prev) => prev.filter((j) => j.id !== id));
}

export function clearUploadJobs() {
  if (jobs.length === 0) return;
  setJobs([]);
}

/** Abort every in-flight client upload loop (e.g. on logout). */
export function abortAllUploads() {
  for (const ac of aborts.values()) ac.abort();
  aborts.clear();
}

export async function cancelUploadJob(job: UploadJob) {
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
  aborts.get(job.id)?.abort();
  if (job.serverId) {
    try {
      await cancelUpload(job.serverId);
    } catch {
      /* best-effort; abort still stops client loop */
    }
  }
}

/**
 * Run a resumable upload. Progress is stored externally — callers do not need
 * to hold React state. Returns true when Drive completed the file.
 */
export async function runUpload(
  file: File,
  parentId?: string,
  opts?: { onHistory?: UploadHistorySink; onTerminal?: UploadTerminalSink },
): Promise<boolean> {
  const id = `job-${++seq}`;
  const ac = new AbortController();
  aborts.set(id, ac);
  const draft: UploadJob = {
    id,
    name: file.name,
    received: 0,
    total: file.size,
    status: "starting",
  };
  setJobs((prev) => [draft, ...prev]);

  const history = opts?.onHistory;
  const onTerminal = opts?.onTerminal;

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
    history?.({
      name: file.name,
      status: cancelled ? "cancelled" : ok ? "completed" : job.status || "failed",
      at: new Date().toISOString(),
      error: cancelled ? undefined : job.error ?? undefined,
    });
    onTerminal?.({
      file,
      parentId,
      ok,
      cancelled,
      fileId: job.fileId,
      error: cancelled ? undefined : job.error ?? undefined,
    });
    if (cancelled) {
      window.setTimeout(() => dismissUploadJob(id), DISMISS_CANCEL_MS);
      return false;
    }
    if (ok) {
      // Auto-dismiss later; user can still dismiss manually. Failed jobs stay.
      window.setTimeout(() => dismissUploadJob(id), DISMISS_OK_MS);
    }
    return ok;
  } catch (e) {
    const aborted =
      ac.signal.aborted || (e instanceof ApiError && e.code === "upload_aborted");
    if (aborted) {
      patchJob(id, { status: "cancelled", cancelling: false, error: undefined });
      history?.({
        name: file.name,
        status: "cancelled",
        at: new Date().toISOString(),
      });
      onTerminal?.({ file, parentId, ok: false, cancelled: true });
      window.setTimeout(() => dismissUploadJob(id), DISMISS_CANCEL_MS);
      return false;
    }
    const err = e instanceof Error ? e.message : "Upload failed";
    patchJob(id, {
      status: "failed",
      error: err,
      cancelling: false,
    });
    history?.({
      name: file.name,
      status: "failed",
      at: new Date().toISOString(),
      error: err,
    });
    onTerminal?.({ file, parentId, ok: false, cancelled: false, error: err });
    // Failures stay until the user dismisses — don't hide errors.
    return false;
  } finally {
    aborts.delete(id);
  }
}
