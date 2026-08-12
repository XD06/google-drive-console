export type HealthResponse = {
  status: string;
  service: string;
  timeUtc: string;
  devMode?: boolean;
};

export type MeResponse = {
  email: string;
  connected: boolean;
};

export type FileItem = {
  id: string;
  name: string;
  mimeType: string;
  size: number | null;
  modifiedTime: string;
  isFolder: boolean;
  description?: string;
  thumbnailUrl?: string;
  md5Checksum?: string;
  version?: string;
};

export type FilesListResponse = {
  folderId: string;
  items: FileItem[];
  nextPageToken: string | null;
};

export type ApiErrorBody = {
  error?: {
    code?: string;
    message?: string;
  };
};

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

async function parseError(res: Response, fallbackCode: string): Promise<ApiError> {
  let code = fallbackCode;
  let message = `${fallbackCode} ${res.status}`;
  try {
    const body = (await res.json()) as ApiErrorBody;
    if (body.error?.code) code = body.error.code;
    if (body.error?.message) message = body.error.message;
  } catch {
    /* ignore non-JSON */
  }
  return new ApiError(res.status, code, message);
}

export async function fetchHealth(): Promise<HealthResponse> {
  const res = await fetch("/api/health");
  if (!res.ok) {
    throw new Error(`health ${res.status}`);
  }
  return res.json() as Promise<HealthResponse>;
}

export async function fetchMe(): Promise<MeResponse> {
  const res = await fetch("/api/auth/me", { credentials: "include" });
  if (!res.ok) {
    throw await parseError(res, "auth_me");
  }
  return res.json() as Promise<MeResponse>;
}

export async function logout(clearToken = false): Promise<void> {
  const q = clearToken ? "?clearToken=1" : "";
  const res = await fetch(`/api/auth/logout${q}`, {
    method: "POST",
    credentials: "include",
  });
  if (!res.ok) {
    throw await parseError(res, "logout");
  }
}

export type ListFilesParams = {
  folderId?: string;
  pageToken?: string;
  pageSize?: number;
};

export async function listFiles(
  params: ListFilesParams = {},
  signal?: AbortSignal,
): Promise<FilesListResponse> {
  const sp = new URLSearchParams();
  if (params.folderId) sp.set("folderId", params.folderId);
  if (params.pageToken) sp.set("pageToken", params.pageToken);
  // 100 per page (vs Drive's 50 default) halves the round-trips needed to
  // fully load large folders; fields are trimmed server-side so the payload
  // stays small.
  sp.set("pageSize", String(params.pageSize ?? 100));
  const qs = sp.toString();
  const res = await fetch(`/api/files${qs ? `?${qs}` : ""}`, {
    credentials: "include",
    signal,
  });
  if (!res.ok) {
    throw await parseError(res, "list_files");
  }
  const data = (await res.json()) as FilesListResponse;
  if (!data.items) data.items = [];
  return data;
}

export type SearchScope = "folder" | "drive";

export type SearchFilesParams = {
  q: string;
  scope?: SearchScope;
  folderId?: string;
  pageToken?: string;
  pageSize?: number;
  signal?: AbortSignal;
};

export type SearchFilesResponse = {
  query: string;
  scope: SearchScope | string;
  folderId?: string;
  items: FileItem[];
  nextPageToken: string | null;
};

export async function searchFiles(
  params: SearchFilesParams,
): Promise<SearchFilesResponse> {
  const sp = new URLSearchParams();
  sp.set("q", params.q);
  if (params.scope) sp.set("scope", params.scope);
  if (params.folderId) sp.set("folderId", params.folderId);
  if (params.pageToken) sp.set("pageToken", params.pageToken);
  if (params.pageSize != null) sp.set("pageSize", String(params.pageSize));
  const res = await fetch(`/api/files/search?${sp.toString()}`, {
    credentials: "include",
    signal: params.signal,
  });
  if (!res.ok) {
    throw await parseError(res, "search_files");
  }
  const data = (await res.json()) as SearchFilesResponse;
  if (!data.items) data.items = [];
  if (data.nextPageToken === undefined) data.nextPageToken = null;
  return data;
}

