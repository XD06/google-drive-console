import { formatBytes } from "../lib/api";
import { IconClose, IconStop, IconUpload } from "../lib/icons";

export type UploadJobView = {
  id: string;
  name: string;
  received: number;
  total: number;
  status: string;
  error?: string;
  cancelling?: boolean;
};

export type UploadToastHostProps = {
  jobs: UploadJobView[];
  onCancel: (job: UploadJobView) => void;
  onDismiss: (id: string) => void;
};

function jobPhase(job: UploadJobView) {
  if (job.status === "failed" || !!job.error) return "err" as const;
  if (job.status === "completed" || job.status === "done") return "done" as const;
  if (job.status === "cancelled") return "cancelled" as const;
  if (job.status === "cancelling" || job.cancelling) return "cancelling" as const;
  return "active" as const;
}

export function UploadToastHost({ jobs, onCancel, onDismiss }: UploadToastHostProps) {
  if (jobs.length === 0) return null;

  return (
    <div className="upload-toast-host" aria-live="polite" aria-label="Upload progress">
      {jobs.map((job) => {
        const phase = jobPhase(job);
        const rawPct =
          job.total > 0 ? Math.min(100, (job.received / job.total) * 100) : phase === "done" ? 100 : 0;
        const pctLabel =
          rawPct >= 99.5 && phase === "active"
            ? "99%"
            : rawPct < 10
              ? `${rawPct.toFixed(1)}%`
              : `${Math.round(rawPct)}%`;
        const canCancel = phase === "active";
        const canDismiss = phase === "done" || phase === "err" || phase === "cancelled";
        const meta =
          phase === "done"
            ? "Uploaded"
            : phase === "err"
              ? job.error || "Upload failed"
              : phase === "cancelled"
                ? "Cancelled"
                : phase === "cancelling"
                  ? "Cancelling…"
                  : job.total > 0
                    ? `${formatBytes(job.received)} / ${formatBytes(job.total)} · ${pctLabel}`
                    : "Starting…";

        return (
          <div
            key={job.id}
            className={`upload-toast is-${phase}`}
            role="status"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={Math.round(rawPct)}
          >
            <div className="upload-toast-top">
              <span className="upload-toast-icon" aria-hidden="true">
                <IconUpload size={14} />
              </span>
              <div className="upload-toast-body">
                <span className="upload-toast-name" title={job.name}>
                  {job.name}
                </span>
                <span className="upload-toast-meta">{meta}</span>
              </div>
              {canCancel && (
                <button
                  type="button"
                  className="btn btn-ghost btn-sm btn-icon upload-toast-action"
                  title="Cancel upload"
                  aria-label={`Cancel ${job.name}`}
                  onClick={() => onCancel(job)}
                >
                  <IconStop size={14} />
                </button>
              )}
              {canDismiss && (
                <button
                  type="button"
                  className="btn btn-ghost btn-sm btn-icon upload-toast-action"
                  title="Dismiss"
                  aria-label={`Dismiss ${job.name}`}
                  onClick={() => onDismiss(job.id)}
                >
                  <IconClose size={14} />
                </button>
              )}
            </div>
            <div className="upload-toast-bar" aria-hidden="true">
              <i
                style={{
                  width: `${phase === "err" || phase === "cancelled" ? 100 : rawPct}%`,
                }}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}
