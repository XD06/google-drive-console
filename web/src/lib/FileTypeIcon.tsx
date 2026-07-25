import type { ReactNode } from "react";
import type { FileKind } from "./fileKind";
import { fileKind } from "./fileKind";

type Props = {
  item: { name?: string; mimeType?: string; isFolder?: boolean };
  size?: number;
};

function Svg({
  size,
  children,
}: {
  size: number;
  children: ReactNode;
}) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.75"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {children}
    </svg>
  );
}

export function FileTypeIcon({ item, size = 16 }: Props) {
  const kind = fileKind(item);
  return <KindIcon kind={kind} size={size} />;
}

export function KindIcon({ kind, size = 16 }: { kind: FileKind; size?: number }) {
  switch (kind) {
    case "folder":
      return (
        <Svg size={size}>
          <path d="M4 7h4l2-2h8a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V9a2 2 0 0 1 2-2z" />
        </Svg>
      );
    case "pdf":
      return (
        <Svg size={size}>
          <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
          <path d="M14 3v5h5" />
          <path d="M8 13h3a1.5 1.5 0 0 1 0 3H8v2" />
          <path d="M14 13v5" />
          <path d="M14 15.5h2" />
        </Svg>
      );
    case "image":
      return (
        <Svg size={size}>
          <rect x="3" y="5" width="18" height="14" rx="2" />
          <circle cx="9" cy="10" r="1.5" />
          <path d="M3 16l5-4 4 3 3-2 6 5" />
        </Svg>
      );
    case "video":
      return (
        <Svg size={size}>
          <rect x="3" y="6" width="14" height="12" rx="2" />
          <path d="M17 10l4-2v8l-4-2v-4z" />
        </Svg>
      );
    case "audio":
      return (
        <Svg size={size}>
          <path d="M9 18V6l10-2v12" />
          <circle cx="7" cy="18" r="2" />
          <circle cx="17" cy="16" r="2" />
        </Svg>
      );
    case "archive":
      return (
        <Svg size={size}>
          <path d="M4 7h16v12a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V7z" />
          <path d="M4 7l2-3h12l2 3" />
          <path d="M10 11h4M10 14h4M12 11v6" />
        </Svg>
      );
    case "font":
      return (
        <Svg size={size}>
          <path d="M6 20l4-16h2l4 16" />
          <path d="M8.5 14h5" />
        </Svg>
      );
    case "code":
      return (
        <Svg size={size}>
          <path d="M8 8l-4 4 4 4" />
          <path d="M16 8l4 4-4 4" />
          <path d="M13 6l-2 12" />
        </Svg>
      );
    case "text":
      return (
        <Svg size={size}>
          <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
          <path d="M14 3v5h5" />
          <path d="M8 13h8M8 17h6" />
        </Svg>
      );
    case "sheet":
      return (
        <Svg size={size}>
          <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
          <path d="M14 3v5h5" />
          <path d="M8 12h8M8 16h8M12 12v8" />
        </Svg>
      );
    case "doc":
      return (
        <Svg size={size}>
          <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
          <path d="M14 3v5h5" />
          <path d="M8 13h8M8 17h5" />
        </Svg>
      );
    case "slides":
      return (
        <Svg size={size}>
          <rect x="3" y="5" width="18" height="12" rx="2" />
          <path d="M8 21h8M12 17v4" />
        </Svg>
      );
    case "google":
      return (
        <Svg size={size}>
          <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
          <path d="M14 3v5h5" />
          <path d="M9 14h6" />
          <circle cx="12" cy="14" r="3.5" />
        </Svg>
      );
    default:
      return (
        <Svg size={size}>
          <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
          <path d="M14 3v5h5" />
        </Svg>
      );
  }
}

export function iconBoxClass(kind: FileKind): string {
  return `icon-box kind-${kind}`;
}
