# 架构改进建议 — 2026-07-27

基于对 `drive-backup-console` 全栈代码的逐文件审查，以下列出实际存在的问题和具体改进方案。
每条建议都对应真实代码位置，无泛泛而谈。

---

## 一、后端（Go）

### 1.1 `api` 包直接依赖具体 `*drive.Client` — 最大耦合点

**现状**：`internal/api/files_handlers.go:19`

```go
type DriveFactory func(r *http.Request, svc *auth.Service) (*drive.Client, error)
```

所有 handler 通过 `DriveFactory` 拿到的是**具体类型** `*drive.Client`，不是接口。
这意味着：
- 测试只能起 `httptest.Server` 模拟 Drive 的 HTTP 协议（`files_handlers_test.go` 确实这么做的），无法用纯 Go object mock。
- 如果将来要接 S3 / OneDrive / 本地文件系统，需要改每一个 handler。

**对比**：同项目的 `internal/upload/service.go:29-38` 已经做对了：

```go
type DriveUploader interface {
    ResumableStart(ctx context.Context, name, mimeType, parentID string, size int64) (string, error)
    UploadRange(ctx context.Context, sessionURL string, start, end, total int64, data []byte) (drive.UploadChunkResult, error)
    // ...
}
```

`upload.Service` 只依赖这个接口，测试时传一个 struct 就行。

**建议**：在 `internal/drive/` 中定义一个 `Backend` 接口，覆盖 handler 用到的全部操作：

```go
// internal/drive/backend.go
type Backend interface {
    List(ctx context.Context, opt ListOptions) (ListResult, error)
    GetMeta(ctx context.Context, fileID string) (FileMeta, error)
    Download(ctx context.Context, fileID string) (FileMeta, io.ReadCloser, string, error)
    DownloadMedia(ctx context.Context, meta FileMeta) (FileMeta, io.ReadCloser, string, error)
    DownloadRangeMedia(ctx context.Context, meta FileMeta, rangeHeader string) (FileMeta, io.ReadCloser, string, string, int, error)
    Search(ctx context.Context, opt SearchOptions) (SearchResult, error)
    CreateFolder(ctx context.Context, name, parentID string) (FileItem, error)
    Trash(ctx context.Context, fileID string) error
    Rename(ctx context.Context, fileID, name string) (FileItem, error)
    Move(ctx context.Context, fileID, addParent, removeParent string) (FileItem, error)
    Copy(ctx context.Context, fileID, parentID, name string) (FileItem, error)
    // ... share, revisions, zip, simple upload, about
}
```

`*drive.Client` 已经满足这个接口（结构体方法签名完全匹配），不需要改 drive 包内部。
只需把 `DriveFactory` 的返回类型改为 `Backend`：

```go
type DriveFactory func(r *http.Request, svc *auth.Service) (drive.Backend, error)
```

改动量：`files_handlers.go` 一行 + `files_handlers_ext.go` 一行。
收益：api 测试可以纯 mock；未来换后端只需实现接口。

---

### 1.2 `FilesHandlers` 内聚偏低 — 21 个方法跨 4 个职责

**现状**：
- `internal/api/files_handlers.go`（611 行）：List、Download、Range、Thumbnail、Content、Mkdir、Trash、Rename、Move
- `internal/api/files_handlers_ext.go`（462 行）：Copy、Share、Unshare、Revisions、RestoreRevision、Batch、Zip、MultiZip、SimpleUpload

一个 struct 同时处理 CRUD、流式传输、权限分享、批量操作。

**建议**：按职责拆为 4 个 struct，共享同一个 `DriveFactory`：

```
files_crud.go      → List, Mkdir, Trash, Rename, Move, Copy, CreateFile
files_stream.go    → Download, Range, Thumbnail, Content, Zip
files_share.go     → Share, Unshare, Revisions, RestoreRevision
files_bulk.go      → Batch, MultiZip, SimpleUpload
```

`router.go` 中分别注册。每个 struct 3-6 个方法，内聚清晰。
这不是"为了拆而拆"——当前 `files_handlers_ext.go` 里的 Batch 和 Share 逻辑完全没有共享状态。

---

### 1.3 错误映射分散，偶尔泄露内部信息

**现状**：handler 中有三种错误写法：

```go
// 写法 1：files_handlers.go
writeDriveError(w, err)

// 写法 2：upload_handlers.go
writeUploadErr(w, err)

// 写法 3：auth_handlers.go 内联
writeJSONError(w, http.StatusBadGateway, "oauth_error", err.Error())  // ← 泄露原始错误
```

`err.Error()` 可能包含 Google API 的内部 URL、token 片段等。

