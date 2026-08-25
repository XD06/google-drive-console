import {
  IconClock,
  IconDownload,
  IconDrive,
  IconGrid,
  IconHelp,
  IconLogout,
  IconMoon,
  IconRefresh,
  IconSettings,
  IconSun,
} from "../lib/icons";

export type RecentFolder = { id: string; name: string };

export type SidebarProps = {
  view: "files" | "overview" | "downloads";
  email: string;
  avatar: string;
  busy: boolean;
  recentFolders: RecentFolder[];
  onOpenFiles: () => void;
  onOpenOverview: () => void;
  onOpenDownloads: () => void;
  onOpenRecent: (id: string, name: string) => void;
  onLogout: () => void;
  // Mobile-only utilities (shown in the drawer on phones; hidden on desktop
  // where the topbar already hosts these controls).
  isDark: boolean;
  connected: boolean;
  refreshBusy: boolean;
  onToggleTheme: () => void;
  onRefresh: () => void;
  onOpenSettings: () => void;
  onOpenShortcuts: () => void;
};

export function Sidebar({
  view,
  email,
  avatar,
  busy,
  recentFolders,
  onOpenFiles,
  onOpenOverview,
  onOpenDownloads,
  onOpenRecent,
  onLogout,
  isDark,
  connected,
  refreshBusy,
  onToggleTheme,
  onRefresh,
  onOpenSettings,
  onOpenShortcuts,
}: SidebarProps) {
  return (
    <aside className="sidebar" aria-label="导航">
      <div className="side-brand">
        <span className="side-brand-mark" aria-hidden="true">
          <IconDrive size={14} strokeWidth={2} />
        </span>
        Drive Backup
      </div>
      <div className="nav-label" id="nav-library-label">
        Library
      </div>
      <nav aria-labelledby="nav-library-label">
        <button
          type="button"
          className={`nav-item${view === "files" ? " is-active" : ""}`}
          aria-current={view === "files" ? "page" : undefined}
          onClick={onOpenFiles}
        >
          <IconDrive size={16} />
          My Drive
        </button>
        <button
          type="button"
          className={`nav-item${view === "overview" ? " is-active" : ""}`}
          onClick={onOpenOverview}
        >
          <IconGrid size={16} />
          Overview
        </button>
        <button
          type="button"
          className={`nav-item${view === "downloads" ? " is-active" : ""}`}
          onClick={onOpenDownloads}
        >
          <IconDownload size={16} />
          Downloads
        </button>
      </nav>
      {recentFolders.length > 0 && (
        <div className="nav-section">
          <div className="nav-label">Recent</div>
          <nav>
            {recentFolders.map((f) => (
              <button
                key={f.id}
                type="button"
                className="recent-folder-item"
                onClick={() => onOpenRecent(f.id, f.name)}
                title={f.name}
              >
                <IconClock size={14} />
                <span className="recent-name">{f.name}</span>
              </button>
            ))}
          </nav>
        </div>
      )}
      <div className="nav-section side-utils">
        <div className="nav-label">Settings</div>
        <nav>
          <button type="button" className="nav-item" onClick={onToggleTheme}>
            {isDark ? <IconSun size={16} /> : <IconMoon size={16} />}
            {isDark ? "Light mode" : "Dark mode"}
          </button>
          <button
            type="button"
            className="nav-item"
            disabled={refreshBusy}
            onClick={onRefresh}
          >
            <IconRefresh size={16} />
            Refresh
          </button>
          <button type="button" className="nav-item" onClick={onOpenSettings}>
            <IconSettings size={16} />
            Settings
          </button>
          <button type="button" className="nav-item" onClick={onOpenShortcuts}>
            <IconHelp size={16} />
            Shortcuts
          </button>
        </nav>
      </div>
      <div className="side-foot">
        <div className="account-row">
          <span className="account-avatar" aria-hidden="true">
            {avatar}
          </span>
          <div className="account-meta">
            <span className="account-line" title={email}>
              {email}
            </span>
            {connected && (
              <span className="account-status">
                <span className="dot" aria-hidden="true" />
                Connected
              </span>
            )}
          </div>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-icon account-logout"
            title="Disconnect"
            aria-label="Disconnect"
            disabled={busy}
            onClick={onLogout}
          >
            <IconLogout size={15} />
          </button>
        </div>
      </div>
    </aside>
  );
}
