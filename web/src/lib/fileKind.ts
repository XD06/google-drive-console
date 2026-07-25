/** File kind for list icons / overview type buckets. */

export type FileKind =
  | "folder"
  | "pdf"
  | "image"
  | "video"
  | "audio"
  | "archive"
  | "font"
  | "code"
  | "text"
  | "sheet"
  | "doc"
  | "slides"
  | "google"
  | "other";

const EXT_KIND: Record<string, FileKind> = {
  // text
  txt: "text",
  md: "text",
  markdown: "text",
  log: "text",
  csv: "sheet",
  tsv: "sheet",
  // code
  json: "code",
  xml: "code",
  html: "code",
  htm: "code",
  css: "code",
  scss: "code",
  js: "code",
  jsx: "code",
  ts: "code",
  tsx: "code",
  go: "code",
  py: "code",
  rs: "code",
  java: "code",
  c: "code",
  h: "code",
  cpp: "code",
  cs: "code",
  sh: "code",
  bat: "code",
  ps1: "code",
  yml: "code",
  yaml: "code",
  env: "code",
  toml: "code",
  sql: "code",
  // docs
  pdf: "pdf",
  doc: "doc",
  docx: "doc",
  rtf: "doc",
  odt: "doc",
  xls: "sheet",
  xlsx: "sheet",
  ods: "sheet",
  ppt: "slides",
  pptx: "slides",
  odp: "slides",
  // media
  png: "image",
  jpg: "image",
  jpeg: "image",
  gif: "image",
  webp: "image",
  svg: "image",
  bmp: "image",
  ico: "image",
  heic: "image",
  mp4: "video",
  webm: "video",
  mov: "video",
  mkv: "video",
  avi: "video",
  mp3: "audio",
  wav: "audio",
  flac: "audio",
  aac: "audio",
  ogg: "audio",
  m4a: "audio",
  // archive
  zip: "archive",
  rar: "archive",
  "7z": "archive",
  tar: "archive",
  gz: "archive",
  tgz: "archive",
  bz2: "archive",
  // font
  ttf: "font",
  otf: "font",
  woff: "font",
  woff2: "font",
};

function extOf(name: string): string {
  const i = name.lastIndexOf(".");
  if (i < 0 || i === name.length - 1) return "";
  return name.slice(i + 1).toLowerCase();
}

export function fileKind(item: {
  name?: string;
  mimeType?: string;
  isFolder?: boolean;
}): FileKind {
  if (item.isFolder) return "folder";
  const mime = (item.mimeType || "").toLowerCase();
  const name = item.name || "";
  const ext = extOf(name);

  if (mime === "application/vnd.google-apps.folder") return "folder";
  if (mime.startsWith("application/vnd.google-apps.")) return "google";
  if (mime === "application/pdf" || ext === "pdf") return "pdf";
  if (mime.startsWith("image/")) return "image";
  if (mime.startsWith("video/")) return "video";
  if (mime.startsWith("audio/")) return "audio";
  if (
    mime.includes("zip") ||
    mime.includes("compressed") ||
    mime.includes("tar") ||
    mime.includes("gzip") ||
    mime === "application/x-7z-compressed" ||
    mime === "application/vnd.rar"
  ) {
    return "archive";
  }
  if (mime.startsWith("font/") || mime.includes("font-sfnt") || mime.includes("font-woff")) {
    return "font";
  }
  if (
    mime.includes("spreadsheet") ||
    mime === "text/csv" ||
    mime.includes("excel")
  ) {
    return "sheet";
  }
  if (mime.includes("presentation") || mime.includes("powerpoint")) return "slides";
  if (
    mime.includes("word") ||
    mime.includes("document") ||
    mime === "application/msword" ||
    mime === "application/rtf"
  ) {
    return "doc";
  }
  if (
    mime.startsWith("text/") ||
    mime === "application/json" ||
    mime === "application/xml" ||
    mime === "application/javascript" ||
    mime === "application/typescript"
  ) {
    if (
      mime.includes("javascript") ||
      mime.includes("typescript") ||
      mime === "application/json" ||
      mime === "application/xml" ||
      mime === "text/css" ||
      mime === "text/html" ||
      mime === "text/x-python" ||
      mime === "text/x-go"
    ) {
      return "code";
    }
    return "text";
  }

  if (ext && EXT_KIND[ext]) return EXT_KIND[ext];
  return "other";
}

export function fileKindLabel(kind: FileKind): string {
  switch (kind) {
    case "folder":
      return "Folder";
    case "pdf":
      return "PDF";
    case "image":
      return "Image";
    case "video":
      return "Video";
    case "audio":
      return "Audio";
    case "archive":
      return "Archive";
    case "font":
      return "Font";
    case "code":
      return "Code";
    case "text":
      return "Text";
    case "sheet":
      return "Spreadsheet";
    case "doc":
      return "Document";
    case "slides":
      return "Slides";
    case "google":
      return "Google Doc";
    default:
      return "Other";
  }
}