export async function createFolder(name: string, parentId?: string): Promise<FileItem> {
  const res = await fetch("/api/files/mkdir", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, parentId }),
  });
  if (!res.ok) {
    throw await parseError(res, "create_folder");
  }
  return res.json() as Promise<FileItem>;
}

export function mimeFromFilename(name: string): string {
  const ext = (name || "").trim().toLowerCase().split(".");
  const last = ext.length > 1 ? `.${ext[ext.length - 1]}` : "";
  switch (last) {
    case ".md":
    case ".markdown":
      return "text/markdown";
    case ".json":
      return "application/json";
    case ".xml":
      return "application/xml";
    case ".html":
    case ".htm":
      return "text/html";
    case ".css":
      return "text/css";
    case ".js":
      return "text/javascript";
    case ".ts":
      return "text/plain";
    case ".csv":
      return "text/csv";
    case ".yaml":
    case ".yml":
      return "text/yaml";
    case ".log":
    case ".env":
    case ".txt":
      return "text/plain";
    default:
      return "text/plain";
  }
}

export async function createFile(input: {
  name: string;
  parentId?: string;
  mimeType?: string;
  content?: string;
}): Promise<FileItem> {
  const res = await fetch("/api/files", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!res.ok) {
    throw await parseError(res, "create_file");
  }
  return res.json() as Promise<FileItem>;
}

// simpleUpload sends a small file (<5MB) as raw binary in one request.
// Uses uploadType=multipart on the backend — saves 2 round-trips vs resumable.
export async function simpleUpload(file: File, parentId?: string): Promise<FileItem> {
  const params = new URLSearchParams({
    name: file.name,
    mimeType: file.type || "application/octet-stream",
  });
  if (parentId && parentId !== "root") params.set("parentId", parentId);
  const res = await fetch(`/api/files/simple?${params}`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": file.type || "application/octet-stream" },
    body: file,
  });
  if (!res.ok) {
    throw await parseError(res, "simple_upload");
  }
  return res.json() as Promise<FileItem>;
}

export const SIMPLE_UPLOAD_THRESHOLD = 5 * 1024 * 1024; // 5MB

