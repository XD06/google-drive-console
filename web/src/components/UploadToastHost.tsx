import { formatBytes } from "../lib/api";
import { IconClose, IconStop, IconUpload } from "../lib/icons";
import {
  cancelUploadJob,
  dismissUploadJob,
  useUploadJobs,
  type UploadJob,
} from "../lib/uploadSession";

export type UploadJobView = UploadJob;

function jobPhase(job: UploadJobView) {
  if (job.status === "failed" || !!job.error) return "err" as const;
  if (job.status === "completed" || job.status === "done") return "done" as const;
  if (job.status === "cancelled") return "cancelled" as const;
  if (job.status === "cancelling" || job.cancelling) return "cancelling" as const;
  if (job.status === "flushing") return "flushing" as const;
  return "active" as const;
}

/** Subscribes to the external upload store — progress ticks do not re-render App. */
export function UploadToastHost() {
  const jobs = useUploadJobs();
  if (jobs.length === 0) return null;

  const onCancel = (job: UploadJobView) => {
    void cancelUploadJob(job);
  };
  const onDismiss = (id: string) => {
    dismissUploadJob(id);
  };

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
        const canCancel = phase === "active" || phase === "flushing";
        const canDismiss = phase === "done" || phase === "err" || phase === "cancelled";
        const meta =
          phase === "done"
            ? "Uploaded — file is in this folder"
            : phase === "err"
              ? job.error || "Upload failed"
              : phase === "cancelled"
                ? "Cancelled"
                : phase === "cancelling"
                  ? "Cancelling…"
                  : phase === "flushing"
                    ? job.total > 0
                      ? `${formatBytes(job.received)} / ${formatBytes(job.total)} · sending to Drive…`
                      : "Sending to Drive…"
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
