import { useEffect, useState } from "react";
import { shareFile, unshareFile, type FileItem, type ShareInfo } from "../lib/api";
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

  useEffect(() => {
    if (!item) { setInfo(null); setError(null); return; }
    setLoading(true);
    setError(null);
    shareFile(item.id)
      .then((data) => { setInfo(data); onShared?.(); })
      .catch((e) => setError(e instanceof Error ? e.message : "Share failed"))
      .finally(() => setLoading(false));
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
            </>
          )}
        </div>
      </div>
    </>
  );
}
