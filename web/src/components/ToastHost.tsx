import { IconAlert, IconCheck, IconClose } from "../lib/icons";

export type ToastView = {
  id: string;
  title: string;
  msg: string;
  isErr: boolean;
  phase: "enter" | "in" | "out";
};

export type ToastHostProps = {
  toasts: ToastView[];
  onDismiss: (id: string) => void;
};

export function ToastHost({ toasts, onDismiss }: ToastHostProps) {
  return (
    <div className="toast-host" id="toast-host" aria-live="polite" aria-relevant="additions">
      {toasts.map((t) => (
        <div
          key={t.id}
          className={`toast${t.isErr ? " is-err" : ""}${
            t.phase === "in" ? " is-in" : t.phase === "out" ? " is-out" : ""
          }`}
          role={t.isErr ? "alert" : "status"}
        >
          <span className="toast-icon" aria-hidden="true">
            {t.isErr ? <IconAlert size={14} /> : <IconCheck size={14} />}
          </span>
          <div className="toast-body">
            <span className="toast-title">{t.title}</span>
            <span className="toast-msg">{t.msg}</span>
          </div>
          <button
            type="button"
            className="toast-close"
            aria-label="Dismiss"
            onClick={() => onDismiss(t.id)}
          >
            <IconClose size={14} />
          </button>
        </div>
      ))}
    </div>
  );
}
