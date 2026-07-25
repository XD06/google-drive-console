import { describe, expect, it, vi, afterEach } from "vitest";
import {
  ApiError,
  createUpload,
  fetchHealth,
  fetchMe,
  fetchOverview,
  formatBytes,
  listFiles,
  logout,
  putUploadChunk,
  UPLOAD_CHUNK_SIZE,
  createFolder,
  trashFile,
  searchFiles,
  renameFile,
  moveFile,
} from "./api";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("fetchHealth", () => {
  it("parses JSON on success", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          status: "ok",
          service: "drive-backup-console",
          timeUtc: "2026-07-21T00:00:00Z",
          devMode: true,
        }),
      }),
    );

    const h = await fetchHealth();
    expect(h.status).toBe("ok");
    expect(h.service).toBe("drive-backup-console");
  });

  it("throws on non-ok", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
      }),
    );
    await expect(fetchHealth()).rejects.toThrow(/health 500/);
  });
});

describe("fetchMe", () => {
  it("returns email when authenticated", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ email: "a@b.com", connected: true }),
      }),
    );
    const me = await fetchMe();
    expect(me.email).toBe("a@b.com");
    expect(me.connected).toBe(true);
  });

  it("throws ApiError on 401", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
        json: async () => ({
          error: { code: "not_authenticated", message: "Sign in" },
        }),
      }),
    );
    await expect(fetchMe()).rejects.toMatchObject({
      name: "ApiError",
      status: 401,
      code: "not_authenticated",
    } satisfies Partial<ApiError>);
  });
});

describe("listFiles", () => {
  it("sends folderId query and normalizes items", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        folderId: "abc",
        items: null,
        nextPageToken: null,
      }),
    });
    vi.stubGlobal("fetch", fetchMock);

    const res = await listFiles({ folderId: "abc" });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/files?folderId=abc",
      expect.objectContaining({ credentials: "include" }),
    );
    expect(res.items).toEqual([]);
    expect(res.folderId).toBe("abc");
  });
});

describe("logout", () => {
  it("POSTs logout", async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) });
    vi.stubGlobal("fetch", fetchMock);
    await logout();
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/auth/logout",
      expect.objectContaining({ method: "POST", credentials: "include" }),
    );
  });
});

describe("formatBytes", () => {
  it("formats sizes", () => {
    expect(formatBytes(null)).toBe("—");
    expect(formatBytes(500)).toBe("500 B");
    expect(formatBytes(2048)).toBe("2 KB");
  });
});

describe("createUpload", () => {
  it("POSTs JSON body", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        uploadId: "up_1",
        status: "pending",
        total: 10,
        name: "a.txt",
      }),
    });
    vi.stubGlobal("fetch", fetchMock);
    const j = await createUpload({ name: "a.txt", size: 10 });
    expect(j.uploadId).toBe("up_1");
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/uploads",
      expect.objectContaining({ method: "POST", credentials: "include" }),
    );
  });
});

describe("putUploadChunk", () => {
  it("sends Content-Range and offset headers via XHR", async () => {
    class FakeXHR {
      static last: FakeXHR | null = null;
      status = 200;
      responseText = JSON.stringify({
        uploadId: "up_1",
        status: "completed",
        total: 4,
        bytesReceived: 4,
        bytesSent: 4,
      });
      responseType = "";
      withCredentials = false;
      upload = {
        onprogress: null as ((ev: ProgressEvent) => void) | null,
      };
      onload: (() => void) | null = null;
      onerror: (() => void) | null = null;
      onabort: (() => void) | null = null;
      opened: { method: string; url: string } | null = null;
      headers: Record<string, string> = {};
      sent: Blob | ArrayBuffer | null = null;

      constructor() {
        FakeXHR.last = this;
      }
      open(method: string, url: string) {
        this.opened = { method, url };
      }
      setRequestHeader(k: string, v: string) {
        this.headers[k] = v;
      }
      send(body?: Document | XMLHttpRequestBodyInit | null) {
        this.sent = (body as Blob | ArrayBuffer) ?? null;
        queueMicrotask(() => this.onload?.());
      }
    }
    vi.stubGlobal("XMLHttpRequest", FakeXHR as unknown as typeof XMLHttpRequest);

    const buf = new Uint8Array([1, 2, 3, 4]).buffer;
    const job = await putUploadChunk("up_1", buf, 0, 4);
    expect(job.status).toBe("completed");
    expect(FakeXHR.last?.opened).toEqual({
      method: "PUT",
      url: "/api/uploads/up_1/chunk",
    });
    expect(FakeXHR.last?.headers["Content-Range"]).toBe("bytes 0-3/4");
    expect(FakeXHR.last?.headers["X-Upload-Offset"]).toBe("0");
    expect(FakeXHR.last?.withCredentials).toBe(true);
    expect(UPLOAD_CHUNK_SIZE).toBe(16 * 1024 * 1024);
  });
});

