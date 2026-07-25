import { describe, expect, it, vi } from "vitest";
import { render, fireEvent } from "@testing-library/react";
import { MediaLightbox, type MediaPreviewState } from "./MediaLightbox";
import type { FileItem } from "../lib/api";

function makeItem(over: Partial<FileItem> = {}): FileItem {
  return {
    id: "img1",
    name: "photo.png",
    mimeType: "image/png",
    size: 2048,
    modifiedTime: "2026-07-20T10:00:00Z",
    isFolder: false,
    ...over,
  };
}

function makePreview(over: Partial<NonNullable<MediaPreviewState>> = {}): MediaPreviewState {
  return {
    id: "img1",
    name: "photo.png",
    url: "http://localhost/photo.png",
    loading: false,
    error: null,
    ...over,
  };
}

function renderLightbox() {
  const onClose = vi.fn();
  const utils = render(<MediaLightbox preview={makePreview()} item={makeItem()} onClose={onClose} />);
  const body = utils.container.querySelector(".lightbox-body") as HTMLElement;
  const img = utils.container.querySelector(".lightbox-img") as HTMLImageElement;
  const label = () => utils.container.querySelector(".lightbox-zoom-label")?.textContent;
  return { ...utils, onClose, body, img, label };
}

const touch = (identifier: number, clientX: number, clientY: number) => ({ identifier, clientX, clientY });

describe("MediaLightbox", () => {
  it("zooms via the +/- buttons and reset restores 100%", () => {
    const { container, label } = renderLightbox();
    expect(label()).toBe("100%");
    fireEvent.click(container.querySelector('button[title="Zoom in"]')!);
    expect(label()).toBe("125%");
    fireEvent.click(container.querySelector('button[title="Zoom out"]')!);
    expect(label()).toBe("100%");
    fireEvent.click(container.querySelector('button[title="Zoom in"]')!);
    fireEvent.click(container.querySelector('button[title="Reset"]')!);
    expect(label()).toBe("100%");
  });

  it("zooms with the mouse wheel", () => {
    const { body, label } = renderLightbox();
    fireEvent.wheel(body, { deltaY: -100 });
    expect(label()).toBe("120%");
    fireEvent.wheel(body, { deltaY: 100 });
    expect(label()).toBe("100%");
  });

  it("pinch gesture with two fingers zooms the image", () => {
    const { body, img, label } = renderLightbox();
    fireEvent.touchStart(body, {
      touches: [touch(0, 100, 100), touch(1, 200, 100)],
      changedTouches: [touch(0, 100, 100), touch(1, 200, 100)],
    });
    // Spread fingers from 100px to 200px apart -> scale doubles.
    fireEvent.touchMove(body, {
      touches: [touch(0, 50, 100), touch(1, 250, 100)],
      changedTouches: [touch(0, 50, 100), touch(1, 250, 100)],
    });
    expect(label()).toBe("200%");
    expect(img.style.transform).toContain("scale(2)");
    fireEvent.touchEnd(body, { touches: [], changedTouches: [touch(0, 50, 100)] });
  });

  it("one-finger drag pans the zoomed image", () => {
    const { body, img } = renderLightbox();
    fireEvent.touchStart(body, { touches: [touch(0, 100, 100)], changedTouches: [touch(0, 100, 100)] });
    fireEvent.touchMove(body, { touches: [touch(0, 140, 120)], changedTouches: [touch(0, 140, 120)] });
    fireEvent.touchEnd(body, { touches: [], changedTouches: [touch(0, 140, 120)] });
    expect(img.style.transform).toContain("translate(40px, 20px)");
  });
});
