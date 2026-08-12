import { ApiError, listFiles, type FileItem, type FilesListResponse } from "./api";
import {
  bustCache,
  enterFolderFromSearch,
  getFileListSnapshot,
  goCrumb,
  hydrate,
  initFileStore,
  insertResumableRow,
  insertSimpleRow,
  loadFiles,
  loadMoreFiles,
  openFolder,
  openRecentFolder,
  patchItems,
  resetOnLogout,
  revalidateIfViewing,
} from "./fileStore";

// Unit-test the store in isolation: mock the network boundary (listFiles) but
// keep the real ApiError so the store's `e instanceof ApiError` branch works.
vi.mock("./api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./api")>();
  return { ...actual, listFiles: vi.fn() };
});

const mockListFiles = vi.mocked(listFiles);

function file(id: string, over: Partial<FileItem> = {}): FileItem {
  return {
    id,
    name: id,
    mimeType: "text/plain",
    size: 1,
    modifiedTime: "2026-01-01T00:00:00Z",
    isFolder: false,
    ...over,
  };
}

function resp(
  folderId: string,
  items: FileItem[],
  nextPageToken: string | null = null,
): FilesListResponse {
  return { folderId, items, nextPageToken };
}

const flush = () => new Promise((r) => setTimeout(r, 0));

let sinks: {
  onError: ReturnType<typeof vi.fn>;
  onUnauthorized: ReturnType<typeof vi.fn>;
  onFolderOpened: ReturnType<typeof vi.fn>;
};

beforeEach(() => {
  mockListFiles.mockReset();
  resetOnLogout(); // clears items/trail/cache between tests (module singleton)
  sinks = { onError: vi.fn(), onUnauthorized: vi.fn(), onFolderOpened: vi.fn() };
  initFileStore(sinks);
});

describe("loadFiles", () => {
  it("replaces items, sets folderId, and bumps version on network success", async () => {
    const v0 = getFileListSnapshot().version;
    mockListFiles.mockResolvedValueOnce(resp("A", [file("a1"), file("a2")]));
    await loadFiles("A");
    const s = getFileListSnapshot();
    expect(s.items.map((i) => i.id)).toEqual(["a1", "a2"]);
    expect(s.folderId).toBe("A");
    expect(s.loading).toBe(false);
    expect(s.version).toBe(v0 + 1);
  });

  it("serves a cached folder with no loading flip, then revalidates silently (invariant 3)", async () => {
    mockListFiles.mockResolvedValueOnce(resp("A", [file("a1")]));
    await loadFiles("A"); // caches A

    mockListFiles.mockResolvedValueOnce(resp("A", [file("a1"), file("a2")]));
    const p = loadFiles("A"); // cache hit — paints synchronously
    // Cached data is shown immediately and loading is NOT set.
    expect(getFileListSnapshot().loading).toBe(false);
    expect(getFileListSnapshot().items.map((i) => i.id)).toEqual(["a1"]);
    await p;
    expect(getFileListSnapshot().items.map((i) => i.id)).toEqual(["a1", "a2"]);
  });

  it("discards a superseded (aborted) list response (invariant 1)", async () => {
    let resolveA!: (v: FilesListResponse) => void;
    mockListFiles.mockImplementationOnce(() => new Promise((r) => (resolveA = r)));
    mockListFiles.mockResolvedValueOnce(resp("B", [file("b1")]));

    const pA = loadFiles("A"); // pending, not cached
    const pB = loadFiles("B"); // aborts A's controller
    await pB;
    expect(getFileListSnapshot().folderId).toBe("B");

    resolveA(resp("A", [file("a1")])); // stale A response lands late
    await pA;
    const s = getFileListSnapshot();
    expect(s.folderId).toBe("B");
    expect(s.items.map((i) => i.id)).toEqual(["b1"]); // A discarded
  });

  it("routes a 401 to the onUnauthorized sink and clears error", async () => {
    mockListFiles.mockRejectedValueOnce(new ApiError(401, "unauthorized", "nope"));
    await loadFiles("A");
    expect(sinks.onUnauthorized).toHaveBeenCalledTimes(1);
    expect(getFileListSnapshot().error).toBeNull();
  });

  it("keeps stale rows and surfaces the error on a background-refresh failure", async () => {
    mockListFiles.mockResolvedValueOnce(resp("A", [file("a1")]));
    await loadFiles("A");
    mockListFiles.mockRejectedValueOnce(new Error("network"));
    await loadFiles("A"); // cache hit + failing revalidate
    const s = getFileListSnapshot();
    expect(s.items.map((i) => i.id)).toEqual(["a1"]); // cached rows retained
    expect(s.error).toBe("network");
  });

  it("evicts the oldest cache entry beyond the 30-entry cap (invariant 4)", async () => {
    mockListFiles.mockImplementation((params) =>
      Promise.resolve(resp(params?.folderId ?? "root", [file(`${params?.folderId ?? "root"}-x`)])),
    );
    for (let i = 0; i <= 30; i++) await loadFiles(`f${i}`); // 31 folders -> f0 evicted

    const p0 = loadFiles("f0"); // oldest, evicted -> cache miss -> loading
    expect(getFileListSnapshot().loading).toBe(true);
    await p0;

    const p30 = loadFiles("f30"); // still cached -> no loading flip
    expect(getFileListSnapshot().loading).toBe(false);
    await p30;
  });
});