export async function trashFile(id: string): Promise<void> {
  const res = await fetch(`/api/files/${encodeURIComponent(id)}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok) {
    throw await parseError(res, "trash_file");
  }
}

export async function renameFile(id: string, name: string): Promise<FileItem> {
  const res = await fetch(`/api/files/${encodeURIComponent(id)}`, {
    method: "PATCH",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  if (!res.ok) {
    throw await parseError(res, "rename_file");
  }
  return res.json() as Promise<FileItem>;
}

// updateFile patches a file's name and/or description via the metadata PATCH.
// Passing description: "" clears it. At least one field must be provided.
export async function updateFile(
  id: string,
  patch: { name?: string; description?: string },
): Promise<FileItem> {
  const res = await fetch(`/api/files/${encodeURIComponent(id)}`, {
    method: "PATCH",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(patch),
  });
  if (!res.ok) {
    throw await parseError(res, "update_file");
  }
  return res.json() as Promise<FileItem>;
}

export async function moveFile(id: string, parentId: string, fromParentId?: string): Promise<FileItem> {
const res = await fetch(`/api/files/${encodeURIComponent(id)}/move`, {
method: "POST",
credentials: "include",
headers: { "Content-Type": "application/json" },
body: JSON.stringify({ parentId, fromParentId }),
});
  if (!res.ok) {
    throw await parseError(res, "move_file");
  }
  return res.json() as Promise<FileItem>;
}

/**
 * Download URL. When list metadata is supplied it is appended as query
 * params so the server can skip its Drive GetMeta round-trip (2 RTT → 1 RTT)
 * and emit a strong ETag immediately.
 */
export function fileDownloadUrl(
  id: string,
  meta?: Pick<FileItem, "name" | "mimeType" | "size" | "md5Checksum" | "version">,
): string {
  const base = `/api/files/${encodeURIComponent(id)}/download`;
  if (!meta || !meta.mimeType) return base;
  const sp = new URLSearchParams();
  sp.set("mime", meta.mimeType);
  if (meta.name) sp.set("name", meta.name);
  if (meta.size != null) sp.set("size", String(meta.size));
  if (meta.md5Checksum) sp.set("md5", meta.md5Checksum);
  else if (meta.version) sp.set("ver", meta.version);
  return `${base}?${sp.toString()}`;
}

/** Thumbnail proxy URL — uses server-side OAuth to fetch Google's thumbnail.
 *  When thumbnailLink is available from the list response, pass it to skip
 *  the server-side GetMeta round-trip (eliminates N+1 on folder grids). */
export function fileThumbnailUrl(id: string, thumbnailLink?: string): string {
  const base = `/api/files/${encodeURIComponent(id)}/thumbnail`;
  if (thumbnailLink) {
    return `${base}?link=${encodeURIComponent(thumbnailLink)}`;
  }
  return base;
}

export function formatBytes(n: number | null | undefined): string {
  if (n == null || Number.isNaN(n)) return "—";
  if (n < 1024) return `${n} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let v = n;
  let i = -1;
  do {
    v /= 1024;
    i += 1;
  } while (v >= 1024 && i < units.length - 1);
  return `${v.toFixed(v >= 10 || i === 0 ? 0 : 1)} ${units[i]}`;
}

export function formatModified(iso: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export type FileContentResponse = {
  content: string;
  mimeType: string;
  size: number;
  name: string;
};

export type OverviewResponse = {
  storage: { limit: number; usage: number; usageInDrive: number };
  user: { email: string; displayName: string };
};

export async function fetchOverview(): Promise<OverviewResponse> {
  const res = await fetch(`/api/overview`, { credentials: "include" });
  if (!res.ok) {
    throw await parseError(res, "overview");
  }
  return res.json() as Promise<OverviewResponse>;
}

export async function fetchFileContent(id: string): Promise<FileContentResponse> {
  const res = await fetch(`/api/files/${encodeURIComponent(id)}/content`, { credentials: "include" });
  if (!res.ok) {
    throw await parseError(res, "fetch_file_content");
  }
  return res.json() as Promise<FileContentResponse>;
}

export async function saveFileContent(id: string, content: string): Promise<void> {
  const res = await fetch(`/api/files/${encodeURIComponent(id)}/content`, {
    method: "PUT",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ content }),
  });
  if (!res.ok) {
    throw await parseError(res, "save_file_content");
  }
}

export function isTextPreviewable(item: Pick<FileItem, "name" | "mimeType" | "isFolder">): boolean {
  if (item.isFolder) return false;
  const mime = item.mimeType || "";
  const name = item.name || "";
  const lowerMime = mime.toLowerCase();
  if (lowerMime.startsWith("text/")) return true;
  if (lowerMime === "application/json" || lowerMime === "application/xml") return true;
  const exts = [
    ".txt", ".md", ".markdown", ".json", ".csv", ".log", ".yaml", ".yml", ".xml", ".html", ".htm", ".css", ".js", ".ts", ".env",
  ];
  const idx = name.lastIndexOf(".");
  if (idx >= 0) {
    const ext = name.substring(idx).toLowerCase();
    if (exts.includes(ext)) return true;
  }
  return false;
}

const IMAGE_EXTS = [".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg", ".ico", ".heic", ".avif"];

export function isImagePreviewable(item: Pick<FileItem, "name" | "mimeType" | "isFolder">): boolean {
  if (item.isFolder) return false;
  const mime = (item.mimeType || "").toLowerCase();
  if (mime.startsWith("image/")) return true;
  const name = item.name || "";
  const idx = name.lastIndexOf(".");
  if (idx >= 0) {
    const ext = name.substring(idx).toLowerCase();
    if (IMAGE_EXTS.includes(ext)) return true;
  }
  return false;
}

