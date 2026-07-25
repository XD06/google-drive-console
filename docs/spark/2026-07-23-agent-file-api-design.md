# 面向 AI Agent / 开发者的文件 API 设计（v1）

> 状态：设计已评审确认，待转实现计划
> 日期：2026-07-23
> 范围：在现有 Drive Backup Console 之上，提供可被 AI agent / 脚本安全调用的
> REST API 底座（API Key 鉴权 + `/api/v1` 契约 + OpenAPI），并新增「文件夹描述」
> 能力帮助 AI 判断文件夹用途。**不含** MCP、dry-run、路径寻址（留待后续）。

## 1. 背景与目标

现状：应用已有一套较完整的文件 REST API（list / search / 读写 / 新建 / mkdir /
upload / rename / move / copy / trash / share / permissions / revisions / zip /
thumbnail / batch / 断点续传 / overview），但**全部由浏览器 session cookie
（交互式 Google OAuth）鉴权，且没有机读契约与程序化鉴权**，因此无法被 AI 或脚本
直接调用。

目标（v1）：
1. **程序化鉴权** —— 用 API Key（Bearer token）代替 cookie，让 agent/脚本能调用。
2. **稳定对外契约** —— 新增 `/api/v1/*` 命名空间 + OpenAPI，复用现有 handler。
3. **权限护栏** —— 命名、可吊销的 Key + 只读 / 读写两档 scope。
4. **文件夹描述** —— 复用 Drive 原生 `description` 字段，列表随手返回（AI 可读），
   支持通过 API 修改，前端右键可编辑并在文件夹下截断显示。

成功标准：
- 用一个 readwrite Key 可完成「列目录 → 读文件 → 写/新建/移动」全流程。
- 用一个 read Key 只能读；对写端点返回 403 `insufficient_scope`。
- Key 可在 UI 创建（明文只显示一次）、列出、吊销。
- `GET /api/v1/openapi.json` 返回可用契约。
- AI list 文件夹时能看到 `description`；前端右键可增/改/清描述并截断显示。

## 2. 方案决策（含被否方案）

| 决策点 | 选定 | 备选（否） | 理由 |
| --- | --- | --- | --- |
| 接入形态 | 第一版做 REST + API Key + OpenAPI 底座 | 直接做 MCP | 先打基础不迈大步；底座可被后续 MCP 薄封装复用 |
| 端点面 | 新增 `/api/v1/*`，复用现有 handler | 直接给现有 `/api/*` 加 Key | 契约与 UI 内部 API 解耦，UI 改动不会弄坏 agent；成本极低 |
| UI 是否迁 v1 | 否，UI 继续走 `/api/*` | 迁移 UI 到 v1 | 避免本版动前端，风险为零；留作后续合并项 |
| Key 存储/哈希 | `data/apikeys.json` + SHA-256 | bcrypt / 本地明文 | token 为 256bit 高熵密钥，SHA-256 足够（同 GitHub PAT 做法） |
| scope 档位 | read / readwrite 两档 | 更细粒度（按端点） | 两档已满足「检索型 vs 自动化型」AI，YAGNI |
| 文件夹描述存储 | Drive 原生 `description` 字段 | 本地 sidecar json | 随 Drive 走、官方界面可见、列表顺手带回、零额外存储/同步 |

## 3. 总体架构

```
                         ┌─────────────── /api/v1/* ───────────────┐
Bearer Key ─┐            │  RequireScope(read)  ─┐                  │
            ├─ Authenticate ─▶ Principal{Kind,   ├─▶ 复用现有        │
Cookie(UI) ─┘            │      KeyID, Scope}    │   FilesHandlers   │
                         │  RequireScope(write) ─┘                  │
                         │  Key 管理端点（仅 session）              │
                         └──────────────────────────────────────────┘
UI 的 /api/* 保持不变（仅 cookie）
```

新增/改动的单元（每个单一职责、可独立测试）：
- `internal/apikey`（新包）：`Key` 模型、`Store`（持久化 + 哈希校验）。
- `internal/api/auth_mw.go`（新增中间件）：`Authenticate` + `RequireScope`。
- `internal/api/apikey_handlers.go`（新增）：Key 增 / 查 / 吊销（仅 session）。
- `internal/api/router.go`：挂载 `/api/v1/*` + Key 管理 + OpenAPI。
- `internal/drive`：`FileItem.Description`、字段掩码 + `UpdateMetadata`。
- 前端 `FileRow` / context menu / `api.ts`：描述显示 + 编辑动作 + PATCH。

## 4. 模块 A：API Key 模型与存储

`internal/apikey/apikey.go`

```go
type Scope string // "read" | "readwrite"

type Key struct {
    ID         string `json:"id"`         // 短随机 id，用于吊销/引用
    Name       string `json:"name"`       // 用户可读名字，如 "research-bot"
    Hint       string `json:"hint"`       // 明文前缀，如 "dbc_a1b2…"，仅用于识别
    Hash       string `json:"hash"`       // SHA-256(token) 十六进制
    Scope      Scope  `json:"scope"`
    CreatedAt  string `json:"createdAt"`
    LastUsedAt string `json:"lastUsedAt,omitempty"`
    Revoked    bool   `json:"revoked"`
}
```