**建议**：统一为一个函数：

```go
// internal/api/errors.go
func respondError(w http.ResponseWriter, err error) {
    var ae *drive.APIError
    if errors.As(err, &ae) {
        writeJSONError(w, ae.Status, ae.Code, ae.Message)
        return
    }
    var ve *upload.ValidationError
    if errors.As(err, &ve) {
        writeJSONError(w, http.StatusBadRequest, "validation", ve.Message)
        return
    }
    // 未知错误：不暴露 err.Error()
    log.Printf("unhandled error: %v", err)
    writeJSONError(w, http.StatusInternalServerError, "internal", "An unexpected error occurred")
}
```

所有 handler 只调 `respondError(w, err)`。删掉 `writeDriveError`、`writeUploadErr`。

---

### 1.4 无 panic recovery 中间件

**现状**：`cmd/server/main.go` 的中间件链是 `Logging ∘ Gzip ∘ router`。
如果任何 handler panic（比如 nil pointer），整个进程崩溃。

**建议**：加一层：

```go
func RecoverMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if rec := recover(); rec != nil {
                log.Printf("panic: %v\n%s", rec, debug.Stack())
                http.Error(w, `{"error":{"code":"internal","message":"internal server error"}}`, 500)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

放在 Logging 外层。

---

### 1.5 Gzip 旁路用 URL 子串匹配 — 脆弱

**现状**：`internal/api/middleware.go` 中 gzip 跳过逻辑：

```go
if strings.Contains(p, "/download") || strings.Contains(p, "/thumbnail") || strings.Contains(p, "/zip") {
    next.ServeHTTP(w, r)  // 不压缩
    return
}
```

如果将来有个 `/api/files/search?scope=download` 之类的路径，会被误跳过。

**建议**：改为按路由注册时标记，或者只按响应的 Content-Type 决定（当前已经有 Content-Type 检查逻辑，去掉 path 检查即可）。

---

## 二、前端（React）

### 2.1 App.tsx 3121 行 god-component — 最紧迫的问题

**现状**：一个函数组件包含：
- ~50 个 `useState`（auth、folderId、trail、items、selection、sort、8 个 dialog 开关、search 9 个变量、editor、lightbox…）
- ~15 个 `useEffect`
- ~60 个 handler 函数
- 1270 行 JSX return（line 1852 → 3121）

每次上传进度 tick（80ms）、每次按键、每次选中行，都触发这 3121 行重新执行。

**实际影响**：
- 无法对任何业务逻辑写单元测试（`doTrash`、`runSearch`、`onPickFile` 都锁在组件里）
- 加任何功能都要改这个文件，merge conflict 不可避免
- Code review 时无法只看"搜索功能的变更"——它和 dialog、上传、键盘导航混在一起

**建议**：分三步拆。

**第一步（最高 ROI）**：把文件列表逻辑抽成外部 store。
项目里已经有现成模板——`lib/uploadSession.ts`（218 行）：

```typescript
// lib/uploadSession.ts 的模式：
let jobs: UploadJob[] = [];
const listeners = new Set<() => void>();
export function useUploadJobs() { return useSyncExternalStore(subscribe, () => jobs); }
```

用同样模式建 `lib/fileStore.ts`：

```typescript
// lib/fileStore.ts
let items: FileItem[] = [];
let folderId: string | undefined;
let trail: { id: string; name: string }[] = [];
let nextToken: string | null = null;
let loading = false;
let error: string | null = null;
const cache = new Map<string, CacheEntry>();  // 现有 folderCacheRef 搬过来
// ...
export function navigateToFolder(id?: string) { /* 现有 loadFiles 逻辑 */ }
export function useFileList() { return useSyncExternalStore(sub, () => snapshot); }
export function useTrail() { ... }
```

App.tsx 中删掉 `items`、`folderId`、`trail`、`listLoading`、`listError`、`listNextToken`、`folderCacheRef`、`listAbortRef`、`loadFiles`、`loadMoreFiles`、`patchItems` 这 ~12 个 state/ref/函数，改为：

```tsx
const { items, loading, error, nextToken } = useFileList();
const trail = useTrail();
```

这一步能砍掉 App.tsx 约 400 行，且文件列表的竞态逻辑（abort、generation guard、LRU）可以独立测试。

**第二步**：把搜索抽成 `lib/searchStore.ts`（同理），删掉 App 中 9 个 search 相关 state。

**第三步**：把 dialog 状态抽成 `lib/dialogStore.ts`（confirm、rename、move、copy、mkdir、desc、saveAs），或者用一个 `useDialogState` hook。

三步做完，App.tsx 应该缩到 ~800 行（纯 layout + 组合）。

---

### 2.2 无路由 — 加页面成本极高

**现状**：`view` 是一个 string union `"files" | "overview"`，`folderId` 和 `trail` 存在 state + localStorage。
URL 永远是 `http://localhost:5174/`，刷新后靠 `loadNavState()` 恢复。