/** Fetch file bytes with session cookie (for image lightbox). */
export async function fetchFileBlob(
  id: string,
  meta?: Pick<FileItem, "name" | "mimeType" | "size" | "md5Checksum" | "version">,
): Promise<Blob> {
  const res = await fetch(fileDownloadUrl(id, meta), { credentials: "include" });
  if (!res.ok) {
    throw await parseError(res, "fetch_file_blob");
  }
  return res.blob();
}

export type UploadJob = {
  uploadId: string;
  status: string;
  total: number;
  name?: string;
  fileId?: string | null;
  bytesSent?: number;
  bytesReceived?: number;
  error?: string | null;
};

export async function createUpload(body: {
  name: string;
  size: number;
  parentId?: string;
  mimeType?: string;
}): Promise<UploadJob> {
  const res = await fetch("/api/uploads", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw await parseError(res, "create_upload");
  }
  return res.json() as Promise<UploadJob>;
}

export async function cancelUpload(uploadId: string): Promise<void> {
  const res = await fetch(`/api/uploads/${encodeURIComponent(uploadId)}/cancel`, {
    method: "POST",
    credentials: "include",
  });
  if (!res.ok) {
    throw await parseError(res, "cancel_upload");
  }
}

/** Default client chunk size (16 MiB). */
export const UPLOAD_CHUNK_SIZE = 16 * 1024 * 1024;

/** Max client chunk size per API contract (32 MiB). */
export const MAX_CLIENT_CHUNK = 32 * 1024 * 1024;

/**
 * Adaptive chunk size based on file size (U2).
 * < 200MB: 16 MiB, 200MB-1GB: 32 MiB, >1GB: 32 MiB (capped).
 * For 5-32MB files, returns file size (single-chunk upload, U5).
 */
export function getAdaptiveChunkSize(fileSize: number): number {
  // U5: files between 5MB and 32MB use single-chunk upload
  if (fileSize > SIMPLE_UPLOAD_THRESHOLD && fileSize <= MAX_CLIENT_CHUNK) {
    return fileSize;
  }
  // U2: adaptive chunk size for larger files
  if (fileSize > 1024 * 1024 * 1024) {
    return MAX_CLIENT_CHUNK; // 32 MiB for >1GB
  }
  if (fileSize > 200 * 1024 * 1024) {
    return MAX_CLIENT_CHUNK; // 32 MiB for >200MB
  }
  return UPLOAD_CHUNK_SIZE; // 16 MiB default
}

function parseApiErrorFromXhr(xhr: XMLHttpRequest, fallbackCode: string): ApiError {
  let code = fallbackCode;
  let message = `${fallbackCode} ${xhr.status}`;
  try {
    const body = JSON.parse(xhr.responseText || "{}") as ApiErrorBody;
    if (body.error?.code) code = body.error.code;
    if (body.error?.message) message = body.error.message;
  } catch {
    /* ignore non-JSON */
  }
  return new ApiError(xhr.status, code, message);
}

/** Chunk progress: client = browser→server, drive = server→Google (may be slow). */
export type ChunkProgressPhase = "client" | "drive";

/**
 * PUT one upload chunk via XHR so upload progress events fire continuously
 * (fetch has no upload progress API).
 *
 * After the browser finishes sending the body, the Go server still uploads to
 * Drive — that phase can take a long time. We poll job status so the bar keeps
 * reflecting real BytesSent instead of freezing on a fake estimate.
 */
