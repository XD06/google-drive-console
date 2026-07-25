import { useEffect, useState } from "react";
import { formatBytes, listRevisions, restoreRevision, type FileItem, type Revision } from "../lib/api";
import { IconClose } from "../lib/icons";

export type RevisionsPanelProps = {
  item: FileItem | null;
  onClose: () => void;
};

export function RevisionsPanel({ item, onClose }: RevisionsPanelProps) {
  const [revs, setRevs] = useState<Revision[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [restoring, setRestoring] = useState<string | null>(null);

  useEffect(() => {
    if (!item) { setRevs([]); setError(null); return; }
    setLoading(true);
    setError(null);
    listRevisions(item.id)
      .then((data) => setRevs(data))
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load revisions"))
      .finally(() => setLoading(false));
  }, [item?.id]);

  async function restore(rev: Revision) {
    if (!item) return;
    const ok = confirm(`Restore version from ${new Date(rev.modifiedTime).toLocaleString()}?`);
    if (!ok) return;
    setRestoring(rev.id);
    try {
      await restoreRevision(item.id, rev.id);
      // Refresh list
      const data = await listRevisions(item.id);
      setRevs(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Restore failed");
    } finally {
      setRestoring(null);
    }
  }

  if (!item) return null;

  return (
    <>
      <div className="revisions-overlay is-on" onClick={onClose} />
      <div className="revisions-sheet is-on" role="dialog" aria-modal="true" aria-label={`Version history for ${item.name}`}>
        <div className="revisions-head">
          <h3>Version history · {item.name}</h3>
          <button type="button" className="btn btn-ghost btn-sm btn-icon" aria-label="Close" onClick={onClose}>
            <IconClose size={16} />
          </button>
        </div>
        <div className="revisions-body">
          {loading && <div className="revisions-status">Loading…</div>}
          {error && <div className="revisions-status is-err">{error}</div>}
          {!loading && !error && revs.length === 0 && (
            <div className="revisions-status">No version history available</div>
          )}
          {!loading && revs.length > 0 && (
            <ul className="revisions-list">
              {revs.map((rev, i) => (
                <li key={rev.id} className={i === 0 ? "is-current" : ""}>
                  <div className="rev-info">
                    <span className="rev-date">{new Date(rev.modifiedTime).toLocaleString()}</span>
                    {rev.lastModifyingUser && <span className="rev-user">{rev.lastModifyingUser}</span>}
                    {rev.size != null && <span className="rev-size">{formatBytes(rev.size)}</span>}
                    {i === 0 && <span className="rev-badge">Current</span>}
                  </div>
                  {i > 0 && (
                    <button
                      type="button"
                      className="btn btn-ghost btn-sm"
                      disabled={restoring === rev.id}
                      onClick={() => restore(rev)}
                    >
                      {restoring === rev.id ? "Restoring…" : "Restore"}
                    </button>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </>
  );
}
