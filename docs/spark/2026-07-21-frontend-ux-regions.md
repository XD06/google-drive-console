# Drive Backup Console — Frontend UX Regions (v1 demo)

**Date:** 2026-07-21  
**Design read:** Personal backup **product console** (not marketing site): modern, calm, dense enough for files, visual progress. Language: Linear / Finder hybrid, zinc neutrals + single emerald accent.

**Dials:** VARIANCE 4 · MOTION 3 · DENSITY 6

## Information architecture (v1 screens)

1. **Gate / Login** — not signed in  
2. **Browser** — primary workspace (list + navigate + upload/download)  
3. **Transient overlays** — toast, confirm delete (later), upload drop overlay  

No sidebar multi-app nav in v1 (single purpose app).

## Region map (Browser shell)

```
┌─────────────────────────────────────────────────────────────┐
│ A. Top bar          logo · connection status · account     │
├─────────────────────────────────────────────────────────────┤
│ B. Path bar         breadcrumb · refresh · view toggle     │
├─────────────────────────────────────────────────────────────┤
│ C. Toolbar          Upload · New folder · Search (optional)│
├───────────────────────────────┬─────────────────────────────┤
│ D. Main file table            │ E. Activity rail (optional) │
│    name · size · modified     │    upload queue / progress  │
│    row actions: open/dl       │    collapse on narrow       │
├───────────────────────────────┴─────────────────────────────┤
│ F. Status footer   selection count · quota hint · errors   │
└─────────────────────────────────────────────────────────────┘
```

| Region | Job | Primary actions | Empty / error |
|--------|-----|-----------------|---------------|
| A Top bar | Trust + session | Connect / Disconnect | Show “Not connected” pill |
| B Path | Orientation | Click crumb, go up | Root = Drive / Backup |
| C Toolbar | Create ingress | Upload (primary), New folder | Disable when offline |
| D Table | Browse & select | Open folder, download file | Illustrated empty + Upload CTA |
| E Activity | Upload feedback | Pause/cancel later; v1 progress only | Hidden when queue empty |
| F Footer | Ambient status | Retry failed | Inline error + retry |

## Key flows

### Connect Google
1. Gate screen → primary **Connect Google**  
2. Browser redirect OAuth (real app)  
3. Return → Browser shell, status pill **Connected**

### Navigate
- Click folder row → push breadcrumb, replace table  
- Crumb click → pop stack  
- Keyboard: arrows + Enter (demo: basic)

### Upload
1. Drag onto D (full-region drop overlay) **or** Toolbar Upload  
2. Jobs appear in E with determinate progress  
3. Complete → toast + table refresh highlight  

### Download
- Row action or double-click file → browser download via API (real app)

## Visual system (demo tokens)

| Token | Value | Note |
|-------|--------|------|
| Surface | `#0c0e12` / elevated `#141820` | Dark product UI |
| Border | `rgba(255,255,255,0.08)` | Hairline |
| Text | `#e8eaed` / muted `#8b939e` | AA on dark |
| Accent | `#34d399` (emerald) | Progress, primary CTA |
| Danger | `#f87171` | Errors only |
| Radius | 8px controls, 12px panels | One scale |
| Type | System UI + tabular nums for sizes | No Inter/purple |

**Anti-patterns avoided:** purple gradients, 3-card marketing hero, emoji icons, centered “SaaS landing” layout.

## Component inventory (demo)

- `TopBar`, `Breadcrumb`, `Toolbar`, `FileTable`, `UploadRail`, `DropOverlay`, `GateScreen`, `Toast`, `StatusPill`

## Responsive

- `≥1024px`: D + E side by side  
- `<1024px`: E becomes bottom sheet / collapsible panel  
- Touch targets ≥44px for primary actions  

## Demo scope

Static HTML mock with fake data: switch Gate ↔ Browser, fake upload progress, breadcrumb navigation. No real Google API.