export async function putUploadChunk(
  uploadId: string,
  chunk: Blob | ArrayBuffer,
  offset: number,
  total: number,
  onChunkProgress?: (
    loaded: number,
    chunkSize: number,
    phase: ChunkProgressPhase,
    absoluteBytes?: number,
  ) => void,
  signal?: AbortSignal,
): Promise<UploadJob> {
  const body = chunk instanceof Blob ? chunk : new Blob([chunk]);
  const chunkSize = body.size;
  const end = offset + chunkSize - 1;

  return new Promise<UploadJob>((resolve, reject) => {
    if (signal?.aborted) {
      reject(new ApiError(0, "upload_aborted", "upload cancelled"));
      return;
    }

    const xhr = new XMLHttpRequest();
    xhr.open("PUT", `/api/uploads/${encodeURIComponent(uploadId)}/chunk`);
    xhr.withCredentials = true;
    xhr.setRequestHeader("Content-Range", `bytes ${offset}-${end}/${total}`);
    xhr.setRequestHeader("X-Upload-Offset", String(offset));
    xhr.responseType = "text";

    const onAbort = () => xhr.abort();
    signal?.addEventListener("abort", onAbort);

    let pollTimer: ReturnType<typeof setInterval> | null = null;
    let pollInflight = false;

    const cleanup = () => {
      signal?.removeEventListener("abort", onAbort);
      if (pollTimer) {
        clearInterval(pollTimer);
        pollTimer = null;
      }
    };

    xhr.upload.onprogress = (ev) => {
      if (!onChunkProgress) return;
      // Real browser→server bytes (not a 0–50% fake scale).
      const loaded = ev.lengthComputable ? ev.loaded : 0;
      onChunkProgress(Math.min(loaded, chunkSize), chunkSize, "client");
    };

    xhr.upload.onload = () => {
      if (!onChunkProgress) return;
      // Client transfer done; server is now flushing this range to Drive.
      onChunkProgress(chunkSize, chunkSize, "drive", offset);
      // Poll absolute server progress so the bar does not freeze for minutes.
      pollTimer = setInterval(() => {
        if (pollInflight || signal?.aborted) return;
        pollInflight = true;
        void getUploadStatus(uploadId)
          .then((st) => {
            const abs = Math.max(st.bytesSent ?? 0, st.bytesReceived ?? 0, offset);
            onChunkProgress?.(chunkSize, chunkSize, "drive", abs);
          })
          .catch(() => {
            /* ignore transient status errors during flush */
          })
          .finally(() => {
            pollInflight = false;
          });
      }, 700);
    };

    xhr.onload = () => {
      cleanup();
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          resolve(JSON.parse(xhr.responseText || "{}") as UploadJob);
        } catch {
          reject(new ApiError(xhr.status, "upload_chunk", "invalid JSON response"));
        }
        return;
      }
      reject(parseApiErrorFromXhr(xhr, "upload_chunk"));
    };
    xhr.onerror = () => {
      cleanup();
      reject(new ApiError(0, "upload_chunk", "network error during upload"));
    };
    xhr.onabort = () => {
      cleanup();
      reject(new ApiError(0, "upload_aborted", "upload cancelled"));
    };
    xhr.send(body);
  });
}

/**
 * Query server-side upload status (for resume after failure).
 */
export async function getUploadStatus(uploadId: string): Promise<UploadJob> {
  const res = await fetch(`/api/uploads/${encodeURIComponent(uploadId)}`, {
    credentials: "include",
  });
  if (!res.ok) throw await parseError(res, "upload_status");
  return res.json() as Promise<UploadJob>;
}

