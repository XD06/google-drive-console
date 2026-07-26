import { useEffect, useRef, useState } from "react";
import { IconClose } from "../lib/icons";
import {
  createApiKey,
  listApiKeys,
  revokeApiKey,
  type ApiKeyItem,
  type ApiKeyScope,
} from "../lib/api";

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
          <ApiKeysSection open={open} />
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

/** API key management: list, create (token shown once), revoke. Session-only. */
function ApiKeysSection({ open }: { open: boolean }) {
  const [keys, setKeys] = useState<ApiKeyItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [creating, setCreating] = useState(false);
  const [formOpen, setFormOpen] = useState(false);
  const [name, setName] = useState("");
  const [scope, setScope] = useState<ApiKeyScope>("read");
  const [newToken, setNewToken] = useState("");
  const [copied, setCopied] = useState(false);
  const loadedRef = useRef(false);

  useEffect(() => {
    if (!open) {
      loadedRef.current = false;
      return;
    }
    if (loadedRef.current) return;
    loadedRef.current = true;
    setLoading(true);
    setError("");
    listApiKeys()
      .then(setKeys)
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load keys"))
      .finally(() => setLoading(false));
  }, [open]);

  const submit = async () => {
    const trimmed = name.trim();
    if (!trimmed || creating) return;
    setCreating(true);
    setError("");
    try {
      const created = await createApiKey(trimmed, scope);
      setNewToken(created.token);
      setCopied(false);
      const { token: _token, ...meta } = created;
      setKeys((prev) => [...prev, meta]);
      setName("");
      setFormOpen(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to create key");
    } finally {
      setCreating(false);
    }
  };

  const revoke = async (id: string) => {
    setError("");
    try {
      await revokeApiKey(id);
      setKeys((prev) => prev.map((k) => (k.id === id ? { ...k, revoked: true } : k)));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to revoke key");
    }
  };

  const copyToken = async () => {
    try {
      await navigator.clipboard.writeText(newToken);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      /* clipboard unavailable; token stays visible for manual copy */
    }
  };

  const active = keys.filter((k) => !k.revoked);
  const revoked = keys.filter((k) => k.revoked);

  return (
    <section className="settings-group" aria-labelledby="set-apikeys">
      <h3 id="set-apikeys">API Keys</h3>
      <p className="apikey-desc">
        Programmatic access for AI agents and scripts via <code>/api/v1</code>.
      </p>

      {newToken && (
        <div className="apikey-token-box" role="status">
          <div className="apikey-token-warn">Copy this key now — it won't be shown again.</div>
          <div className="apikey-token-row">
            <code className="apikey-token">{newToken}</code>
            <button type="button" className="btn btn-primary btn-sm apikey-copy" onClick={copyToken}>
              {copied ? "Copied ✓" : "Copy"}
            </button>
          </div>
          <button type="button" className="apikey-token-dismiss" onClick={() => setNewToken("")}>
            Done, I saved it
          </button>
        </div>
      )}

      {loading && <div className="apikey-empty">Loading…</div>}
      {!loading && keys.length === 0 && !error && (
        <div className="apikey-empty">No keys yet. Create one to let an agent use the API.</div>
      )}
      {error && <div className="apikey-error">{error}</div>}

      {active.map((k) => (
        <div key={k.id} className="apikey-row">
          <span className="apikey-row-main">
            <strong>{k.name}</strong>
            <span className="apikey-hint">
              <code>{k.hint}</code>
              <span className={`apikey-scope is-${k.scope}`}>
                {k.scope === "readwrite" ? "read / write" : "read-only"}
              </span>
            </span>
          </span>
          <button
            type="button"
            className="btn btn-ghost btn-sm apikey-revoke"
            onClick={() => revoke(k.id)}
          >
            Revoke
          </button>
        </div>
      ))}

      {revoked.length > 0 && (
        <div className="apikey-revoked-note">
          {revoked.length} revoked key{revoked.length > 1 ? "s" : ""}
        </div>
      )}

      {formOpen ? (
        <div className="apikey-form">
          <input
            type="text"
            className="apikey-input"
            placeholder="Key name (e.g. research-bot)"
            value={name}
            maxLength={64}
            autoFocus
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") void submit();
              if (e.key === "Escape") setFormOpen(false);
            }}
          />
          <div className="apikey-form-foot">
            <div className="apikey-scope-group" role="radiogroup" aria-label="Key scope">
              {(
                [
                  ["read", "Read-only"],
                  ["readwrite", "Read & write"],
                ] as [ApiKeyScope, string][]
              ).map(([value, label]) => (
                <button
                  key={value}
                  type="button"
                  role="radio"
                  aria-checked={scope === value}
                  className={`apikey-scope-btn${scope === value ? " is-active" : ""}`}
                  onClick={() => setScope(value)}
                >
                  {label}
                </button>
              ))}
            </div>
            <div className="apikey-form-actions">
              <button type="button" className="btn btn-ghost btn-sm" onClick={() => setFormOpen(false)}>
                Cancel
              </button>
              <button
                type="button"
                className="btn btn-primary btn-sm apikey-create"
                disabled={!name.trim() || creating}
                onClick={() => void submit()}
              >
                {creating ? "Creating…" : "Create key"}
              </button>
            </div>
          </div>
        </div>
      ) : (
        <button type="button" className="apikey-new" onClick={() => setFormOpen(true)}>
          + New key
        </button>
      )}
    </section>
  );
}
