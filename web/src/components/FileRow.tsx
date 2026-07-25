import { memo, useRef, useEffect, type DragEvent, type MouseEvent, type TouchEvent } from "react";
import { formatBytes, formatModified, type FileItem } from "../lib/api";
import { fileKind } from "../lib/fileKind";
import { FileTypeIcon, iconBoxClass } from "../lib/FileTypeIcon";
import { IconMore } from "../lib/icons";

// Touch devices don't reliably fire dblclick, so on coarse pointers a single tap
// opens the row (matching native file apps). Desktop keeps double-click to open.
const isCoarsePointer = () =>
  typeof window !== "undefined" &&
  typeof window.matchMedia === "function" &&
  window.matchMedia("(pointer: coarse)").matches;

// click is a PointerEvent in modern browsers (dblclick is NOT — it stays a
// plain MouseEvent), so the actual input type is read per gesture when present
// and otherwise from the pointerdown that preceded the click. This matters on
// touchscreen laptops: they report (pointer: coarse) even when a mouse is being
// used, which made plain mouse clicks open rows. Per-gesture checks keep
// mouse = double-click, touch/pen = single-tap.
const pointerTypeOf = (e: { nativeEvent: unknown }) => {
  const pt = (e.nativeEvent as PointerEvent).pointerType;
  return pt === undefined || pt === "" ? undefined : pt;
};

// Long-press detection constants
const LONGPRESS_DELAY = 500; // ms — matches Android native behavior
const MOVE_THRESHOLD = 10;   // px — cancel longpress if finger moves more than this

export interface FileRowProps {
  item: FileItem;
  /** selectedId === item.id (single active row) */
  isActive: boolean;
  /** selectedIds.has(item.id) (multi-select checkbox) */
  isMulti: boolean;
  /** dragItemId === item.id (this row is being dragged) */
  isDragging: boolean;
  /** dragOverFolder === item.id (drop target highlight) */
  isDropTarget: boolean;
  /** a drag is currently in progress (dragItemId !== null) */
  dragActive: boolean;
  onDragStart: (e: DragEvent<HTMLTableRowElement>, item: FileItem) => void;
  onDragEnd: () => void;
  onDrop: (e: DragEvent<HTMLTableRowElement>, item: FileItem) => void;
  onDragOverFolder: (item: FileItem) => void;
  onDragLeaveFolder: () => void;
  onActivate: (item: FileItem) => void;
  onToggleSelect: (id: string) => void;
  onMore: (item: FileItem, rect: DOMRect) => void;
}