export async function uploadFile(
  file: File,
  opts: {
    parentId?: string;
    onProgress?: (received: number, total: number, status: string) => void;
    onCreated?: (uploadId: string) => void;
    signal?: AbortSignal;
  } = {},
): Promise<UploadJob> {
  if (opts.signal?.aborted) {
    throw new ApiError(0, "upload_aborted", "upload cancelled");
  }

  const job = await createUpload({
    name: file.name,
    size: file.size,
    parentId: opts.parentId,
    mimeType: file.type || "application/octet-stream",
  });
  opts.onCreated?.(job.uploadId);

  if (opts.signal?.aborted) {
    try {
      await cancelUpload(job.uploadId);
    } catch {
      /* best-effort */
    }
    throw new ApiError(0, "upload_aborted", "upload cancelled");
  }

  if (job.status === "completed") {
    opts.onProgress?.(file.size, file.size, "completed");
    return job;
  }

  opts.onProgress?.(0, file.size, job.status || "uploading");

  const chunkSize = getAdaptiveChunkSize(file.size);
  let offset = 0;
  let current: UploadJob = job;
  let lastUi = 0;
  // Never let the bar jump backwards (pipeline races / mixed client+drive reports).
  let hiWater = 0;
  let totalRetries = 0;
  const MAX_RETRIES = 5; // total retry budget per upload
  let usePipeline = true; // disable after first failure

  const report = (bytes: number, status: string) => {
    const n = Math.max(0, Math.min(file.size, Math.max(hiWater, bytes)));
    hiWater = n;
    opts.onProgress?.(n, file.size, status);
  };

  const onChunkUi = (
    base: number,
    loaded: number,
    cs: number,
    phase: ChunkProgressPhase,
    absoluteBytes?: number,
  ) => {
    const now = typeof performance !== "undefined" ? performance.now() : Date.now();
    // Always push phase changes / absolute Drive polls; throttle only fine client ticks.
    if (phase === "client" && loaded < cs && now - lastUi < 80) return;
    lastUi = now;
    if (phase === "drive") {
      // Prefer server-confirmed absolute bytes when available.
      const abs = absoluteBytes != null ? absoluteBytes : base + loaded;
      report(abs, "flushing");
      return;
    }
    report(base + loaded, "uploading");
  };

  while (offset < file.size) {
    if (opts.signal?.aborted) {
      try { await cancelUpload(job.uploadId); } catch { /* */ }
      throw new ApiError(0, "upload_aborted", "upload cancelled");
    }

    try {
      if (usePipeline) {
        // Pipelined: 2 chunks inflight
        const inflight: Promise<UploadJob>[] = [];
        const inflightMeta: { base: number; end: number }[] = [];

        while (offset < file.size && inflight.length < 2) {
          const end = Math.min(offset + chunkSize, file.size);
          const base = offset;
          inflight.push(
            putUploadChunk(
              job.uploadId,
              file.slice(offset, end),
              offset,
              file.size,
              (loaded, cs, phase, abs) => onChunkUi(base, loaded, cs, phase, abs),
              opts.signal,
            ),
          );
          inflightMeta.push({ base, end });
          offset = end;
        }

        // Await inflight chunks
        while (inflight.length > 0) {
          current = await inflight.shift()!;
          const meta = inflightMeta.shift()!;
          lastUi = 0;
          const confirmed = current.bytesSent ?? current.bytesReceived ?? meta.end;
          report(
            confirmed,
            current.status === "uploading" ? "uploading" : current.status,
          );
          if (current.status === "completed" || current.status === "failed" || current.status === "cancelled") {
            await Promise.allSettled(inflight);
            if (current.status === "completed") return current;
            throw new ApiError(400, "chunk_failed", current.error || "chunk failed");
          }
        }
      } else {
        // Sequential: one chunk at a time (after a retry)
        const end = Math.min(offset + chunkSize, file.size);
        const base = offset;
        current = await putUploadChunk(
          job.uploadId,
          file.slice(offset, end),
          offset,
          file.size,
          (loaded, cs, phase, abs) => onChunkUi(base, loaded, cs, phase, abs),
          opts.signal,
        );
        lastUi = 0;
        report(
          current.bytesSent ?? current.bytesReceived ?? end,
          current.status === "uploading" ? "uploading" : current.status,
        );
        if (current.status === "completed") return current;
        if (current.status === "failed" || current.status === "cancelled") {
          throw new ApiError(400, "chunk_failed", current.error || "chunk failed");
        }
        offset = end;
      }
    } catch (e) {
      // Abort: don't retry
      if (opts.signal?.aborted || (e instanceof ApiError && e.code === "upload_aborted")) {
        throw e;
      }

      totalRetries++;
      if (totalRetries > MAX_RETRIES) {
        // Exhausted retry budget
        if (e instanceof Error) throw e;
        throw new ApiError(0, "upload_failed", "upload failed after retries");
      }

      // Switch to sequential mode after any failure
      usePipeline = false;

      // Backoff: 1s, 2s, 3s...
      await new Promise((r) => setTimeout(r, totalRetries * 1000));

      // Query server for actual confirmed offset
      try {
        const status = await getUploadStatus(job.uploadId);
        if (status.status === "completed") {
          report(file.size, "completed");
          return status;
        }
        if (status.status === "failed" || status.status === "cancelled") {
          throw new ApiError(400, "upload_failed", status.error || "upload failed on server");
        }
        // Resume from server-confirmed offset
        offset = status.bytesReceived ?? status.bytesSent ?? 0;
        hiWater = offset; // reset monotonic floor to the resume point
        report(offset, "uploading");
      } catch (statusErr) {
        // If status query also fails, re-throw original error
        if (statusErr instanceof ApiError && (statusErr.code === "upload_failed" || statusErr.code === "upload_aborted")) {
          throw statusErr;
        }
        // Otherwise retry from last known offset (optimistic)
      }
    }
  }

  // Final: drain completed
  if (current.status !== "completed" && offset >= file.size) {
    // All chunks sent, query final status
    try {
      const final = await getUploadStatus(job.uploadId);
      if (final.status === "completed") {
        report(file.size, "completed");
        return final;
      }
      return final;
    } catch {
      return current;
    }
  }
  return current;
}

