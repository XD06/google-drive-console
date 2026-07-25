import { fileDownloadUrl } from "../lib/api";
import { IconClose, IconDownload } from "../lib/icons";

export type ImagePreviewState = {
  id: string;
  name: string;
  url: string | null;
  loading: boolean;
  error: string | null;
};

export type ImageLightboxProps = {
  preview: ImagePreviewState | null;
  onClose: () => void;
};

export function ImageLightbox({ preview, onClose }: ImageLightboxProps) {
  if (!preview) return null;
  return (
    <>
      <div className="lightbox-overlay is-on" onClick={onClose} />
      <div
        className="lightbox-sheet is-on"
        role="dialog"
        aria-modal="true"
        aria-label={preview.name || "Image preview"}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="lightbox-head">
          <h3 className="lightbox-title" title={preview.name}>
            {preview.name || "Image"}
          </h3>
          <div className="lightbox-actions">
            <a
              className="btn btn-ghost btn-sm btn-icon"
              href={fileDownloadUrl(preview.id)}
              download={preview.name}
              title="Download"
              aria-label="Download"
              onClick={(e) => e.stopPropagation()}
            >
              <IconDownload size={16} />
            </a>
            <button
              type="button"
              className="btn btn-ghost btn-sm btn-icon"
              aria-label="Close"
              onClick={onClose}
            >
              <IconClose size={16} />
            </button>
          </div>
        </div>
        <div className="lightbox-body">
          {preview.loading && <div className="lightbox-status">Loading image…</div>}
          {preview.error && (
            <div className="lightbox-status is-err">{preview.error}</div>
          )}
          {preview.url && !preview.loading && (
            <img className="lightbox-img" src={preview.url} alt={preview.name} />
          )}
        </div>
      </div>
    </>
  );
}
