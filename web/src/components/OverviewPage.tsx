import { formatBytes, type OverviewResponse } from "../lib/api";
import { type FileKind } from "../lib/fileKind";
import { iconBoxClass, KindIcon } from "../lib/FileTypeIcon";
import { useUploadJobCount } from "../lib/uploadSession";

export type UploadHistoryItem = { name: string; status: string; at: string; error?: string };

export type TypeCount = { kind: FileKind; count: number; label: string };

export type OverviewPageProps = {
  overview: OverviewResponse | null;
  overviewLoading: boolean;
  overviewError: string | null;
  onRetry: () => void;
  email: string;
  storageUsage: number;
  storageLimit: number;
  storagePct: number;
  uploadHistory: UploadHistoryItem[];
  histOk: number;
  histFail: number;
  /** Number of items in the current folder listing (drives "Types in view"). */
  itemCount: number;
  typeCounts: TypeCount[];
  typeTotal: number;
};

/** Overview badge — subscribes to job count only (stable during byte progress). */
function SessionUploadCount() {
  const n = useUploadJobCount();
  return <strong>{n}</strong>;
}

/**
 * Overview page — a self-contained view. All state stays in App and is handed in
 * as props, so App is a thin shell that swaps pages by `view`. Adding a third
 * page means adding another component like this plus one `view` branch.
 */
