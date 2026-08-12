import { IconClose } from "../lib/icons";

export type ShortcutHelpProps = {
  open: boolean;
  onClose: () => void;
};

const SHORTCUTS: { keys: string; desc: string }[] = [
  { keys: "↑ / ↓", desc: "Move selection up / down" },
  { keys: "Enter", desc: "Open selected file or folder" },
  { keys: "Delete", desc: "Move selected to trash" },
  { keys: "Ctrl/Cmd + A", desc: "Select all files in view" },
  { keys: "Ctrl/Cmd + C", desc: "Copy selected file name" },
  { keys: "Escape", desc: "Close dialog / clear search" },
  { keys: "?", desc: "Toggle this help overlay" },
  { keys: "Drag file → folder", desc: "Move file into target folder" },
  { keys: "Drag file → page", desc: "Upload files from desktop" },
];

export function ShortcutHelp({ open, onClose }: ShortcutHelpProps) {
  if (!open) return null;
  return (
    <>
      <div className="shortcut-overlay is-on" onClick={onClose} />
      <div className="shortcut-sheet is-on" role="dialog" aria-modal="true" aria-label="Keyboard shortcuts">
        <div className="shortcut-head">
          <h3>Keyboard shortcuts</h3>
          <button type="button" className="btn btn-ghost btn-sm btn-icon" aria-label="Close" onClick={onClose}>
            <IconClose size={16} />
          </button>
        </div>
        <table className="shortcut-table">
          <tbody>
            {SHORTCUTS.map((s) => (
              <tr key={s.keys}>
                <td><kbd>{s.keys}</kbd></td>
                <td>{s.desc}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
