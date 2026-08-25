import { useEffect, useMemo, useState } from "react";
import {
  ACTIVE_STATUSES,
  STATUS_LABELS,
  cancelDownload,
  clearFinishedDownloads,
  createDownload,
  deleteDownload,
  formatDuration,
  formatRelativeTime,
  getThumbClass,
  getTypeIcon,
  loadDownloads,
  retryUpload,
  type DownloadJob,
  useDownloads,
} from "../lib/downloadStore";
import { formatBytes } from "../lib/api";
import {
  IconChevronLeft,
  IconDownload,
  IconHistory,
  IconOpen,
  IconRefresh,
  IconRename,
  IconStop,
  IconTrash,
} from "../lib/icons";

type FilterKey = "all" | "completed" | "failed";
type SubView = "main" | "history";

const FILTERS: { key: FilterKey; label: string }[] = [
  { key: "all", label: "All" },
  { key: "completed", label: "Completed" },
  { key: "failed", label: "Failed" },
];

const SAVE_FOLDERS = [
  { value: "", label: "My Drive (root)" },
  { value: "Videos", label: "📁 Downloads / Videos" },
  { value: "Music", label: "📁 Downloads / Music" },
  { value: "Books", label: "📁 Downloads / Books" },
  { value: "Photos", label: "📁 Downloads / Photos" },
];

const SUPPORTED_SITES = [
  { label: "YouTube", cls: "site-yt" },
  { label: "B站", cls: "site-bili" },
  { label: "抖音", cls: "site-dy" },
  { label: "TikTok", cls: "site-tt" },
  { label: "小红书", cls: "site-xhs" },
  { label: "直链", cls: "site-direct" },
  { label: "1000+", cls: "site-more" },
];

export type DownloadPageProps = {
  onOpenInDrive?: (fileId: string) => void;
  onRename?: (job: DownloadJob) => void;
};

