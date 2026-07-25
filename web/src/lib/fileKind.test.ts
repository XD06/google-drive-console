import { describe, expect, it } from "vitest";
import { fileKind, fileKindLabel } from "./fileKind";

describe("fileKind", () => {
  it("detects folders", () => {
    expect(fileKind({ isFolder: true, name: "a", mimeType: "application/vnd.google-apps.folder" })).toBe(
      "folder",
    );
  });

  it("detects by extension", () => {
    expect(fileKind({ name: "x.pdf", mimeType: "application/octet-stream" })).toBe("pdf");
    expect(fileKind({ name: "x.otf", mimeType: "application/octet-stream" })).toBe("font");
    expect(fileKind({ name: "x.txt", mimeType: "application/octet-stream" })).toBe("text");
    expect(fileKind({ name: "x.json", mimeType: "application/octet-stream" })).toBe("code");
    expect(fileKind({ name: "x.zip", mimeType: "application/octet-stream" })).toBe("archive");
  });

  it("detects by mime", () => {
    expect(fileKind({ name: "a", mimeType: "image/png" })).toBe("image");
    expect(fileKind({ name: "a", mimeType: "video/mp4" })).toBe("video");
    expect(fileKind({ name: "a", mimeType: "application/pdf" })).toBe("pdf");
    expect(fileKind({ name: "a", mimeType: "text/plain" })).toBe("text");
  });

  it("labels kinds", () => {
    expect(fileKindLabel("pdf")).toBe("PDF");
    expect(fileKindLabel("folder")).toBe("Folder");
  });
});