export function OverviewPage({
  overview,
  overviewLoading,
  overviewError,
  onRetry,
  email,
  storageUsage,
  storageLimit,
  storagePct,
  uploadHistory,
  histOk,
  histFail,
  itemCount,
  typeCounts,
  typeTotal,
}: OverviewPageProps) {
  return (
    <div className="overview-page">
      {overviewError && (
        <div className="ov-banner ov-banner-error" role="alert">
          <div>
            <strong>Couldn&apos;t load Drive storage</strong>
            <p>{overviewError}</p>
          </div>
          <button type="button" className="btn btn-sm" onClick={() => onRetry()}>
            Retry
          </button>
        </div>
      )}

      <section className="ov-hero" aria-label="Storage">
        <div className="ov-hero-top">
          <div className="ov-hero-identity">
            <div className="ov-hero-avatar" aria-hidden="true">
              {(overview?.user?.displayName || overview?.user?.email || email || "?")
                .slice(0, 1)
                .toUpperCase()}
            </div>
            <div className="ov-hero-meta">
              <h2>Google Drive storage</h2>
              <p className="ov-muted">
                {overviewLoading && !overview
                  ? "Loading quota…"
                  : overview?.user?.email || email || "Signed in"}
                {overview?.user?.displayName ? ` · ${overview.user.displayName}` : ""}
              </p>
            </div>
          </div>
          <div className="ov-hero-kpis">
            <div className="ov-kpi">
              <span className="ov-kpi-label">Used</span>
              <strong className="ov-kpi-value">
                {overview ? formatBytes(storageUsage) : "—"}
              </strong>
            </div>
            <div className="ov-kpi">
              <span className="ov-kpi-label">Limit</span>
              <strong className="ov-kpi-value">
                {overview && storageLimit > 0 ? formatBytes(storageLimit) : "—"}
              </strong>
            </div>
            <div className="ov-kpi">
              <span className="ov-kpi-label">Free</span>
              <strong className="ov-kpi-value">
                {overview && storageLimit > 0
                  ? formatBytes(Math.max(0, storageLimit - storageUsage))
                  : "—"}
              </strong>
            </div>
          </div>
        </div>

        <div className="ov-hero-bar-wrap">
          <div
            className={`ov-bar ov-bar-lg${storagePct >= 90 ? " is-critical" : storagePct >= 75 ? " is-warn" : ""}`}
            role="progressbar"
            aria-valuenow={storagePct}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label="Storage used"
          >
            <i style={{ width: overview ? `${Math.max(storagePct, storagePct > 0 ? 1.5 : 0)}%` : "0%" }} />
          </div>
          <div className="ov-hero-bar-meta">
            <span>
              {overview ? `${storagePct}% full` : overviewLoading ? "…" : "No data"}
            </span>
            <span className="ov-muted">
              Drive files {overview ? formatBytes(overview.storage.usageInDrive) : "—"}
            </span>
          </div>
        </div>
      </section>

      <div className="overview-grid">
        <section className="ov-card" aria-label="Uploads">
          <div className="ov-card-head">
            <div>
              <h3>Uploads</h3>
              <p className="ov-card-desc">Local console history (this browser)</p>
            </div>
            <span className="ov-pill">{uploadHistory.length} total</span>
          </div>
          <div className="ov-stat-row">
            <div className="ov-stat is-ok">
              <strong>{histOk}</strong>
              <span>Success</span>
            </div>
            <div className="ov-stat is-bad">
              <strong>{histFail}</strong>
              <span>Failed</span>
            </div>
            <div className="ov-stat">
              <SessionUploadCount />
              <span>Session</span>
            </div>
          </div>
          <ul className="ov-hist">
            {uploadHistory.slice(0, 10).length === 0 && (
              <li className="ov-empty">
                <span>No uploads yet</span>
                <span className="ov-muted">Upload from Files to see history here</span>
              </li>
            )}
            {uploadHistory.slice(0, 10).map((h, i) => (
              <li key={`${h.at}-${i}`} className={h.status === "failed" || h.error ? "is-err" : "is-ok"}>
                <span className="ov-hist-dot" aria-hidden="true" />
                <span className="ov-hist-name" title={h.name}>{h.name}</span>
                <span className="ov-hist-meta">{h.status}</span>
              </li>
            ))}
          </ul>
        </section>

        <section className="ov-card" aria-label="File types">
          <div className="ov-card-head">
            <div>
              <h3>Types in view</h3>
              <p className="ov-card-desc">
                {itemCount} item{itemCount === 1 ? "" : "s"} in current folder listing
              </p>
            </div>
            <span className="ov-pill">{typeCounts.length} kinds</span>
          </div>
          {itemCount === 0 ? (
            <div className="ov-empty">
              <span>No files loaded</span>
              <span className="ov-muted">Open Files to browse a folder first</span>
            </div>
          ) : (
            <ul className="ov-types">
              {typeCounts.map((t) => {
                const pct = Math.max(4, (t.count / typeTotal) * 100);
                return (
                  <li key={t.kind} className="ov-type-row">
                    <span className={iconBoxClass(t.kind)} aria-hidden="true">
                      <KindIcon kind={t.kind} />
                    </span>
                    <div className="ov-type-copy">
                      <span className="ov-type-label">{t.label}</span>
                      <span className={`ov-type-bar kind-bar-${t.kind}`}>
                        <i style={{ width: `${pct}%` }} />
                      </span>
                    </div>
                    <span className="ov-type-count">{t.count}</span>
                  </li>
                );
              })}
            </ul>
          )}
        </section>

        <section className="ov-card ov-card-tips" aria-label="Tips">
          <div className="ov-card-head">
            <div>
              <h3>Quick tips</h3>
              <p className="ov-card-desc">How this console works</p>
            </div>
          </div>
          <ul className="ov-tips">
            <li className="tip-desktop">Double-click a folder to open it; double-click text/code to edit.</li>
            <li className="tip-mobile">Tap a folder to open it; tap text/code to edit. Long-press to select.</li>
            <li className="tip-desktop">Drag files onto the Files view to upload into the current folder.</li>
            <li className="tip-mobile">Tap the + button to upload files or create folders.</li>
            <li>Type stats cover the current listing only — not the whole Drive.</li>
            <li>Use localhost:5174 so the session cookie matches the API host.</li>
          </ul>
        </section>
      </div>
    </div>
  );
}
