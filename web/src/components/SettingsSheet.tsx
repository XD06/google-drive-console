import { IconClose } from "../lib/icons";

export type Prefs = {
  banner: boolean;
  uploadToast: boolean;
  compact: boolean;
  autoDismiss: boolean;
};

export type ThemeMode = "auto" | "light" | "dark";

export type SettingsSheetProps = {
  open: boolean;
  prefs: Prefs;
  email: string;
  busy: boolean;
  themeMode: ThemeMode;
  onClose: () => void;
  onPrefsChange: (next: Prefs) => void;
  onThemeChange: (mode: ThemeMode) => void;
  onLogout: () => void;
  onBannerEnabled?: () => void;
};

export function SettingsSheet({
  open,
  prefs,
  email,
  busy,
  themeMode,
  onClose,
  onPrefsChange,
  onThemeChange,
  onLogout,
  onBannerEnabled,
}: SettingsSheetProps) {
  const Toggle = ({ checked, onChange }: { checked: boolean; onChange: (v: boolean) => void }) => (
    <div className="toggle-switch">
      <input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      <span className="toggle-track" />
      <span className="toggle-knob" />
    </div>
  );

  return (
    <>
      <div
        className={`settings-overlay${open ? " is-on" : ""}`}
        id="settings-overlay"
        hidden={!open}
        onClick={onClose}
      />
      <div
        className={`settings-sheet${open ? " is-on" : ""}`}
        id="settings-sheet"
        role="dialog"
        aria-modal="true"
        aria-labelledby="settings-title"
        hidden={!open}
      >
        <div className="settings-head">
          <h2 id="settings-title">Settings</h2>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-icon"
            aria-label="Close settings"
            onClick={onClose}
          >
            <IconClose size={16} />
          </button>
        </div>
        <div className="settings-body">
          <section className="settings-group" aria-labelledby="set-appearance">
            <h3 id="set-appearance">Appearance</h3>
            <div className="settings-row">
              <span className="settings-row-text">
                <strong>Theme</strong>
                <span>Auto / Light / Dark</span>
              </span>
              <div className="theme-toggle-group">
                {(["auto", "light", "dark"] as ThemeMode[]).map((m) => (
                  <button
                    key={m}
                    type="button"
                    className={`theme-toggle-btn${themeMode === m ? " is-active" : ""}`}
                    onClick={() => onThemeChange(m)}
                  >
                    {m === "auto" ? "Auto" : m === "light" ? "Light" : "Dark"}
                  </button>
                ))}
              </div>
            </div>
            <div className="settings-row">
              <span className="settings-row-text">
                <strong>Compact density</strong>
                <span>Tighter table rows</span>
              </span>
              <Toggle checked={prefs.compact} onChange={(v) => onPrefsChange({ ...prefs, compact: v })} />
            </div>
          </section>
          <section className="settings-group" aria-labelledby="set-notif">
            <h3 id="set-notif">Notifications</h3>
            <div className="settings-row">
              <span className="settings-row-text">
                <strong>Banner alerts</strong>
                <span>Phone-style banners for upload events</span>
              </span>
              <Toggle
                checked={prefs.banner}
                onChange={(v) => {
                  onPrefsChange({ ...prefs, banner: v });
                  if (v) onBannerEnabled?.();
                }}
              />
            </div>
            <div className="settings-row">
              <span className="settings-row-text">
                <strong>Upload complete</strong>
                <span>Notify when an upload finishes</span>
              </span>
              <Toggle checked={prefs.uploadToast} onChange={(v) => onPrefsChange({ ...prefs, uploadToast: v })} />
            </div>
            <div className="settings-row">
              <span className="settings-row-text">
                <strong>Auto-dismiss toasts</strong>
                <span>Auto-close success toasts after a few seconds</span>
              </span>
              <Toggle checked={prefs.autoDismiss} onChange={(v) => onPrefsChange({ ...prefs, autoDismiss: v })} />
            </div>
          </section>
          <section className="settings-group" aria-labelledby="set-account">
            <h3 id="set-account">Account</h3>
            <div className="settings-static">
              <span className="settings-static-label">Signed in</span>
              <span className="settings-static-value">{email}</span>
            </div>
            <button
              type="button"
              className="btn settings-danger"
              disabled={busy}
              onClick={() => {
                onClose();
                onLogout();
              }}
            >
              Disconnect Google
            </button>
          </section>
          <div style={{ textAlign: "center", padding: "8px 0 4px", fontSize: 11, color: "var(--muted)" }}>
            Drive Backup Console v1.0
          </div>
        </div>
      </div>
    </>
  );
}