// ---- P1 API extensions ----

export async function copyFile(id: string, parentId?: string, name?: string): Promise<FileItem> {
  const res = await fetch(`/api/files/${encodeURIComponent(id)}/copy`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ parentId, name }),
  });
  if (!res.ok) throw await parseError(res, "copy_file");
  return res.json() as Promise<FileItem>;
}

export type ShareInfo = {
  shareUrl?: string;
  permission: { id: string; type: string; role: string; linkShare: boolean };
  anyoneCanRead: boolean;
};

export async function shareFile(id: string, role?: string): Promise<ShareInfo> {
  const res = await fetch(`/api/files/${encodeURIComponent(id)}/share`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ role }),
  });
  if (!res.ok) throw await parseError(res, "share_file");
  return res.json() as Promise<ShareInfo>;
}

export async function unshareFile(id: string, permissionId: string): Promise<void> {
  const res = await fetch(`/api/files/${encodeURIComponent(id)}/share?permissionId=${encodeURIComponent(permissionId)}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok) throw await parseError(res, "unshare_file");
}

export type Revision = {
  id: string;
  modifiedTime: string;
  size: number | null;
  lastModifyingUser?: string;
};

export async function listRevisions(id: string): Promise<Revision[]> {
  const res = await fetch(`/api/files/${encodeURIComponent(id)}/revisions`, { credentials: "include" });
  if (!res.ok) throw await parseError(res, "list_revisions");
  const data = await res.json() as { revisions: Revision[] };
  return data.revisions || [];
}

export async function restoreRevision(id: string, rev: string): Promise<void> {
  const res = await fetch(`/api/files/${encodeURIComponent(id)}/revisions/${encodeURIComponent(rev)}/restore`, {
    method: "POST",
    credentials: "include",
  });
  if (!res.ok) throw await parseError(res, "restore_revision");
}

export function zipDownloadUrl(id: string): string {
  return `/api/files/${encodeURIComponent(id)}/zip`;
}

export function isPdfPreviewable(item: Pick<FileItem, "name" | "mimeType" | "isFolder">): boolean {
  if (item.isFolder) return false;
  const mime = (item.mimeType || "").toLowerCase();
  if (mime === "application/pdf") return true;
  return (item.name || "").toLowerCase().endsWith(".pdf");
}

export function isVideoPreviewable(item: Pick<FileItem, "name" | "mimeType" | "isFolder">): boolean {
  if (item.isFolder) return false;
  const mime = (item.mimeType || "").toLowerCase();
  if (mime.startsWith("video/")) return true;
  const exts = [".mp4", ".webm", ".ogg", ".mov", ".mkv", ".avi"];
  const name = (item.name || "").toLowerCase();
  return exts.some((e) => name.endsWith(e));
}

// ---- D1: Range first-block preview ----

/**
 * Fetches the first `bytes` of a file using HTTP Range (D1).
 * Useful for progressive preview of PDFs and videos without downloading the entire file.
 * Falls back to full download if the server doesn't support Range.
 */
export async function fetchFileRange(id: string, bytes: number): Promise<Blob> {
  const res = await fetch(`${fileDownloadUrl(id)}`, {
    headers: { Range: `bytes=0-${bytes - 1}` },
    credentials: "include",
  });
  if (!res.ok && res.status !== 206) throw await parseError(res, "fetch_range");
  return res.blob();
}

// ---- L2: Batch operations ----

export type BatchResult = {
  succeeded: number;
  failed: number;
  errors: { id: string; error: string }[];
};

export async function batchTrash(ids: string[]): Promise<BatchResult> {
  const res = await fetch("/api/files/batch", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action: "trash", ids }),
  });
  if (!res.ok) throw await parseError(res, "batch_trash");
  return res.json() as Promise<BatchResult>;
}

export async function batchMove(ids: string[], parentId: string): Promise<BatchResult> {
  const res = await fetch("/api/files/batch", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action: "move", ids, parentId }),
  });
  if (!res.ok) throw await parseError(res, "batch_move");
  return res.json() as Promise<BatchResult>;
}

// ---- F3: Multi-select ZIP download ----

/**
 * Triggers a multi-file ZIP download (F3).
 * Sends a POST with file IDs and names, receives a streaming ZIP.
 */
export async function downloadMultiZip(items: { id: string; name: string }[]): Promise<void> {
  const res = await fetch("/api/files/zip", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ items }),
  });
  if (!res.ok) throw await parseError(res, "multi_zip");

  // Preferred path: stream the ZIP straight to disk via the File System Access
  // API so the whole archive is never buffered in memory (matters for large
  // multi-file selections). Chromium-only + secure-context; falls back below.
  const body = res.body;
  const picker = (
    window as unknown as {
      showSaveFilePicker?: (opts?: {
        suggestedName?: string;
        types?: { description?: string; accept: Record<string, string[]> }[];
      }) => Promise<{ createWritable: () => Promise<WritableStream<Uint8Array>> }>;
    }
  ).showSaveFilePicker;
  if (picker && body) {
    let handle: { createWritable: () => Promise<WritableStream<Uint8Array>> } | undefined;
    try {
      handle = await picker({
        suggestedName: "selected-files.zip",
        types: [{ description: "ZIP archive", accept: { "application/zip": [".zip"] } }],
      });
    } catch (err) {
      // User dismissed the save dialog → nothing to do.
      if (err instanceof DOMException && err.name === "AbortError") return;
      // Picker failed before the body was touched → fall back to the blob path.
      handle = undefined;
    }
    if (handle) {
      // From here the response body is consumed; no blob fallback is possible.
      const writable = await handle.createWritable();
      await body.pipeTo(writable);
      return;
    }
  }

  // Fallback: buffer to a blob and trigger an anchor download.
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "selected-files.zip";
  document.body.appendChild(a);
  a.click();
  a.remove();
  // Delay revoke — some browsers abort the download if the blob URL is
  // revoked synchronously after click(), before the navigation starts.
  setTimeout(() => URL.revokeObjectURL(url), 60_000);
}

// ---- API keys (/api/v1/keys, session-only management) ----

export type ApiKeyScope = "read" | "readwrite";

export type ApiKeyItem = {
  id: string;
  name: string;
  hint: string;
  scope: ApiKeyScope;
  createdAt: string;
  lastUsedAt?: string;
  revoked: boolean;
};

export type CreatedApiKey = ApiKeyItem & { token: string };

export async function listApiKeys(): Promise<ApiKeyItem[]> {
  const res = await fetch("/api/v1/keys", { credentials: "include" });
  if (!res.ok) throw await parseError(res, "keys_list");
  const body = (await res.json()) as { keys: ApiKeyItem[] };
  return body.keys ?? [];
}

export async function createApiKey(name: string, scope: ApiKeyScope): Promise<CreatedApiKey> {
  const res = await fetch("/api/v1/keys", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, scope }),
  });
  if (!res.ok) throw await parseError(res, "keys_create");
  return res.json() as Promise<CreatedApiKey>;
}

export async function revokeApiKey(id: string): Promise<void> {
  const res = await fetch(`/api/v1/keys/${encodeURIComponent(id)}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!res.ok && res.status !== 204) throw await parseError(res, "keys_revoke");
}