import { describe, expect, it, vi, afterEach } from "vitest";
import {
  fetchFileContent,
  saveFileContent,
  isTextPreviewable,
  isImagePreviewable,
  fetchFileBlob,
} from "./api";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("isTextPreviewable", () => {
  it("recognizes text previewable files and folders", () => {
    const cases: Array<[boolean, { name: string; mimeType?: string; isFolder: boolean }]> = [
      [false, { name: "dir", isFolder: true }],
      [true, { name: "a.txt", mimeType: "text/plain", isFolder: false }],
      [true, { name: "b.MD", mimeType: "application/octet-stream", isFolder: false }],
      [true, { name: "c.json", mimeType: "application/json", isFolder: false }],
      [true, { name: "d.XML", mimeType: "application/xml", isFolder: false }],
      [false, { name: "e.bin", mimeType: "application/octet-stream", isFolder: false }],
    ];
    for (const [expectation, item] of cases) {
      expect(isTextPreviewable(item as any)).toBe(expectation);
    }
  });
});

describe("isImagePreviewable", () => {
  it("detects images by mime or extension", () => {
    expect(isImagePreviewable({ name: "a.png", mimeType: "image/png", isFolder: false })).toBe(true);
    expect(isImagePreviewable({ name: "b.JPG", mimeType: "application/octet-stream", isFolder: false })).toBe(true);
    expect(isImagePreviewable({ name: "c.txt", mimeType: "text/plain", isFolder: false })).toBe(false);
    expect(isImagePreviewable({ name: "dir", mimeType: "", isFolder: true })).toBe(false);
  });
});

describe("fetchFileBlob", () => {
  it("fetches download URL with credentials", async () => {
    const blob = new Blob(["img"], { type: "image/png" });
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, blob: async () => blob });
    vi.stubGlobal("fetch", fetchMock);
    const res = await fetchFileBlob("img_1");
    expect(res).toBe(blob);
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/files/img_1/download",
      expect.objectContaining({ credentials: "include" }),
    );
  });
});

describe("file content APIs", () => {
  it("fetchFileContent returns JSON body on 200", async () => {
    const body = { content: "hello", mimeType: "text/plain", size: 5, name: "a.txt" };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => body }));
    const res = await fetchFileContent("f_123");
    expect(res).toEqual(body);
    expect(fetch).toHaveBeenCalledWith('/api/files/f_123/content', expect.objectContaining({ credentials: 'include' }));
  });

  it("saveFileContent PUTs JSON and resolves on 200", async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal("fetch", fetchMock);
    await expect(saveFileContent('f_123', 'new content')).resolves.toBeUndefined();
    expect(fetchMock).toHaveBeenCalledWith('/api/files/f_123/content', expect.objectContaining({ method: 'PUT', credentials: 'include', headers: { 'Content-Type': 'application/json' } }));
  });
});