- **生成**：随机 32 字节 → token 形如 `dbc_<base32(无填充)>`。返回给用户的**明文只在创建时出现一次**；库里只存 `Hash`。`Hint` = `dbc_` + 随机段前 6 位。
- **校验**：`token → SHA-256 → 常量时间比对 Hash`；命中且 `!Revoked` 才通过。
- **持久化**：`internal/apikey/store.go`，落 `data/apikeys.json`（沿用 tokenstore
  的模式：文件权限 0600、临时文件 + 原子 rename、内存加读写锁）。
- **LastUsedAt**：每次成功校验后更新（可节流为「距上次 > 1 分钟才写盘」，避免频繁 IO）。

## 5. 模块 B：鉴权解析（cookie 或 Key）

`internal/api/auth_mw.go`

```go
type PrincipalKind int // session | apikey

type Principal struct {
    Kind  PrincipalKind
    KeyID string        // apikey 时有值
    Scope apikey.Scope  // session => readwrite
}
```

`Authenticate(svc, keyStore)` 中间件按序判定并把 `Principal` 放进 context：
1. 请求头有 `Authorization: Bearer <token>` → 校验 Key；失败 401，成功 → `Principal{apikey, keyID, key.Scope}`。
2. 否则查 session cookie（复用 `svc.Sessions.FromRequest`）→ `Principal{session, "", readwrite}`。
3. 都没有 → 401 `not_authenticated`。

**关键语义**：Key 不携带自身 Google 身份，只是「以站点主人身份调用 API」的凭证
（仍使用服务端那份 Google token）。scope 限制**能做什么**（读/写），不是**谁的盘**。

## 6. 模块 C：Scope 门禁

`RequireScope(min apikey.Scope)` 中间件（在 `Authenticate` 之后）：
- `session` principal → 恒通过（UI 主人 = 完整权限）。
- `apikey` principal → 需满足 `key.Scope >= min`（`readwrite` 覆盖 `read`），否则
  403 `insufficient_scope`。

端点 → 最低 scope 映射：

| scope | 端点 |
| --- | --- |
| `read` | GET list / search / content / download / permissions / revisions / zip / thumbnail / overview / uploads 状态(GET /api/uploads/{id}) |
| `readwrite` | PUT content；POST create / mkdir / simple / copy / move / batch / zip / share；uploads 创建/分片/取消(POST /api/uploads、PUT chunk、POST cancel)；PATCH（rename+description）；DELETE trash / unshare；POST revisions/restore |

- **Key 管理端点仅认 session**（`RequireSession`），Key 自己不能再造/吊销 Key。

## 7. 模块 D：`/api/v1` 端点面 + 发现

- `internal/api/router.go` 中新增：对现有 file handlers 用
  `Authenticate(...)` + `RequireScope(...)` 包一层，挂到 `/api/v1/files...`
  （**复用同一批 handler 函数，不复制业务逻辑**；响应体与现有一致）。
- Key 管理（仅 session）：
  - `POST /api/v1/keys` `{name, scope}` → 返回 `{id, name, scope, token}`（`token` 明文仅此一次）。
  - `GET /api/v1/keys` → `[{id, name, hint, scope, createdAt, lastUsedAt, revoked}]`（**不含明文/hash**）。
  - `DELETE /api/v1/keys/{id}` → 标记 `Revoked=true`。
- 发现：`GET /api/v1/openapi.json` 返回静态 OpenAPI 3.1 契约（描述所有 v1 端点、
  scope、错误码、schema）。契约文件维护在 `internal/api/openapi.json` 内嵌（`go:embed`）。
- **寻址**：v1 仍为 **Drive ID 寻址**（agent 先 list/search 拿 ID）；路径寻址留待后续。

## 8. 模块 E：文件夹描述（复用 Drive 原生 `description`）

**Drive 层**（`internal/drive`）
- `types.go`：`FileItem` 增 `Description string \`json:"description,omitempty"\``。
- 字段掩码追加 `description`：
  - `client.go`（list，约 L63）、`search.go`（约 L89）：
    `...,thumbnailLink)` → `...,thumbnailLink,description)`。
  - `parseFileItem` 解析 `description`。
- `mutate.go`：把 `Rename(id, name)` 泛化为
  `UpdateMetadata(ctx, id string, name *string, description *string) (FileItem, error)`：
  只把非 nil 字段写进 files.update body；`description == ""`（非 nil）表示清空；
  返回字段掩码加 `description`。保留 `Rename` 作为薄封装（`UpdateMetadata(id,&name,nil)`）。

**API 层**
- 扩展 `PATCH /api/files/{id}` 与 `/api/v1/files/{id}` 请求体为
  `{ "name"?: string, "description"?: string }`：
  - 二者都缺 → 400 `bad_request`。
  - 仅 `name` → 改名（同现状）；仅 `description` → 改描述；两者都有 → 一次更新。
  - `description: ""` → 清空描述。
  - 写描述属 **readwrite**；读（随 list/get 返回）属 **read**。