describe("loadMoreFiles", () => {
  it("merges the next page, de-duplicating by id, without bumping version (invariant 5)", async () => {
    mockListFiles.mockResolvedValueOnce(resp("A", [file("a1"), file("a2")], "tok"));
    await loadFiles("A");
    const vAfterLoad = getFileListSnapshot().version;

    mockListFiles.mockResolvedValueOnce(resp("A", [file("a2"), file("a3")], null));
    await loadMoreFiles();
    const s = getFileListSnapshot();
    expect(s.items.map((i) => i.id)).toEqual(["a1", "a2", "a3"]); // a2 not duplicated
    expect(s.nextToken).toBeNull();
    expect(s.version).toBe(vAfterLoad); // loadMore never bumps version
  });

  it("discards a page loaded after the user switched folders (invariant 2)", async () => {
    mockListFiles.mockResolvedValueOnce(resp("A", [file("a1")], "tok"));
    await loadFiles("A");

    let resolveMore!: (v: FilesListResponse) => void;
    mockListFiles.mockImplementationOnce(() => new Promise((r) => (resolveMore = r)));
    const pMore = loadMoreFiles(); // requestKey = A, pending

    mockListFiles.mockResolvedValueOnce(resp("B", [file("b1")]));
    await loadFiles("B"); // now viewing B

    resolveMore(resp("A", [file("a2")], null)); // stale page-2 of A
    await pMore;
    const s = getFileListSnapshot();
    expect(s.folderId).toBe("B");
    expect(s.items.map((i) => i.id)).toEqual(["b1"]); // A's page-2 discarded
    expect(s.loadingMore).toBe(false);
  });

  it("routes a loadMore failure to the onError sink", async () => {
    mockListFiles.mockResolvedValueOnce(resp("A", [file("a1")], "tok"));
    await loadFiles("A");
    mockListFiles.mockRejectedValueOnce(new Error("boom"));
    await loadMoreFiles();
    expect(sinks.onError).toHaveBeenCalledWith("boom");
    expect(getFileListSnapshot().loadingMore).toBe(false);
  });
});

describe("patchItems", () => {
  it("updates the live list and the cache in sync, without bumping version", async () => {
    mockListFiles.mockResolvedValueOnce(resp("A", [file("a1", { name: "old" })]));
    await loadFiles("A");
    const vAfter = getFileListSnapshot().version;

    patchItems((list) => list.map((it) => (it.id === "a1" ? { ...it, name: "new" } : it)));
    expect(getFileListSnapshot().items[0].name).toBe("new");
    expect(getFileListSnapshot().version).toBe(vAfter);

    // Navigate away, then back: the cache must reflect the patch (no flash-back).
    mockListFiles.mockResolvedValueOnce(resp("B", [file("b1")]));
    await loadFiles("B");
    mockListFiles.mockResolvedValueOnce(resp("A", [file("a1", { name: "new" })]));
    const p = loadFiles("A"); // cache hit paints the patched name
    expect(getFileListSnapshot().items[0].name).toBe("new");
    await p;
  });
});

describe("upload row inserts", () => {
  it("insertResumableRow prepends only while viewing and never clobbers an existing row", async () => {
    mockListFiles.mockResolvedValueOnce(resp("A", [file("a1")]));
    await loadFiles("A");

    insertResumableRow("A", file("new1"));
    expect(getFileListSnapshot().items.map((i) => i.id)).toEqual(["new1", "a1"]);

    insertResumableRow("A", file("a1", { name: "changed" })); // id present -> skip
    const s = getFileListSnapshot();
    expect(s.items.map((i) => i.id)).toEqual(["new1", "a1"]);
    expect(s.items.find((i) => i.id === "a1")!.name).toBe("a1"); // not clobbered

    insertResumableRow("Z", file("z1")); // not viewing dest -> no visible insert
    expect(getFileListSnapshot().items.some((i) => i.id === "z1")).toBe(false);
  });

  it("insertSimpleRow replaces any stale copy and hoists the fresh row to the top", async () => {
    mockListFiles.mockResolvedValueOnce(resp("A", [file("a1"), file("a2")]));
    await loadFiles("A");
    insertSimpleRow("A", file("a2", { name: "fresh" }));
    const s = getFileListSnapshot();
    expect(s.items.map((i) => i.id)).toEqual(["a2", "a1"]);
    expect(s.items[0].name).toBe("fresh");
  });

  it("revalidateIfViewing refetches only when still in the destination folder", async () => {
    mockListFiles.mockResolvedValue(resp("A", [file("a1")]));
    await loadFiles("A");
    mockListFiles.mockClear();

    revalidateIfViewing("Z"); // not viewing -> no refetch
    expect(mockListFiles).not.toHaveBeenCalled();

    revalidateIfViewing("A"); // viewing -> refetch (listFiles called synchronously)
    expect(mockListFiles).toHaveBeenCalledTimes(1);
    await flush();
  });

  it("bustCache forces the next visit of that folder to hit the network", async () => {
    mockListFiles.mockResolvedValueOnce(resp("A", [file("a1")]));
    await loadFiles("A"); // caches A
    bustCache("A");
    mockListFiles.mockResolvedValueOnce(resp("A", [file("a1")]));
    const p = loadFiles("A"); // cache busted -> loading flips
    expect(getFileListSnapshot().loading).toBe(true);
    await p;
  });
});

