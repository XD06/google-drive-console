import { useCallback, useEffect, useRef, useState } from "react";
import { fileDownloadUrl, isImagePreviewable, isPdfPreviewable, isVideoPreviewable, type FileItem } from "../lib/api";
import { IconClose, IconDownload, IconZoomIn, IconZoomOut } from "../lib/icons";

export type MediaPreviewState = {
  id: string;
  name: string;
  mimeType?: string;
  url: string | null;
  loading: boolean;
  error: string | null;
} | null;

export type MediaLightboxProps = {
  preview: MediaPreviewState;
  item: FileItem | null;
  onClose: () => void;
};

export function MediaLightbox({ preview, item, onClose }: MediaLightboxProps) {
  const [scale, setScale] = useState(1);
  const [tx, setTx] = useState(0);
  const [ty, setTy] = useState(0);
  const dragging = useRef(false);
  const lastPos = useRef({ x: 0, y: 0 });
  const imgRef = useRef<HTMLImageElement | null>(null);
  // Live state refs for touch callbacks (avoids rebuilding callbacks on every frame)
  const stateRef = useRef({ scale: 1, tx: 0, ty: 0 });
  stateRef.current = { scale, tx, ty };
  // Snapshot taken when a two-finger pinch starts; null while not pinching.
  const pinch = useRef<{
    d0: number; s0: number; mx: number; my: number;
    cx: number; cy: number; tx0: number; ty0: number;
  } | null>(null);

  const reset = useCallback(() => { setScale(1); setTx(0); setTy(0); }, []);

  useEffect(() => { reset(); }, [preview?.id, reset]);

  const onWheel = useCallback((e: React.WheelEvent) => {
    e.preventDefault();
    setScale((s) => Math.max(0.25, Math.min(8, s - Math.sign(e.deltaY) * 0.2)));
  }, []);

  const onMouseDown = useCallback((e: React.MouseEvent) => {
    dragging.current = true;
    lastPos.current = { x: e.clientX, y: e.clientY };
  }, []);

  const onMouseMove = useCallback((e: React.MouseEvent) => {
    if (!dragging.current) return;
    // Compute deltas before the setters: updaters may run after lastPos changes.
    const dx = e.clientX - lastPos.current.x;
    const dy = e.clientY - lastPos.current.y;
    lastPos.current = { x: e.clientX, y: e.clientY };
    setTx((x) => x + dx);
    setTy((y) => y + dy);
  }, []);

  const onMouseUp = useCallback(() => { dragging.current = false; }, []);

  // Touch gestures (mobile): one finger pans, two fingers pinch-zoom around the
  // pinch midpoint. The image renders at C + t + s·P (C = layout center on
  // screen, t = translate, s = scale), so on pinch we solve for the new
  // translate that keeps the content point under the midpoint anchored:
  //   t1 = m − C − (s1/s0)·(m0 − C − t0)
  // Using the *current* midpoint m also gives two-finger panning for free.
  const onTouchStart = useCallback((e: React.TouchEvent) => {
    if (e.touches.length === 2) {
      const [a, b] = [e.touches[0], e.touches[1]];
      const { scale: s, tx: curTx, ty: curTy } = stateRef.current;
      const rect = imgRef.current?.getBoundingClientRect();
      // Layout (untransformed) center = visual center minus current translate.
      const cx = rect ? rect.left + rect.width / 2 - curTx : window.innerWidth / 2;
      const cy = rect ? rect.top + rect.height / 2 - curTy : window.innerHeight / 2;
      pinch.current = {
        d0: Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY),
        s0: s,
        mx: (a.clientX + b.clientX) / 2,
        my: (a.clientY + b.clientY) / 2,
        cx, cy, tx0: curTx, ty0: curTy,
      };
      dragging.current = false;
    } else if (e.touches.length === 1) {
      pinch.current = null;
      dragging.current = true;
      lastPos.current = { x: e.touches[0].clientX, y: e.touches[0].clientY };
    }
  }, []);

  const onTouchMove = useCallback((e: React.TouchEvent) => {
    if (e.touches.length === 2 && pinch.current) {
      const p = pinch.current;
      const [a, b] = [e.touches[0], e.touches[1]];
      const d = Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY);
      if (p.d0 <= 0) return;
      const s1 = Math.max(0.25, Math.min(8, p.s0 * (d / p.d0)));
      const k = s1 / p.s0;
      const mx = (a.clientX + b.clientX) / 2;
      const my = (a.clientY + b.clientY) / 2;
      setScale(s1);
      setTx(mx - p.cx - k * (p.mx - p.cx - p.tx0));
      setTy(my - p.cy - k * (p.my - p.cy - p.ty0));
      return;
    }
    if (e.touches.length === 1 && dragging.current) {
      const t = e.touches[0];
      const dx = t.clientX - lastPos.current.x;
      const dy = t.clientY - lastPos.current.y;
      lastPos.current = { x: t.clientX, y: t.clientY };
      setTx((x) => x + dx);
      setTy((y) => y + dy);
    }
  }, []);

  const onTouchEnd = useCallback((e: React.TouchEvent) => {
    if (e.touches.length === 1) {
      // Dropped from two fingers to one: resume panning from the remaining finger.
      pinch.current = null;
      dragging.current = true;
      lastPos.current = { x: e.touches[0].clientX, y: e.touches[0].clientY };
    } else if (e.touches.length === 0) {
      pinch.current = null;
      dragging.current = false;
    }
  }, []);

  if (!preview) return null;

  const isImage = item ? isImagePreviewable(item) : false;
  const isPdf = item ? isPdfPreviewable(item) : false;
  const isVideo = item ? isVideoPreviewable(item) : false;

  return (
    <>
      <div className="lightbox-overlay is-on" onClick={onClose} />
      <div
        className="lightbox-sheet is-on"
        role="dialog"
        aria-modal="true"
        aria-label={preview.name || "Media preview"}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="lightbox-head">
          <h3 className="lightbox-title" title={preview.name}>{preview.name || "Preview"}</h3>
          <div className="lightbox-actions">
            {isImage && (
              <>
                <button type="button" className="btn btn-ghost btn-sm btn-icon" title="Zoom out" onClick={() => setScale((s) => Math.max(0.25, s - 0.25))}>
                  <IconZoomOut size={16} />
                </button>
                <button type="button" className="btn btn-ghost btn-sm btn-icon" title="Zoom in" onClick={() => setScale((s) => Math.min(8, s + 0.25))}>
                  <IconZoomIn size={16} />
                </button>
                <button type="button" className="btn btn-ghost btn-sm" title="Reset" onClick={reset}>Reset</button>
                <span className="lightbox-zoom-label">{Math.round(scale * 100)}%</span>
              </>
            )}
            <a className="btn btn-ghost btn-sm btn-icon" href={fileDownloadUrl(preview.id, item ?? undefined)} download={preview.name} title="Download" aria-label="Download">
              <IconDownload size={16} />
            </a>
            <button type="button" className="btn btn-ghost btn-sm btn-icon" aria-label="Close" onClick={onClose}>
              <IconClose size={16} />
            </button>
          </div>
        </div>
        <div
          className="lightbox-body"
          onWheel={isImage ? onWheel : undefined}
          onMouseDown={isImage ? onMouseDown : undefined}
          onMouseMove={isImage ? onMouseMove : undefined}
          onMouseUp={isImage ? onMouseUp : undefined}
          onMouseLeave={isImage ? onMouseUp : undefined}
          onTouchStart={isImage ? onTouchStart : undefined}
          onTouchMove={isImage ? onTouchMove : undefined}
          onTouchEnd={isImage ? onTouchEnd : undefined}
          onTouchCancel={isImage ? onTouchEnd : undefined}
        >
          {preview.loading && <div className="lightbox-status">Loading…</div>}
          {preview.error && <div className="lightbox-status is-err">{preview.error}</div>}
          {isImage && preview.url && !preview.loading && (
            <img
              ref={imgRef}
              className="lightbox-img"
              src={preview.url}
              alt={preview.name}
              style={{ transform: `translate(${tx}px, ${ty}px) scale(${scale})`, cursor: scale > 1 ? "grab" : "default", transition: dragging.current ? "none" : "transform 0.1s" }}
              onDoubleClick={reset}
            />
          )}
          {isPdf && !preview.loading && (
            <iframe className="lightbox-pdf" src={fileDownloadUrl(preview.id, item ?? undefined)} title={preview.name} />
          )}
          {isVideo && !preview.loading && (
            <video className="lightbox-video" src={fileDownloadUrl(preview.id, item ?? undefined)} controls autoPlay />
          )}
        </div>
      </div>
    </>
  );
}