- 描述在 Drive 元数据层对任意文件通用，API 不作类型限制；UI 只在文件夹暴露编辑入口。
- 长度：Drive `description` 上限约 4096 字符；超长由 Drive 自身报错，API 透传 400。

**前端**（`web/src`）
- `lib/api.ts`：`FileItem` 增 `description?: string`；`renameFile` 旁新增
  `updateFile(id, { name?, description? })`（PATCH）。
- `components/FileRow.tsx`：name-cell 结构调整为
  `[图标] [ .name-text（名称） + .item-desc（描述，可选） ]`
  （图标左侧，右侧为竖向 name+desc）。`.item-desc` 单行截断：
  `white-space:nowrap; overflow:hidden; text-overflow:ellipsis; max-width:100%`，
  并加 `title={description}` 悬浮显示全文。无描述时只渲染名称，布局同现状。
- 右键上下文菜单（`App.tsx` 的 ctx-menu，文件夹分支）：加一项
  **「编辑描述」**（描述为空时显示「添加描述」）→ 打开一个小 sheet（复用 rename
  sheet 的样式与交互），textarea 预填当前描述，保存调用 `updateFile(id,{description})`
  后刷新当前列表。
- 样式：`.item-desc` 定义放 `demo.css`；桌面与移动同一结构（移动端 name-cell 已在
  grid row1，描述行在其内竖排，不影响 grid）。

## 9. 错误与规范

- 复用现有 `writeJSONError{code, message}` 结构化错误。OpenAPI 列全错误码：
  - 401 `not_authenticated`、403 `insufficient_scope`、400 `bad_request`、
    404 `not_found`、409 冲突、413 body 超限、5xx 上游/内部。
- 分页沿用 `nextPageToken`；响应体保持与现有 handler 一致。

## 10. 配置与审计

- 配置：可选 `APIKEYS_PATH`（默认 `<DataDir>/apikeys.json`）；无强制新增项。
  Key 能力在 `Auth.Enabled` 时可用。
- 审计（轻量）：在现有 `LoggingMiddleware` 基础上，为带 Key 的请求日志追加
  `keyHint + scope + method + path + status`。结构化审计文件留待后续。

## 11. 测试计划

- `internal/apikey`：生成 → 哈希 → 校验（命中/未命中/已吊销）→ 持久化往返（reload）。
- `Authenticate`：仅 cookie / 仅 bearer / 无凭证 / 无效 token / 已吊销 → 对应结果。
- `RequireScope`：read Key 打写端点 → 403；readwrite → 通过；session → 全通。
- v1 冒烟：read Key `GET /api/v1/files` 200；read Key `POST /api/v1/files` 403；
  readwrite Key 写成功；`GET /api/v1/openapi.json` 可取。
- Key 管理：Key 无法访问 `/api/v1/keys`（仅 session）；创建仅返回一次明文；吊销后失效。
- 描述：`UpdateMetadata` 写入并回读 `description`；list 带回 `description`；
  PATCH `{description}` readwrite 成功 / read → 403；`parseFileItem` 解析。
- 前端：`FileRow` 在有/无描述时的渲染（截断 + title）；ctx-menu「编辑描述」触发 PATCH。

## 12. 明确不做（v1）与后续路线

- **不做**：MCP 封装、破坏性操作 dry-run、路径寻址（/Documents/x.txt）、限流、
  结构化审计文件、把 UI 迁到 `/api/v1`。
- **后续**：
  - v1.1：MCP 服务器薄封装（工具即调用），目标 `/api/v1`；破坏性操作 dry-run；路径寻址。
  - v1.2：结构化审计日志文件、限流、更细 scope。

## 13. 文件改动地图（决策完备，供实现计划参考）

新增：
- `internal/apikey/apikey.go`、`internal/apikey/store.go`（+ 测试）
- `internal/api/auth_mw.go`（`Authenticate` / `RequireScope` / `Principal`，+ 测试）
- `internal/api/apikey_handlers.go`（Key 增/查/吊销，+ 测试）
- `internal/api/openapi.json`（`go:embed` 内容） + `openapi_handler.go`

改动：
- `internal/api/router.go`：挂载 `/api/v1/*`、Key 管理、OpenAPI。
- `internal/config/config.go`：可选 `APIKEYS_PATH`。
- `cmd/server/main.go`：构造 `apikey.Store` 并注入 Deps。
- `internal/drive/types.go`：`FileItem.Description`。
- `internal/drive/client.go`、`search.go`：字段掩码加 `description`。
- `internal/drive/mutate.go`：`Rename` → `UpdateMetadata`（name/description 可选）。
- `internal/api/files_handlers*.go`：PATCH handler 支持 `description`。
- `web/src/lib/api.ts`：`FileItem.description`、`updateFile`。
- `web/src/components/FileRow.tsx`：name-cell 竖排 name+desc、截断。
- `web/src/App.tsx`：ctx-menu「编辑描述」+ 编辑 sheet。
- `web/src/demo.css`：`.item-desc` 截断样式。