describe("createFolder", () => {
  it("POSTs JSON body and returns FileItem on 201", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      json: async () => ({ id: "f_1", name: "New Folder", mimeType: "application/vnd.google-apps.folder", size: null, modifiedTime: "2026-07-21T00:00:00Z", isFolder: true }),
    });
    vi.stubGlobal("fetch", fetchMock);
    const res = await createFolder("New Folder", "parent_1");
    expect(res.id).toBe("f_1");
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/files/mkdir",
      expect.objectContaining({
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
      }),
    );
  });
});

describe("fetchOverview", () => {
  it("fetches overview and parses JSON", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ storage: { limit: 1000, usage: 200, usageInDrive: 150 }, user: { email: "a@b.com", displayName: "AB" } }),
    });
    vi.stubGlobal("fetch", fetchMock);
    const res = await fetchOverview();
    expect(res.storage.limit).toBe(1000);
    expect(res.user.email).toBe("a@b.com");
  });

  it("throws ApiError on 401", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
        json: async () => ({ error: { code: "not_authenticated", message: "Sign in" } }),
      }),
    );
    await expect(fetchOverview()).rejects.toMatchObject({ name: "ApiError", status: 401, code: "not_authenticated" } as Partial<ApiError>);
  });
});

describe("trashFile", () => {
  it("DELETEs and resolves on 204", async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 204 });
    vi.stubGlobal("fetch", fetchMock);
    await expect(trashFile("f_1")).resolves.toBeUndefined();
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/files/f_1",
      expect.objectContaining({ method: "DELETE", credentials: "include" }),
    );
  });
});

describe("searchFiles", () => {
  it("queries /api/files/search with scope and signal", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        query: "bak",
        scope: "drive",
        items: [
          {
            id: "1",
            name: "backup.zip",
            mimeType: "application/zip",
            size: 10,
            modifiedTime: "2026-07-21T00:00:00Z",
            isFolder: false,
          },
        ],
        nextPageToken: null,
      }),
    });
    vi.stubGlobal("fetch", fetchMock);
    const ac = new AbortController();
    const res = await searchFiles({ q: "bak", scope: "drive", signal: ac.signal });
    expect(res.items).toHaveLength(1);
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/files/search?q=bak&scope=drive",
      expect.objectContaining({ credentials: "include", signal: ac.signal }),
    );
  });

  it("includes folderId for folder scope", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ query: "ab", scope: "folder", folderId: "fld", items: [], nextPageToken: null }),
    });
    vi.stubGlobal("fetch", fetchMock);
    await searchFiles({ q: "ab", scope: "folder", folderId: "fld", pageSize: 8 });
    expect(fetchMock.mock.calls[0][0]).toContain("scope=folder");
    expect(fetchMock.mock.calls[0][0]).toContain("folderId=fld");
    expect(fetchMock.mock.calls[0][0]).toContain("pageSize=8");
  });
});

describe("renameFile", () => {
  it("PATCHes name and returns FileItem", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        id: "f_1",
        name: "new.txt",
        mimeType: "text/plain",
        size: 1,
        modifiedTime: "2026-07-22T00:00:00Z",
        isFolder: false,
      }),
    });
    vi.stubGlobal("fetch", fetchMock);
    const res = await renameFile("f_1", "new.txt");
    expect(res.name).toBe("new.txt");
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/files/f_1",
      expect.objectContaining({
        method: "PATCH",
        credentials: "include",
        body: JSON.stringify({ name: "new.txt" }),
      }),
    );
  });
});

describe("moveFile", () => {
  it("POSTs parentId to /move", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        id: "f_1",
        name: "a.txt",
        mimeType: "text/plain",
        size: 1,
        modifiedTime: "2026-07-22T00:00:00Z",
        isFolder: false,
      }),
    });
    vi.stubGlobal("fetch", fetchMock);
    await moveFile("f_1", "folder_2");
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/files/f_1/move",
      expect.objectContaining({
        method: "POST",
        credentials: "include",
        body: JSON.stringify({ parentId: "folder_2" }),
      }),
    );
  });
});