# Design Decisions

## Security Model

- **Single-user**: This application manages one Google Drive account at a time.
  Multi-user is not supported. Any valid session maps to the stored token.
- **Token storage**: `data/token.json` contains the OAuth token for the connected
  account. All authenticated API calls use this single token.
- **Session**: Session cookies are signed with `SESSION_SECRET`. In DevMode, a
  random ephemeral secret is generated on each startup. In production,
  `SESSION_SECRET` must be set explicitly.
- **Scope**: `auth/drive` (full access) is required because the application
  browses, uploads, downloads, and organizes files across the user's entire
  Drive. `drive.file` (app-only) is insufficient for this use case.
- **Cookie security**: `SecureCookie` defaults to `!DevMode` — cookies are
  `Secure=true` in production, `false` in dev (for local HTTP).
