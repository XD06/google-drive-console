import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { FileRow, type FileRowProps } from "./FileRow";
import type { FileItem } from "../lib/api";

// JSDOM lacks PointerEvent; polyfill so fireEvent.pointerDown carries pointerType.
if (typeof window !== "undefined" && !window.PointerEvent) {
  class PointerEventPolyfill extends MouseEvent {
    readonly pointerType: string;
    constructor(type: string, params: PointerEventInit & MouseEventInit = {}) {
      super(type, params);
      this.pointerType = params.pointerType || "";
    }
  }
  (window as unknown as Record<string, unknown>).PointerEvent = PointerEventPolyfill;
}

function makeItem(over: Partial<FileItem> = {}): FileItem {
  return {
    id: "f1",
    name: "report.pdf",
    mimeType: "application/pdf",
    size: 1024,
    modifiedTime: "2026-07-20T10:00:00Z",
    isFolder: false,
    ...over,
  };
}

function renderRow(over: Partial<FileRowProps> = {}) {
  const props: FileRowProps = {
    item: makeItem(),
    isActive: false,
    isMulti: false,
    isDragging: false,
    isDropTarget: false,
    dragActive: false,
    onDragStart: vi.fn(),
    onDragEnd: vi.fn(),
    onDrop: vi.fn(),
    onDragOverFolder: vi.fn(),
    onDragLeaveFolder: vi.fn(),
    onActivate: vi.fn(),
    onToggleSelect: vi.fn(),
    onMore: vi.fn(),
    ...over,
  };
  render(
    <table>
      <tbody>
        <FileRow {...props} />
      </tbody>
    </table>,
  );
  return props;
}

describe("FileRow", () => {
  it("renders the display name and reflects multi-select in the checkbox", () => {
    renderRow({ isMulti: true });
    expect(screen.getByText("report.pdf")).toBeInTheDocument();
    const checkbox = screen.getByRole("checkbox") as HTMLInputElement;
    expect(checkbox.checked).toBe(true);
  });

  it("fires onToggleSelect with the item id when the checkbox is clicked", () => {
    const props = renderRow();
    fireEvent.click(screen.getByRole("checkbox"));
    expect(props.onToggleSelect).toHaveBeenCalledWith("f1");
  });

  it("fires onActivate with the item on double click", () => {
    const props = renderRow();
    fireEvent.doubleClick(screen.getByRole("row"));
    expect(props.onActivate).toHaveBeenCalledTimes(1);
    expect(props.onActivate).toHaveBeenCalledWith(props.item);
  });

  it("fires onActivate only once for a double-tap on coarse pointers", () => {
    const original = window.matchMedia;
    // Simulate a touch device: (pointer: coarse) matches.
    window.matchMedia = ((query: string) => ({
      matches: query === "(pointer: coarse)",
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    })) as unknown as typeof window.matchMedia;
    try {
      const props = renderRow();
      const row = screen.getByRole("row");
      // A double-tap produces two click events (detail 1, then detail 2).
      fireEvent.click(row, { detail: 1 });
      fireEvent.click(row, { detail: 2 });
      expect(props.onActivate).toHaveBeenCalledTimes(1);
      expect(props.onActivate).toHaveBeenCalledWith(props.item);
    } finally {
      window.matchMedia = original;
    }
  });

  // Touchscreen laptops report (pointer: coarse) even when a mouse is used.
  describe("on coarse-pointer devices (touchscreen laptop)", () => {
    const original = window.matchMedia;
    const coarseMock = ((query: string) => ({
      matches: query === "(pointer: coarse)",
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    })) as unknown as typeof window.matchMedia;

    beforeEach(() => {
      window.matchMedia = coarseMock;
    });
    afterEach(() => {
      window.matchMedia = original;
    });

    it("mouse double-click still opens (per-gesture pointerType wins)", () => {
      const props = renderRow();
      const row = screen.getByRole("row");
      fireEvent.pointerDown(row, { pointerType: "mouse" });
      fireEvent.doubleClick(row);
      expect(props.onActivate).toHaveBeenCalledTimes(1);
    });

    it("plain mouse click does not open", () => {
      const props = renderRow();
      const row = screen.getByRole("row");
      fireEvent.pointerDown(row, { pointerType: "mouse" });
      fireEvent.click(row, { detail: 1 });
      expect(props.onActivate).not.toHaveBeenCalled();
    });

    it("touch tap opens once; touch dblclick does not re-open", () => {
      const props = renderRow();
      const row = screen.getByRole("row");
      fireEvent.pointerDown(row, { pointerType: "touch" });
      fireEvent.click(row, { detail: 1 });
      fireEvent.pointerDown(row, { pointerType: "touch" });
      fireEvent.doubleClick(row);
      expect(props.onActivate).toHaveBeenCalledTimes(1);
    });
  });

  it("fires onMore with the item and a rect from the more-actions button", () => {
    const props = renderRow();
    fireEvent.click(screen.getByRole("button", { name: /More actions/i }));
    expect(props.onMore).toHaveBeenCalledTimes(1);
    // called with the item and a DOMRect-like from getBoundingClientRect
    expect(props.onMore).toHaveBeenCalledWith(props.item, expect.anything());
  });

  it("fires drag callbacks", () => {
    const props = renderRow();
    const row = screen.getByRole("row");
    fireEvent.dragStart(row);
    fireEvent.dragEnd(row);
    expect(props.onDragStart).toHaveBeenCalledTimes(1);
    expect(props.onDragEnd).toHaveBeenCalledTimes(1);
  });

  it("reflects selection and folder state in the row className", () => {
    renderRow({ item: makeItem({ isFolder: true, name: "docs" }), isActive: true, isDropTarget: true });
    const row = screen.getByRole("row");
    expect(row.className).toContain("is-folder");
    expect(row.className).toContain("is-selected");
    expect(row.className).toContain("drop-target");
  });
});
