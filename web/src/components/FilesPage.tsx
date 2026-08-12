import { type Dispatch, type DragEvent, type RefObject, type SetStateAction } from "react";
import { type FileItem, type SearchScope } from "../lib/api";
import { FileRow } from "./FileRow";
import { IconChevronDown, IconChevronUp, IconDownload, IconTrash } from "../lib/icons";

type SortKey = "name" | "size" | "modified";
type SortDir = "asc" | "desc";

/** Row interaction callbacks (mirrors App's stable `rowHandlers` object). */
export type RowHandlers = {
  onDragStart: (e: DragEvent<HTMLTableRowElement>, item: FileItem) => void;
  onDragEnd: () => void;
  onDrop: (e: DragEvent<HTMLTableRowElement>, item: FileItem) => void;
  onActivate: (item: FileItem) => void;
  onToggleSelect: (id: string) => void;
  onDragOverFolder: (item: FileItem) => void;
  onDragLeaveFolder: () => void;
  onMore: (item: FileItem, rect: DOMRect) => void;
};

export type FilesPageProps = {
  dropOn: boolean;
  // Data (already sorted upstream) + the search/list mode split.
  sortedItems: FileItem[];
  sortedSearchResults: FileItem[];
  searchMode: boolean;
  searchError: string | null;
  listError: string | null;
  searchLoading: boolean;
  listLoading: boolean;
  // Sorting.
  sortKey: SortKey;
  sortDir: SortDir;
  toggleSort: (key: SortKey) => void;
  // Selection.
  selectedId: string | null;
  selectedIds: Set<string>;
  setSelectedIds: (ids: Set<string>) => void;
  toggleSelectAllVisible: (visible: FileItem[]) => void;
  doBulkDownload: (visible: FileItem[]) => void;
  doBulkZip: (visible: FileItem[]) => void;
  doBulkTrash: () => void;
  // Row context menu.
  setCtxMenu: (menu: { x: number; y: number; item: FileItem | null } | null) => void;
  // Rows / progressive rendering.
  renderLimit: number;
  setRenderLimit: Dispatch<SetStateAction<number>>;
  dragItemId: string | null;
  dragOverFolder: string | null;
  rowHandlers: RowHandlers;
  // Paging — search results.
  searchNextToken: string | null;
  runSearch: (opts: {
    q: string;
    scope: SearchScope;
    pageToken?: string;
    suggest?: boolean;
    append?: boolean;
  }) => void;
  searchActiveQuery: string;
  searchScope: SearchScope;
  // Paging — folder listing.
  listNextToken: string | null;
  listLoadingMore: boolean;
  loadMoreFiles: () => void;
  // Empty state.
  busy: boolean;
  fileInputRef: RefObject<HTMLInputElement | null>;
  exitSearchMode: () => void;
  loadFiles: (fid?: string) => void;
  folderId?: string;
};

/**
 * Files page — the "My Drive" listing view. Presentational shell: all list data
 * and mutations live in App/fileStore and are handed in as props, so App stays a
 * thin `view`-switcher. Adding a third page means another component like this.
 */