// FileRowBase renders a single file/folder table row. It is wrapped in React.memo
// (see export below) so that high-frequency parent re-renders (upload progress
// ticks, toast animations) do NOT re-render every row: props only change when a
// row's own data/selection/drag state changes. All callback props must be stable
// (the parent provides ref-delegating handlers) for the memo to be effective.
function FileRowBase({
  item,
  isActive,
  isMulti,
  isDragging,
  isDropTarget,
  dragActive,
  onDragStart,
  onDragEnd,
  onDrop,
  onDragOverFolder,
  onDragLeaveFolder,
  onActivate,
  onToggleSelect,
  onMore,
}: FileRowProps) {
  // pointerType of the most recent pointerdown (dblclick itself carries none).
  const lastPointerType = useRef<string | undefined>(undefined);
  const isTouchLike = (e: MouseEvent<HTMLTableRowElement>) => {
    const pt = pointerTypeOf(e) ?? lastPointerType.current;
    return pt !== undefined ? pt !== "mouse" : isCoarsePointer();
  };

  // --- Long-press detection for mobile selection mode ---
  const longPressTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const longPressFired = useRef(false);
  const touchStartPos = useRef({ x: 0, y: 0 });

  const cancelLongPress = () => {
    if (longPressTimer.current) {
      clearTimeout(longPressTimer.current);
      longPressTimer.current = null;
    }
  };

  // Cleanup timer on unmount to prevent calling onToggleSelect on a dead component
  useEffect(() => cancelLongPress, []);

  const onRowTouchStart = (e: TouchEvent<HTMLTableRowElement>) => {
    // Only handle single-finger touches, ignore if on interactive elements
    if (e.touches.length !== 1) { cancelLongPress(); return; }
    const t = e.target as HTMLElement;
    if (t.closest(".row-more") || t.closest(".col-check") || t.closest("button") || t.closest("a")) return;
    longPressFired.current = false;
    const touch = e.touches[0];
    touchStartPos.current = { x: touch.clientX, y: touch.clientY };
    longPressTimer.current = setTimeout(() => {
      longPressFired.current = true;
      longPressTimer.current = null;
      onToggleSelect(item.id);
      // Haptic feedback if available
      if (navigator.vibrate) navigator.vibrate(30);
    }, LONGPRESS_DELAY);
  };

  const onRowTouchMove = (e: TouchEvent<HTMLTableRowElement>) => {
    if (!longPressTimer.current) return;
    const touch = e.touches[0];
    const dx = touch.clientX - touchStartPos.current.x;
    const dy = touch.clientY - touchStartPos.current.y;
    if (Math.abs(dx) > MOVE_THRESHOLD || Math.abs(dy) > MOVE_THRESHOLD) {
      cancelLongPress();
    }
  };

  const onRowTouchEnd = () => {
    cancelLongPress();
  };

  const displayName = item.name.replace(/\s+/g, " ").trim();
  const className =
    `${item.isFolder ? "is-folder" : ""}` +
    `${isActive || isMulti ? " is-selected" : ""}` +
    `${isDropTarget ? " drop-target" : ""}` +
    `${isDragging ? " dragging" : ""}`;

  return (
    <tr
      data-id={item.id}
      // Disable HTML5 drag on touch devices (not supported natively).
      // On touchscreen laptops the last pointer type distinguishes mouse vs touch.
      draggable={!isCoarsePointer()}
      onDragStart={(e) => {
        // Extra guard: if somehow dragstart fires from a touch gesture, cancel it.
        if (lastPointerType.current && lastPointerType.current !== "mouse") {
          e.preventDefault();
          return;
        }
        onDragStart(e, item);
      }}
      onDragEnd={onDragEnd}
      onDragOver={(e) => {
        if (item.isFolder && dragActive) {
          e.preventDefault();
          onDragOverFolder(item);
        }
      }}
      onDragLeave={() => onDragLeaveFolder()}
      onDrop={(e) => onDrop(e, item)}
      className={className}
      // Long-press to enter selection mode on mobile
      onTouchStart={onRowTouchStart}
      onTouchMove={onRowTouchMove}
      onTouchEnd={onRowTouchEnd}
      onTouchCancel={onRowTouchEnd}
      // Suppress native context menu on touch (our longpress handles it).
      // Desktop right-click still bubbles to the tbody handler.
      onContextMenu={(e) => {
        if (lastPointerType.current && lastPointerType.current !== "mouse") {
          e.preventDefault();
          e.stopPropagation();
        }
      }}
      onClick={(e) => {
        // If a long-press just fired, swallow the click that follows touchend.
        if (longPressFired.current) {
          longPressFired.current = false;
          e.preventDefault();
          e.stopPropagation();
          return;
        }
        // Single-tap to open on touch/pen (double-click is unreliable there).
        if (!isTouchLike(e)) return;
        // Swallow the trailing click(s) of a multi-tap so one gesture can't
        // activate the row more than once (duplicated breadcrumb bug).
        if (e.detail > 1) return;
        const t = e.target as HTMLElement;
        if (t.closest(".row-more") || t.closest(".col-check")) return;
        // While multi-selecting, a tap toggles this row instead of opening it.
        if (t.closest(".shell.is-selecting")) {
          onToggleSelect(item.id);
          return;
        }
        onActivate(item);
      }}
      onDoubleClick={(e) => {
        if (isTouchLike(e)) return;
        onActivate(item);
      }}
      onPointerDown={(e) => {
        lastPointerType.current = e.pointerType || undefined;
      }}
      onMouseDown={(e: MouseEvent<HTMLTableRowElement>) => {
        if (e.detail > 1) e.preventDefault();
      }}
    >
      <td className="col-check" onClick={(e) => e.stopPropagation()}>
        <input
          type="checkbox"
          className="row-check"
          aria-label={`Select ${displayName}`}
          checked={isMulti}
          onChange={() => onToggleSelect(item.id)}
        />
      </td>
      <td>
        <div className="name-cell">
          <span className={iconBoxClass(fileKind(item))} aria-hidden="true">
            <FileTypeIcon item={item} />
          </span>
          <span className="name-text">
            <span className="name-label" title={item.name}>{displayName}</span>
            {item.description ? (
              <span className="item-desc" title={item.description}>{item.description}</span>
            ) : null}
          </span>
        </div>
      </td>
      <td className="num">{item.isFolder ? "—" : formatBytes(item.size)}</td>
      <td className="num">{formatModified(item.modifiedTime)}</td>
      <td>
        <div className="row-actions">
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-icon row-more-btn"
            title="More actions"
            aria-label={`More actions for ${displayName}`}
            onClick={(e) => {
              e.stopPropagation();
              onMore(item, e.currentTarget.getBoundingClientRect());
            }}
          >
            <IconMore size={16} />
          </button>
        </div>
      </td>
    </tr>
  );
}

export const FileRow = memo(FileRowBase);
