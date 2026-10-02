import { useEffect, useState } from "react";
import {
  shareFile,
  unshareFile,
  createDirectLink,
  listDirectLinks,
  revokeDirectLink,
  type FileItem,
  type ShareInfo,
  type DirectLink,
} from "../lib/api";
import { IconClose, IconCopy } from "../lib/icons";

export type ShareDialogProps = {
  item: FileItem | null;
  onClose: () => void;
  onShared?: () => void;
};

export function ShareDialog({ item, onClose, onShared }: ShareDialogProps) {
  const [info, setInfo] = useState<ShareInfo | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  // Direct link state (backend-proxied /d/{token} URL).
  const [link, setLink] = useState<DirectLink | null>(null);
  const [linkBusy, setLinkBusy] = useState(false);
  const [linkError, setLinkError] = useState<string | null>(null);
  const [linkCopied, setLinkCopied] = useState(false);

  useEffect(() => {
    if (!item) {
      setInfo(null);
      setError(null);
      setLink(null);
      setLinkError(null);
      return;
    }
    setLoading(true);
    setError(null);
    shareFile(item.id)
      .then((data) => { setInfo(data); onShared?.(); })
      .catch((e) => setError(e instanceof Error ? e.message : "Share failed"))
      .finally(() => setLoading(false));
    // Load any existing direct link for this file.
    setLinkBusy(true);
    setLinkError(null);
    listDirectLinks(item.id)
      .then((links) => setLink(links[0] ?? null))
      .catch((e) => setLinkError(e instanceof Error ? e.message : "Direct link lookup failed"))
      .finally(() => setLinkBusy(false));
  }, [item?.id]);

  function unshare() {
    if (!item || !info) return;
    setLoading(true);
    unshareFile(item.id, info.permission.id)
      .then(() => { setInfo(null); onClose(); })
      .catch(() => setError("Unshare failed"))
      .finally(() => setLoading(false));
  }

  function copyLink() {
    if (info?.shareUrl) {
      navigator.clipboard.writeText(info.shareUrl).then(() => {
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      });
    }
  }

  function makeDirectLink() {
    if (!item) return;
    setLinkBusy(true);
    setLinkError(null);
    createDirectLink(item.id)
      .then((l) => setLink(l))
      .catch((e) => setLinkError(e instanceof Error ? e.message : "Failed to create direct link"))
      .finally(() => setLinkBusy(false));
  }

  function removeDirectLink() {
    if (!link) return;
    setLinkBusy(true);
    setLinkError(null);
    revokeDirectLink(link.token)
      .then(() => setLink(null))
      .catch((e) => setLinkError(e instanceof Error ? e.message : "Failed to remove direct link"))
      .finally(() => setLinkBusy(false));
  }

  function copyDirectLink() {
    if (link?.url) {
      navigator.clipboard.writeText(link.url).then(() => {
        setLinkCopied(true);
        setTimeout(() => setLinkCopied(false), 2000);
      });
    }
  }

  if (!item) return null;

  return (
    <>
      <div className="share-overlay is-on" onClick={onClose} />
      <div className="share-sheet is-on" role="dialog" aria-modal="true" aria-label={`Share ${item.name}`}>
        <div className="share-head">
          <h3>Share “{item.name}”</h3>
          <button type="button" className="btn btn-ghost btn-sm btn-icon" aria-label="Close" onClick={onClose}>
            <IconClose size={16} />
          </button>
        </div>
        <div className="share-body">
          {loading && <div className="share-status">Loading…</div>}
          {error && <div className="share-status is-err">{error}</div>}
          {info && !loading && (
            <>
              <div className="share-link-row">
                <input
                  className="share-link-input"
                  readOnly
                  value={info.shareUrl || ""}
                  onClick={(e) => (e.target as HTMLInputElement).select()}
                />
                <button type="button" className="btn btn-sm" onClick={copyLink} title="Copy link">
                  <IconCopy size={14} />
                  {copied ? "Copied!" : "Copy"}
                </button>
              </div>
              <p className="share-meta">Anyone with the link can {info.permission.role === "reader" ? "view" : "edit"}</p>
              <button type="button" className="btn btn-ghost btn-sm settings-danger" onClick={unshare} disabled={loading}>
                Remove link
              </button>

              <div className="share-divider" role="separator" />

              <h4 className="share-subhead">Direct link</h4>
              <p className="share-meta">
                Streams the file through this server — works anywhere without a Google login.
                Supports video seeking (Range) and resumable downloads.
              </p>
              {linkError && <div className="share-status is-err">{linkError}</div>}
              {link ? (
                <>
                  <div className="share-link-row">
                    <input
                      className="share-link-input"
                      readOnly
                      value={link.url}
                      onClick={(e) => (e.target as HTMLInputElement).select()}
                    />
                    <button type="button" className="btn btn-sm" onClick={copyDirectLink} title="Copy direct link">
                      <IconCopy size={14} />
                      {linkCopied ? "Copied!" : "Copy"}
                    </button>
                  </div>
                  <div className="share-meta">
                    Append <code>?dl=1</code> to force a file download instead of inline preview.
                  </div>
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm settings-danger"
                    onClick={removeDirectLink}
                    disabled={linkBusy}
                  >
                    Remove direct link
                  </button>
                </>
              ) : (
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={makeDirectLink}
                  disabled={linkBusy}
                >
                  Create direct link
                </button>
              )}
            </>
          )}
        </div>
      </div>
    </>
  );
}