**实际影响**：
- 无法分享链接（"打开这个文件夹"只能口头说）
- 浏览器后退按钮直接退出应用
- 加"Shared with me"页面需要改：`view` 类型、App state、JSX 分支、Sidebar props、MobileNav props、键盘 handler、NavState schema

**建议**：加 hash router（零依赖，不需要 react-router）：

```typescript
// lib/router.ts
type Route =
  | { page: "files"; folderId?: string; trail: Crumb[] }
  | { page: "overview" }
  | { page: "shared" };

export function parseHash(): Route { /* 解析 #/files/folderId 或 #/overview */ }
export function navigate(route: Route) { location.hash = serialize(route); }
window.addEventListener("hashchange", () => { /* 更新 store */ });
```

`fileStore.ts` 监听 hash 变化来导航，而不是 App 手动调 `loadFiles`。
加新页面 = 加一个 Route case + 一个页面组件。

---

### 2.3 ContextMenu / Toolbar 是硬编码 JSX — 加操作要 copy-paste

**现状**：`App.tsx` ~line 3005 的右键菜单：

```tsx
{ctxMenu && (
  <div className="ctx-menu" style={{ left: ctxMenu.x, top: ctxMenu.y }}>
    <button onClick={() => { openRename(ctxMenu.item!); }}>Rename</button>
    <button onClick={() => { doShare(ctxMenu.item!); }}>Share</button>
    <button onClick={() => { doRevisions(ctxMenu.item!); }}>Revisions</button>
    <button onClick={() => { doMove(ctxMenu.item!); }}>Move to…</button>
    <button onClick={() => { doCopy(ctxMenu.item!); }}>Copy to…</button>
    <button onClick={() => { doTrash(ctxMenu.item!); }} className="danger">Trash</button>
  </div>
)}
```

加一个 "Star" 操作要：写 handler → 加 button → 加 api 调用 → 记住在 6 个地方（ctx menu、row more menu、mobile menu、keyboard shortcut、FileRow 的 aria、bulk toolbar）都加一遍。

**建议**：改为数据驱动的 action 注册表：

```typescript
// lib/fileActions.ts
export type FileAction = {
  id: string;
  label: string;
  icon: React.FC;
  danger?: boolean;
  visible: (item: FileItem) => boolean;
  run: (item: FileItem) => void;
};

export const fileActions: FileAction[] = [
  { id: "rename", label: "Rename", icon: IconRename, visible: () => true, run: openRename },
  { id: "share", label: "Share", icon: IconShare, visible: () => true, run: doShare },
  { id: "star", label: "Star", icon: IconStar, visible: () => true, run: doStar },  // ← 加一行就完事
  // ...
];
```

ContextMenu / RowMenu / MobileMenu 都渲染 `fileActions.filter(a => a.visible(item))`。
加新操作 = 数组里加一个对象。

---

### 2.4 三个组件各写一遍 fetch 模式 — 缺 `useAsyncData`

**现状**：

`components/ShareDialog.tsx:17-25`：
```tsx
useEffect(() => {
  setLoading(true);
  shareFile(id).then(setInfo).catch(e => setError(e.message)).finally(() => setLoading(false));
}, [id]);
```

`components/RevisionsPanel.tsx:16-24`：
```tsx
useEffect(() => {
  setLoading(true);
  listRevisions(id).then(setRevs).catch(e => setError(e.message)).finally(() => setLoading(false));
}, [id]);
```

`components/SettingsSheet.tsx`（API keys 部分）：同样的模式第三遍。

**建议**：

```typescript
// hooks/useAsyncData.ts
export function useAsyncData<T>(fn: () => Promise<T>, deps: unknown[]) {
  const [state, setState] = useState<{ data: T | null; loading: boolean; error: string | null }>({
    data: null, loading: true, error: null,
  });
  useEffect(() => {
    let cancelled = false;
    setState(s => ({ ...s, loading: true, error: null }));
    fn().then(d => { if (!cancelled) setState({ data: d, loading: false, error: null }); })
      .catch(e => { if (!cancelled) setState({ data: null, loading: false, error: e.message }); });
    return () => { cancelled = true; };
  }, deps);
  return state;
}
```

三个组件各删 8 行，换成 `const { data, loading, error } = useAsyncData(() => shareFile(id), [id]);`。

