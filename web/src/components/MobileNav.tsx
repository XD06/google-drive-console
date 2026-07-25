import { useState } from "react";
import {
  IconDrive,
  IconFilePlus,
  IconFolderPlus,
  IconGrid,
  IconPlus,
  IconUpload,
} from "../lib/icons";

export type MobileNavProps = {
  view: "files" | "overview";
  busy: boolean;
  onOpenFiles: () => void;
  onOpenOverview: () => void;
  onUpload: () => void;
  onNewFolder: () => void;
  onNewFile: () => void;
};

// MobileNav renders the phone-only chrome: a bottom tab bar for switching the
// primary views and a floating "+" button (FAB) that expands into a small
// speed-dial for the create/upload actions. It is hidden on desktop via CSS
// (.bottom-nav / .fab-dock default to display:none), so it can always be
// mounted without affecting the desktop layout.
export function MobileNav({
  view,
  busy,
  onOpenFiles,
  onOpenOverview,
  onUpload,
  onNewFolder,
  onNewFile,
}: MobileNavProps) {
  const [fabOpen, setFabOpen] = useState(false);
  const close = () => setFabOpen(false);
  const run = (fn: () => void) => {
    fn();
    close();
  };

  return (
    <>
      {view === "files" && (
        <div className={`fab-dock${fabOpen ? " is-open" : ""}`}>
          <button
            type="button"
            className="fab-scrim"
            aria-label="Close actions"
            tabIndex={fabOpen ? 0 : -1}
            onClick={close}
          />
          <div className="fab-actions" role="menu" aria-hidden={!fabOpen}>
            <button
              type="button"
              role="menuitem"
              className="fab-action"
              disabled={busy}
              tabIndex={fabOpen ? 0 : -1}
              onClick={() => run(onUpload)}
            >
              <span className="fab-action-label">Upload</span>
              <span className="fab-action-icon">
                <IconUpload size={18} />
              </span>
            </button>
            <button
              type="button"
              role="menuitem"
              className="fab-action"
              disabled={busy}
              tabIndex={fabOpen ? 0 : -1}
              onClick={() => run(onNewFolder)}
            >
              <span className="fab-action-label">New Folder</span>
              <span className="fab-action-icon">
                <IconFolderPlus size={18} />
              </span>
            </button>
            <button
              type="button"
              role="menuitem"
              className="fab-action"
              disabled={busy}
              tabIndex={fabOpen ? 0 : -1}
              onClick={() => run(onNewFile)}
            >
              <span className="fab-action-label">New File</span>
              <span className="fab-action-icon">
                <IconFilePlus size={18} />
              </span>
            </button>
          </div>
          <button
            type="button"
            className="fab"
            aria-label={fabOpen ? "Close actions" : "Create or upload"}
            aria-expanded={fabOpen}
            onClick={() => setFabOpen((v) => !v)}
          >
            <IconPlus size={24} />
          </button>
        </div>
      )}

      <nav className="bottom-nav" aria-label="主导航">
        <button
          type="button"
          className={`bottom-nav-item${view === "overview" ? " is-active" : ""}`}
          aria-current={view === "overview" ? "page" : undefined}
          onClick={onOpenOverview}
        >
          <span className="bn-ico">
            <IconGrid size={20} />
          </span>
          <span>Overview</span>
        </button>
        <button
          type="button"
          className={`bottom-nav-item${view === "files" ? " is-active" : ""}`}
          aria-current={view === "files" ? "page" : undefined}
          onClick={onOpenFiles}
        >
          <span className="bn-ico">
            <IconDrive size={20} />
          </span>
          <span>Files</span>
        </button>
      </nav>
    </>
  );
}