export function DownloadPage({ onOpenInDrive, onRename }: DownloadPageProps) {
  const { jobs, loading, error } = useDownloads();
  const [subView, setSubView] = useState<SubView>("main");
  const [filter, setFilter] = useState<FilterKey>("all");
  const [heroUrl, setHeroUrl] = useState("");
  const [heroName, setHeroName] = useState("");
  const [heroFolder, setHeroFolder] = useState("");
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    void loadDownloads();
  }, []);

  const activeJobs = useMemo(
    () => jobs.filter((j) => ACTIVE_STATUSES.includes(j.status)),
    [jobs],
  );

  const filtered = useMemo(() => {
    if (filter === "completed") return jobs.filter((j) => j.status === "completed");
    if (filter === "failed") return jobs.filter((j) => j.status === "failed" || j.status === "cancelled");
    return jobs;
  }, [jobs, filter]);

  const counts = useMemo(
    () => ({
      all: jobs.length,
      completed: jobs.filter((j) => j.status === "completed").length,
      failed: jobs.filter((j) => j.status === "failed" || j.status === "cancelled").length,
    }),
    [jobs],
  );

  async function handleHeroDownload() {
    const url = heroUrl.trim();
    if (!url) return;
    setCreating(true);
    const job = await createDownload(url, heroFolder || undefined);
    setCreating(false);
    if (job) {
      setHeroUrl("");
      setHeroName("");
    }
  }

  // ---- Main view: centered input + active downloads ----
  if (subView === "main") {
    return (
      <div className="dl-view-main">
        <div className="dl-hero-zone">
          <div className="dl-hero-icon" aria-hidden="true">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.5}>
              <path d="M12 3v12" />
              <path d="M7 10l5 5 5-5" />
              <path d="M5 21h14a2 2 0 0 0 2-2v-2" />
            </svg>
          </div>
          <h2 className="dl-hero-title">Download Anything</h2>
          <p className="dl-hero-subtitle">
            Paste a video link or direct file URL — supports YouTube, B站, 抖音, TikTok, 小红书 and 1000+ sites
          </p>

          <div className="dl-url-wrap">
            <div className="dl-url-inner">
              <svg className="dl-url-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2}>
                <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
                <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
              </svg>
              <input
                type="url"
                className="dl-url-input"
                placeholder="Paste link here..."
                autoFocus
                value={heroUrl}
                disabled={creating}
                onChange={(e) => setHeroUrl(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && heroUrl.trim()) void handleHeroDownload();
                }}
              />
              <button
                type="button"
                className="dl-url-go"
                disabled={creating || !heroUrl.trim()}
                onClick={() => void handleHeroDownload()}
              >
                <IconDownload size={18} />
                <span>Download</span>
              </button>
            </div>
            <div className="dl-url-opts">
              <div className="dl-url-opt">
                <label className="dl-url-opt-label">Save to</label>
                <select
                  className="dl-url-opt-select"
                  value={heroFolder}
                  disabled={creating}
                  onChange={(e) => setHeroFolder(e.target.value)}
                >
                  {SAVE_FOLDERS.map((f) => (
                    <option key={f.value} value={f.value}>{f.label}</option>
                  ))}
                </select>
              </div>
              <div className="dl-url-opt">
                <label className="dl-url-opt-label">Filename</label>
                <input
                  type="text"
                  className="dl-url-opt-input"
                  placeholder="Auto"
                  value={heroName}
                  disabled={creating}
                  onChange={(e) => setHeroName(e.target.value)}
                />
              </div>
            </div>
          </div>

          <div className="dl-sites">
            {SUPPORTED_SITES.map((s) => (
              <span key={s.label} className={`dl-site-badge ${s.cls}`}>
                {s.label}
              </span>
            ))}
          </div>
        </div>

        {/* Active downloads */}
        {activeJobs.length > 0 && (
          <div className="dl-active-section">
            <div className="dl-section-head">
              <h3 className="dl-section-title">Active Downloads</h3>
              <span className="dl-section-count">{activeJobs.length}</span>
            </div>
            <div className="dl-active-list">
              {activeJobs.map((job) => (
                <DownloadCard
                  key={job.id}
                  job={job}
                  large
                  onOpenInDrive={onOpenInDrive}
                  onRename={onRename}
                />
              ))}
            </div>
          </div>
        )}

        {/* Error toast area */}
        {error && (
          <div className="dl-error-banner">{error}</div>
        )}

        {/* History button — floating bottom-right */}
        <button
          type="button"
          className="dl-history-fab"
          title="Download history"
          onClick={() => setSubView("history")}
        >
          <IconHistory size={20} />
          {counts.all > 0 && (
            <span className="dl-fab-badge">{counts.all}</span>
          )}
        </button>
      </div>
    );
  }

  // ---- History view: record list ----
  return (
    <div className="dl-view-history">
      <div className="toolbar dl-toolbar" role="toolbar" aria-label="下载记录">
        <button
          type="button"
          className="btn btn-ghost btn-sm"
          onClick={() => setSubView("main")}
        >
          <IconChevronLeft size={16} />
          <span className="btn-label">Back</span>
        </button>
        <div className="dl-filters">
          {FILTERS.map((f) => (
            <button
              key={f.key}
              type="button"
              className={`type-chip${filter === f.key ? " is-active" : ""}`}
              onClick={() => setFilter(f.key)}
            >
              {f.label}
              <span className="chip-badge">{counts[f.key]}</span>
            </button>
          ))}
        </div>
        <div className="spacer" />
        <button
          type="button"
          className="btn btn-ghost btn-sm"
          disabled={loading}
          onClick={() => void loadDownloads()}
        >
          <IconRefresh size={15} />
          <span className="btn-label">Refresh</span>
        </button>
        {(counts.completed > 0 || counts.failed > 0) && (
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={() => clearFinishedDownloads()}
          >
            <IconTrash size={15} />
            <span className="btn-label">Clear</span>
          </button>
        )}
      </div>

      <div className="dl-content">
        {error && <div className="dl-error-banner">{error}</div>}
        {loading && jobs.length === 0 ? (
          <div className="dl-empty">
            <div className="dl-empty-icon">⟳</div>
            <div className="dl-empty-title">Loading downloads…</div>
          </div>
        ) : filtered.length === 0 ? (
          <div className="dl-empty">
            <div className="dl-empty-icon">∅</div>
            <div className="dl-empty-title">
              {filter === "all" ? "No downloads yet" : `No ${filter} downloads`}
            </div>
            <div className="dl-empty-desc">
              {filter === "all"
                ? "Use the input above to start a download"
                : "Try a different filter"}
            </div>
          </div>
        ) : (
          <div className="dl-list">
            {filtered.map((job) => (
              <DownloadCard
                key={job.id}
                job={job}
                onOpenInDrive={onOpenInDrive}
                onRename={onRename}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// ---- Download Card ---------------------------------------------------------

function DownloadCard({
  job,
  large,
  onOpenInDrive,
  onRename,
}: {
  job: DownloadJob;
  large?: boolean;
  onOpenInDrive?: (fileId: string) => void;
  onRename?: (job: DownloadJob) => void;
}) {
  const isActive = ACTIVE_STATUSES.includes(job.status);
  const fillClass =
    job.status === "uploading"
      ? "is-upload"
      : job.status === "completed"
        ? "is-done"
        : job.status === "failed" || job.status === "cancelled"
          ? "is-fail"
          : "";

  const thumbClass = getThumbClass(job.extractor, job.ext);
  const typeIcon = getTypeIcon(job.extractor);

  return (
    <div className={`dl-card${large ? " is-large" : ""}`}>
      <div className="dl-thumb">
        {job.thumbnail ? (
          <img
            src={job.thumbnail}
            alt=""
            onError={(e) => {
              const img = e.currentTarget;
              img.style.display = "none";
              const parent = img.parentElement;
              if (parent) {
                parent.innerHTML = `<div class="dl-thumb-fallback ${thumbClass}">${typeIcon}</div>`;
              }
            }}
          />
        ) : (
          <div
            className={`dl-thumb-fallback ${thumbClass}`}
            dangerouslySetInnerHTML={{ __html: typeIcon }}
          />
        )}
      </div>
      <div className="dl-body">
        <div className="dl-head">
          <span className="dl-title" title={job.title || job.url}>
            {job.title || job.url}
          </span>
          <span className={`dl-status s-${job.status}`}>
            <span className="dl-status-dot" />
            {STATUS_LABELS[job.status]}
          </span>
        </div>
        <div className="dl-meta">
          {job.extractor && <span className="dl-extractor">{job.extractor}</span>}
          {job.uploader && <span>{job.uploader}</span>}
          {job.duration ? <span>{formatDuration(job.duration)}</span> : null}
          <span>{formatRelativeTime(job.createdAt)}</span>
          {job.total ? <span>{formatBytes(job.total)}</span> : null}
        </div>
        {job.error && <div className="dl-error">{job.error}</div>}
        {job.status !== "failed" && job.status !== "cancelled" && (
          <div className="dl-progress-wrap">
            <div className="dl-progress">
              <div
                className={`dl-progress-fill ${fillClass}`}
                style={{ width: `${Math.max(2, job.progress)}%` }}
              />
            </div>
            <span className="dl-progress-pct">{Math.round(job.progress)}%</span>
            {job.speed && (
              <span className="dl-progress-speed">
                {job.speed}
                {job.eta ? ` · ETA ${job.eta}` : ""}
              </span>
            )}
          </div>
        )}
      </div>
      <div className="dl-actions">
        {isActive && (
          <button
            type="button"
            className="dl-action-btn is-danger"
            title="Cancel"
            aria-label="Cancel download"
            onClick={() => void cancelDownload(job.id)}
          >
            <IconStop size={15} />
          </button>
        )}
        {job.status === "completed" && job.driveFileId && (
          <>
            {onRename && (
              <button
                type="button"
                className="dl-action-btn"
                title="Rename"
                aria-label="Rename"
                onClick={() => onRename(job)}
              >
                <IconRename size={15} />
              </button>
            )}
            <button
              type="button"
              className="dl-action-btn"
              title="Open in Drive"
              aria-label="Open in Drive"
              onClick={() => onOpenInDrive?.(job.driveFileId!)}
            >
              <IconOpen size={15} />
            </button>
          </>
        )}
        {(job.status === "failed" || job.status === "cancelled") && (
          <>
            {job.fileName && (
              <button
                type="button"
                className="dl-action-btn"
                title="Retry upload only"
                aria-label="Retry upload"
                onClick={() => void retryUpload(job.id)}
              >
                <IconRefresh size={15} />
              </button>
            )}
            <button
              type="button"
              className="dl-action-btn"
              title="Re-download from URL"
              aria-label="Re-download"
              onClick={() => void createDownload(job.url, job.parentId || undefined)}
            >
              <IconDownload size={15} />
            </button>
          </>
        )}
        {(job.status === "completed" || job.status === "failed" || job.status === "cancelled") && (
          <button
            type="button"
            className="dl-action-btn is-danger"
            title="Delete record"
            aria-label="Delete record"
            onClick={() => void deleteDownload(job.id)}
          >
            <IconTrash size={15} />
          </button>
        )}
      </div>
    </div>
  );
}