---

### 2.5 CSS：5182 行全局样式 + Tailwind 配了没用

**现状**：
- `demo.css`（3496 行）：定义 `:root` 变量 + 全部组件样式
- `features.css`（1686 行）：在 `html[data-theme="dark"]` 下**重新声明**了一遍变量，加新功能样式
- `tailwind.config.js` + `index.css` 里 `@tailwind components; @tailwind utilities;`：实际 TSX 中只有 1 处用了 utility class

两个文件有 ~200 行重复的变量声明。改一个颜色要改两处。

**建议**（选一个）：

**方案 A**（推荐，改动最小）：删掉 Tailwind（`tailwind.config.js`、`postcss.config.js` 中的 tailwind 插件、`index.css` 中的 `@tailwind` 行）。把 `features.css` 中重复的变量声明删掉，只保留 `demo.css` 的 `:root` + `[data-theme]` 作为唯一变量源。

**方案 B**（长期）：逐步迁移到 CSS Modules（`FileRow.module.css`），每个组件自带样式。但这是大工程，不急。

---

### 2.6 无 Error Boundary — 一个渲染错误白屏

**现状**：`main.tsx` 直接 `<App />`，无任何错误捕获。
如果 `MediaLightbox` 里 pinch-zoom 计算出了 NaN 导致渲染抛异常，整个应用白屏。

**建议**：

```tsx
// main.tsx
<StrictMode>
  <ErrorBoundary fallback={<div className="boot"><div>Something went wrong. <button onClick={() => location.reload()}>Reload</button></div></div>}>
    <App />
  </ErrorBoundary>
</StrictMode>
```

`ErrorBoundary` 是一个 15 行的 class component（React 唯一必须用 class 的地方）。

---

### 2.7 死代码 / 不一致

| 位置 | 问题 | 处理 |
|------|------|------|
| `components/ImageLightbox.tsx`（66 行） | 已被 `MediaLightbox` 取代，无任何 import | 删除 |
| `RevisionsPanel.tsx:28` | 用原生 `window.confirm()` 而非应用的 `ConfirmDialog` | 改为 `askConfirm` |
| App.tsx 中 `toastSeq`、`uploadSeq` | 模块级 mutable 变量，HMR 时不重置 | 改为 useRef 或接受现状（无实际 bug） |
| localStorage key 不统一 | `dbc.nav`、`dbc_upload_history`（下划线）vs `dbc.prefs`（点号） | 统一为 `dbc.` 前缀 + camelCase |

---

## 三、优先级排序

按"投入产出比"排序，不是按"技术完美度"：

| 优先级 | 改动 | 工作量 | 收益 |
|--------|------|--------|------|
| P0 | 后端：`DriveFactory` 返回接口（1.1） | 半天 | 解锁 mock 测试 + 未来换后端 |
| P0 | 前端：抽 `fileStore.ts`（2.1 第一步） | 1-2 天 | App 砍 400 行，文件逻辑可测试 |
| P1 | 前端：加 hash router（2.2） | 半天 | 深链接、后退键、加页面变容易 |
| P1 | 后端：统一 `respondError`（1.3） | 2 小时 | 消灭信息泄露、减少重复代码 |
| P1 | 前端：action 注册表（2.3） | 半天 | 加新文件操作从改 6 处变为改 1 处 |
| P2 | 后端：拆 FilesHandlers（1.2） | 半天 | 内聚清晰，review 容易 |
| P2 | 前端：`useAsyncData`（2.4） | 1 小时 | 消灭重复模式 |
| P2 | 前端：删 Tailwind + 合并 CSS 变量（2.5） | 2 小时 | 消除维护陷阱 |
| P3 | 后端：recover middleware（1.4） | 15 分钟 | 防 panic 杀进程 |
| P3 | 前端：Error Boundary（2.6） | 15 分钟 | 防白屏 |
| P3 | 删死代码（2.7） | 15 分钟 | 减少困惑 |

---

## 四、不建议做的事

- **不要引入 Redux / MobX**：项目规模不需要。`useSyncExternalStore` 模式（已有 `uploadSession.ts` 先例）足够，零依赖。
- **不要引入 react-router**：应用只有 3-4 个"页面"，hash router 20 行代码解决。
- **不要重写 CSS 为 Tailwind**：5000 行 CSS 迁移工作量巨大且无功能收益。删掉没用的 Tailwind 配置即可。
- **不要搞 monorepo / workspace**：一个 Go 后端 + 一个 Vite 前端，当前结构已经最简。
- **不要加 ORM / 数据库**：当前 localStorage + JSON 文件持久化对个人工具完全够用。
