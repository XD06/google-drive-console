import type { ReactNode, SVGProps } from "react";

type IconProps = SVGProps<SVGSVGElement> & {
  size?: number;
};

function Base({ size = 16, children, ...rest }: IconProps & { children: ReactNode }) {
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
      {...rest}
    >
      {children}
    </svg>
  );
}

export function IconUpload(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M12 16V4" />
      <path d="M7 9l5-5 5 5" />
      <path d="M4 20h16" />
    </Base>
  );
}

export function IconFolderPlus(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M4 7h4l2-2h6a2 2 0 0 1 2 2v1" />
      <path d="M4 7a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h10" />
      <path d="M16 13h6M19 10v6" />
    </Base>
  );
}

export function IconFilePlus(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
      <path d="M14 3v5h5" />
      <path d="M12 12v6M9 15h6" />
    </Base>
  );
}

export function IconPlus(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M12 5v14M5 12h14" />
    </Base>
  );
}

export function IconActivity(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M4 14h3l2-5 3 10 2-6h4" />
    </Base>
  );
}

export function IconRefresh(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M21 12a9 9 0 1 1-2.64-6.36" />
      <path d="M21 3v6h-6" />
    </Base>
  );
}

export function IconSettings(p: IconProps) {
  return (
    <Base {...p}>
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9c.3.6.9 1 1.6 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z" />
    </Base>
  );
}

export function IconSearch(p: IconProps) {
  return (
    <Base {...p}>
      <circle cx="11" cy="11" r="7" />
      <path d="M20 20l-3.5-3.5" />
    </Base>
  );
}

export function IconChevronLeft(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M15 18l-6-6 6-6" />
    </Base>
  );
}

export function IconTrash(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M3 6h18" />
      <path d="M8 6v14a2 2 0 0 0 2 2h4a2 2 0 0 0 2-2V6" />
      <path d="M10 11v6M14 11v6" />
      <path d="M9 6V4h6v2" />
    </Base>
  );
}

export function IconDownload(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M12 5v10" />
      <path d="M8 11l4 4 4-4" />
      <path d="M4 21h16" />
    </Base>
  );
}

export function IconEye(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z" />
      <circle cx="12" cy="12" r="3" />
    </Base>
  );
}

export function IconLogout(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
      <path d="M16 17l5-5-5-5" />
      <path d="M21 12H9" />
    </Base>
  );
}

export function IconDrive(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M4 7h4l2-2h8a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V9a2 2 0 0 1 2-2z" />
    </Base>
  );
}

export function IconGrid(p: IconProps) {
  return (
    <Base {...p}>
      <rect x="3" y="3" width="7" height="7" rx="1.5" />
      <rect x="14" y="3" width="7" height="7" rx="1.5" />
      <rect x="3" y="14" width="7" height="7" rx="1.5" />
      <rect x="14" y="14" width="7" height="7" rx="1.5" />
    </Base>
  );
}

export function IconOpen(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M13 11V6l-8 8 8 8v-5h8V11z" />
    </Base>
  );
}

export function IconFileText(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
      <path d="M14 3v5h5" />
      <path d="M9 13h6M9 17h4" />
    </Base>
  );
}

export function IconClose(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M6 6l12 12M18 6L6 18" />
    </Base>
  );
}

export function IconCheck(p: IconProps) {
  return (
    <Base {...p} strokeWidth={p.strokeWidth ?? 2.2}>
      <path d="M20 6L9 17l-5-5" />
    </Base>
  );
}

export function IconAlert(p: IconProps) {
  return (
    <Base {...p} strokeWidth={p.strokeWidth ?? 2.2}>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 8v5M12 17h.01" />
    </Base>
  );
}

export function IconChevronUp(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M6 14l6-6 6 6" />
    </Base>
  );
}

export function IconChevronDown(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M6 10l6 6 6-6" />
    </Base>
  );
}

export function IconStop(p: IconProps) {
  return (
    <Base {...p}>
      <rect x="6" y="6" width="12" height="12" rx="2" />
    </Base>
  );
}

export function IconRename(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M12 20h9" />
      <path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z" />
    </Base>
  );
}

export function IconMenu(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M4 7h16" />
      <path d="M4 12h16" />
      <path d="M4 17h16" />
    </Base>
  );
}

export function IconMove(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M5 12h14" />
      <path d="M13 6l6 6-6 6" />
      <path d="M5 6v12" />
    </Base>
  );
}

export function IconMore(p: IconProps) {
  return (
    <Base {...p}>
      <circle cx="5" cy="12" r="1.6" fill="currentColor" stroke="none" />
      <circle cx="12" cy="12" r="1.6" fill="currentColor" stroke="none" />
      <circle cx="19" cy="12" r="1.6" fill="currentColor" stroke="none" />
    </Base>
  );
}

export function IconCopy(p: IconProps) {
  return (
    <Base {...p}>
      <rect x="9" y="9" width="11" height="11" rx="2" />
      <path d="M5 15V5a2 2 0 0 1 2-2h10" />
    </Base>
  );
}

export function IconZoomIn(p: IconProps) {
  return (
    <Base {...p}>
      <circle cx="11" cy="11" r="7" />
      <path d="M20 20l-3.5-3.5" />
      <path d="M11 8v6M8 11h6" />
    </Base>
  );
}

export function IconZoomOut(p: IconProps) {
  return (
    <Base {...p}>
      <circle cx="11" cy="11" r="7" />
      <path d="M20 20l-3.5-3.5" />
      <path d="M8 11h6" />
    </Base>
  );
}

export function IconShare(p: IconProps) {
  return (
    <Base {...p}>
      <circle cx="18" cy="5" r="3" />
      <circle cx="6" cy="12" r="3" />
      <circle cx="18" cy="19" r="3" />
      <path d="M8.6 10.5l6.8-4M8.6 13.5l6.8 4" />
    </Base>
  );
}

export function IconHistory(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M3 12a9 9 0 1 0 3-6.7" />
      <path d="M3 4v5h5" />
      <path d="M12 8v4l3 2" />
    </Base>
  );
}

export function IconZip(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
      <path d="M14 3v5h5" />
      <path d="M10 12v1M10 15v1M10 18v1" />
    </Base>
  );
}

export function IconPause(p: IconProps) {
  return (
    <Base {...p}>
      <rect x="6" y="5" width="4" height="14" rx="1" fill="currentColor" stroke="none" />
      <rect x="14" y="5" width="4" height="14" rx="1" fill="currentColor" stroke="none" />
    </Base>
  );
}

export function IconFilter(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M3 5h18l-7 9v6l-4-2v-4z" />
    </Base>
  );
}

export function IconClock(p: IconProps) {
  return (
    <Base {...p}>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7v5l3 2" />
    </Base>
  );
}

export function IconSun(p: IconProps) {
  return (
    <Base {...p}>
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M4 12H2M22 12h-2M5 5l1.5 1.5M17.5 17.5L19 19M19 5l-1.5 1.5M6.5 17.5L5 19" />
    </Base>
  );
}

export function IconMoon(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
    </Base>
  );
}

export function IconHelp(p: IconProps) {
  return (
    <Base {...p}>
      <circle cx="12" cy="12" r="9" />
      <path d="M9.5 9a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .8-1 1.5" />
      <path d="M12 17h.01" />
    </Base>
  );
}