describe("navigation actions", () => {
  it("openFolder builds the trail, fires onFolderOpened, and debounces duplicates", async () => {
    mockListFiles.mockResolvedValue(resp("F1", [file("x")]));
    const did1 = openFolder(file("F1", { isFolder: true, name: "Folder 1" }), {
      fromSearch: false,
    });
    expect(did1).toBe(true);
    expect(getFileListSnapshot().trail.map((c) => c.id)).toEqual(["F1"]);
    expect(sinks.onFolderOpened).toHaveBeenCalledWith("F1", "Folder 1");
    await flush();

    // Re-opening the folder we're already in is a guarded no-op.
    const did2 = openFolder(file("F1", { isFolder: true, name: "Folder 1" }), {
      fromSearch: false,
    });
    expect(did2).toBe(false);

    // Non-folders are ignored.
    expect(openFolder(file("doc"), { fromSearch: false })).toBe(false);
  });

  it("openFolder from search resets the trail to a single crumb", async () => {
    mockListFiles.mockResolvedValue(resp("S1", [file("x")]));
    openFolder(file("A", { isFolder: true, name: "A" }), { fromSearch: false });
    await flush();
    openFolder(file("B", { isFolder: true, name: "B" }), { fromSearch: false });
    await flush();
    expect(getFileListSnapshot().trail.map((c) => c.id)).toEqual(["A", "B"]);

    openFolder(file("S1", { isFolder: true, name: "Hit" }), { fromSearch: true });
    expect(getFileListSnapshot().trail.map((c) => c.id)).toEqual(["S1"]);
    await flush();
  });

  it("goCrumb slices the trail and root clears it", async () => {
    mockListFiles.mockResolvedValue(resp("x", [file("x")]));
    openFolder(file("A", { isFolder: true, name: "A" }), { fromSearch: false });
    await flush();
    openFolder(file("B", { isFolder: true, name: "B" }), { fromSearch: false });
    await flush();
    openFolder(file("C", { isFolder: true, name: "C" }), { fromSearch: false });
    await flush();
    expect(getFileListSnapshot().trail.map((c) => c.id)).toEqual(["A", "B", "C"]);

    goCrumb(0); // back to A
    expect(getFileListSnapshot().trail.map((c) => c.id)).toEqual(["A"]);
    await flush();

    goCrumb(-1); // root
    expect(getFileListSnapshot().trail).toEqual([]);
    await flush();
  });

  it("openRecentFolder and enterFolderFromSearch reset to a single crumb", async () => {
    mockListFiles.mockResolvedValue(resp("x", [file("x")]));
    openRecentFolder("R1", "Recent");
    expect(getFileListSnapshot().trail).toEqual([{ id: "R1", name: "Recent" }]);
    await flush();

    enterFolderFromSearch(file("S2", { isFolder: true, name: "SearchHit" }));
    expect(getFileListSnapshot().trail).toEqual([{ id: "S2", name: "SearchHit" }]);
    await flush();
  });
});

describe("hydrate / resetOnLogout", () => {
  it("hydrate restores folderId and trail", () => {
    hydrate({ folderId: "H1", trail: [{ id: "H1", name: "Home" }] });
    const s = getFileListSnapshot();
    expect(s.folderId).toBe("H1");
    expect(s.trail).toEqual([{ id: "H1", name: "Home" }]);
  });

  it("resetOnLogout clears everything and bumps version", async () => {
    mockListFiles.mockResolvedValueOnce(resp("A", [file("a1")], "tok"));
    await loadFiles("A");
    const v0 = getFileListSnapshot().version;
    resetOnLogout();
    const s = getFileListSnapshot();
    expect(s.items).toEqual([]);
    expect(s.folderId).toBeUndefined();
    expect(s.trail).toEqual([]);
    expect(s.nextToken).toBeNull();
    expect(s.version).toBe(v0 + 1);
  });
});
