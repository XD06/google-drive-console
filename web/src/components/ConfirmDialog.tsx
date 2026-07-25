import { IconAlert } from "../lib/icons";

export type ConfirmDialogProps = {
  open: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  danger?: boolean;
  onCancel: () => void;
  onConfirm: () => void;
};

export function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel,
  danger,
  onCancel,
  onConfirm,
}: ConfirmDialogProps) {
  return (
    <>
      <div
        className={`confirm-overlay${open ? " is-on" : ""}`}
        hidden={!open}
        onClick={onCancel}
      />
      <div
        className={`confirm-sheet${open ? " is-on" : ""}`}
        role="dialog"
        aria-modal="true"
        aria-labelledby="confirm-title"
        hidden={!open}
      >
        {open && (
          <>
            <div className="confirm-head">
              <span className={`confirm-icon${danger ? " is-danger" : ""}`} aria-hidden="true">
                <IconAlert size={18} />
              </span>
              <h3 id="confirm-title">{title}</h3>
            </div>
            <p className="confirm-msg">{message}</p>
            <div className="confirm-actions">
              <button type="button" className="btn" onClick={onCancel}>
                Cancel
              </button>
              <button
                type="button"
                className={`btn ${danger ? "btn-danger" : "btn-primary"}`}
                autoFocus
                onClick={onConfirm}
              >
                {confirmLabel || "Confirm"}
              </button>
            </div>
          </>
        )}
      </div>
    </>
  );
}