export function FilesPage({
  dropOn,
  sortedItems,
  sortedSearchResults,
  searchMode,
  searchError,
  listError,
  searchLoading,
  listLoading,
  sortKey,
  sortDir,
  toggleSort,
  selectedId,
  selectedIds,
  setSelectedIds,
  toggleSelectAllVisible,
  doBulkDownload,
  doBulkZip,
  doBulkTrash,
  setCtxMenu,
  renderLimit,
  setRenderLimit,
  dragItemId,
  dragOverFolder,
  rowHandlers,
  searchNextToken,
  runSearch,
  searchActiveQuery,
  searchScope,
  listNextToken,
  listLoadingMore,
  loadMoreFiles,
  busy,
  fileInputRef,
  exitSearchMode,
  loadFiles,
  folderId,
}: FilesPageProps) {
  return (
    <>
      <div
        className={`drop-overlay${dropOn ? " is-on" : ""}`}
        id="drop-overlay"
        aria-hidden={!dropOn}
      >
        Drop files to upload
      </div>
      {(searchMode ? searchError : listError) && (
        <p className="list-error">{searchMode ? searchError : listError}</p>
      )}
      {(() => {
        const tableItems = searchMode ? sortedSearchResults : sortedItems;
        const tableLoading = searchMode ? searchLoading : listLoading;
        const tableError = searchMode ? searchError : listError;
        const sortAria = (key: SortKey) =>
          sortKey === key ? (sortDir === "asc" ? "ascending" : "descending") : "none";
        const SortIcon = ({ col }: { col: SortKey }) =>
          sortKey !== col ? null : sortDir === "asc" ? (
            <IconChevronUp size={12} />
          ) : (
            <IconChevronDown size={12} />
          );
        return (
          <>
            {(tableLoading || tableItems.length > 0 || tableError) && (
              <div className="thead-wrap">
                <table className="file-table">
                  <thead>
                    <tr>
                      <th className="col-check">
                        <input
                          type="checkbox"
                          className="row-check"
                          aria-label="Select all"
                          checked={tableItems.length > 0 && tableItems.every((it) => selectedIds.has(it.id))}
                          onChange={() => toggleSelectAllVisible(tableItems)}
                        />
                      </th>
                      <th aria-sort={sortAria("name")}>
                        <button type="button" className="th-sort" onClick={() => toggleSort("name")}>
                          Name <SortIcon col="name" />
                        </button>
                      </th>
                      <th aria-sort={sortAria("size")}>
                        <button type="button" className="th-sort" onClick={() => toggleSort("size")}>
                          Size <SortIcon col="size" />
                        </button>
                      </th>
                      <th aria-sort={sortAria("modified")}>
                        <button type="button" className="th-sort" onClick={() => toggleSort("modified")}>
                          Modified <SortIcon col="modified" />
                        </button>
                      </th>
                      <th />
                    </tr>
                  </thead>
                </table>
              </div>
            )}
            <div className="table-wrap">
            <div style={{ position: "relative" }}>
            {selectedIds.size > 0 && (
              <div className="selection-bar" role="status">
                <span className="selection-count">{selectedIds.size} selected</span>
                <button type="button" className="btn btn-ghost btn-sm" onClick={() => doBulkDownload(tableItems)}>
                  <IconDownload size={14} /> Download
                </button>
                <button type="button" className="btn btn-ghost btn-sm" onClick={() => void doBulkZip(tableItems)}>
                  <IconDownload size={14} /> ZIP
                </button>
                <button type="button" className="btn btn-ghost btn-sm btn-danger" onClick={() => void doBulkTrash()}>
                  <IconTrash size={14} /> Trash
                </button>
                <button type="button" className="btn btn-ghost btn-sm" onClick={() => setSelectedIds(new Set())}>
                  Clear
                </button>
              </div>
            )}
            {(tableLoading || tableItems.length > 0 || tableError) && (
              <table className="file-table">
                <tbody
                  onContextMenu={(e) => {
                    e.preventDefault();
                    const target = e.target as HTMLElement;
                    const tr = target.closest("tr");
                    const id = tr?.getAttribute("data-id");
                    const item = id ? tableItems.find((it) => it.id === id) ?? null : null;
                    setCtxMenu({ x: e.clientX, y: e.clientY, item });
                  }}
                >
                  {tableLoading && tableItems.length === 0 && (
                    Array.from({ length: 5 }).map((_, i) => (
                      <tr key={`skel-${i}`} className="skel-row-tr">
                        <td colSpan={5} style={{ padding: 0 }}>
                          <div className="skel-row">
                            <span className="skel-bar skel-icon" style={{ width: 24, height: 24 }} />
                            <span className="skel-bar skel-name" style={{ width: `${60 + Math.random() * 30}%` }} />
                            <span className="skel-bar skel-size" />
                            <span className="skel-bar skel-date" />
                          </div>
                        </td>
                      </tr>
                    ))
                  )}
                  {tableItems.slice(0, renderLimit).map((item) => (
                    <FileRow
                      key={item.id}
                      item={item}
                      isActive={selectedId === item.id}
                      isMulti={selectedIds.has(item.id)}
                      isDragging={dragItemId === item.id}
                      isDropTarget={dragOverFolder === item.id}
                      dragActive={dragItemId !== null}
                      onDragStart={rowHandlers.onDragStart}
                      onDragEnd={rowHandlers.onDragEnd}
                      onDrop={rowHandlers.onDrop}
                      onDragOverFolder={rowHandlers.onDragOverFolder}
                      onDragLeaveFolder={rowHandlers.onDragLeaveFolder}
                      onActivate={rowHandlers.onActivate}
                      onToggleSelect={rowHandlers.onToggleSelect}
                      onMore={rowHandlers.onMore}
                    />
                  ))}
                  {tableItems.length > renderLimit && (
                    <tr>
                      <td colSpan={5} style={{ textAlign: "center", padding: "12px" }}>
                        <button
                          type="button"
                          className="btn btn-ghost btn-sm"
                          onClick={() => setRenderLimit((prev) => prev + 200)}
                        >
                          Show more ({tableItems.length - renderLimit} remaining)
                        </button>
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            )}
            {searchMode && searchNextToken && !tableLoading && (
              <div className="search-load-more">
                <button
                  type="button"
                  className="btn"
                  disabled={searchLoading}
                  onClick={() =>
                    void runSearch({
                      q: searchActiveQuery,
                      scope: searchScope,
                      pageToken: searchNextToken,
                      append: true,
                    })
                  }
                >
                  Load more
                </button>
              </div>
            )}
            {!searchMode && listNextToken && !tableLoading && (
              <div className="search-load-more">
                <button
                  type="button"
                  className="btn"
                  disabled={listLoadingMore}
                  onClick={() => void loadMoreFiles()}
                >
                  {listLoadingMore ? "Loading…" : "Load more"}
                </button>
              </div>
            )}
            <div
              className={`empty${!tableLoading && tableItems.length === 0 && !tableError ? " is-on" : ""}`}
              id="empty-state"
            >
              {searchMode ? (
                <>
                  <strong>No matches</strong>
                  <span>Try another name or switch scope (Folder / Drive).</span>
                  <button
                    type="button"
                    className="btn"
                    style={{ marginTop: 12 }}
                    onClick={() => {
                      exitSearchMode();
                      void loadFiles(folderId);
                    }}
                  >
                    Back to folder
                  </button>
                </>
              ) : (
                <>
                  <strong>This folder is empty</strong>
                  <span>Upload a backup or create a folder.</span>
                  <button
                    type="button"
                    className="btn btn-primary"
                    style={{ marginTop: 12 }}
                    disabled={busy}
                    onClick={() => fileInputRef.current?.click()}
                  >
                    Upload
                  </button>
                </>
              )}
            </div>
            </div>
            </div>
          </>
        );
      })()}
    </>
  );
}
