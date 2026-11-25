# Research: Dual credential authentication contract

- Query: Freeze QR purpose, independent credential health, identity and persistence boundaries.
- Scope: internal (local Python reference and LitePan)
- Date: 2026-11-25

## Findings

### Source protocol anchors

`/Users/saki/Desktop/code/ebn/restored-python/util/wangpan/util_115.py:316-353,369-423`: Cookie GET qrcodeapi `/api/1.0/{app}/1.0/token/`; image `/api/1.0/web/1.0/qrcode?uid=...`; background polling GET `/get/status/` parameters uid,time(int),sign, at most five attempts with 7-second sleeps after unsuccessful attempts. Confirm status=2, POST passportapi `/app/1.0/{app}/1.0/login/qrcode/`. Exact fixed boundary `----WebKitFormBoundaryCfAXUMKv8XAmw2EU`, fields app/account(uid), CRLF encoding, Content-Disposition has double spaces after colon and semicolon. Extract response cookies in iteration order, join `; `. Exceptions terminate polling; standalone status helper also exists and router can query concurrently. Preserve both call paths in parity tests rather than claiming one generic loop.

`util_115_open.py:59-97`: default client_id 100197641. verifier token_urlsafe(96) truncated to 128 characters; challenge STANDARD padded base64(SHA256 UTF8 verifier), method literal sha256. POST passportapi `/open/authDeviceCode` form data. GET qrcodeapi `/get/status/` with uid,time(int),sign loops without explicit sleep; empty data terminates, status=2 exchanges POST passportapi `/open/deviceCodeToToken` form uid/code_verifier. Store access,refresh,expires_in,refresh_time(now). No explicit unknown-status mapping or expiry duration found here; do not invent reference guarantees.

`util_115_open.py:99-157`: refresh protected by process lock; requires refresh_token, triggers only if expires_in truthy and refresh_time+expires_in < int(now+1). POST `/open/refreshToken` form refresh_token. On successful data replace token bundle and persist SQL (fallback JSON); unsuccessful refresh retains previous access token. This differs from generic refresh-on-every-401 and must remain documented. `:267-275` user identity GET proapi `/open/user/info`, response data.user_id. `config/cookie_init.py:82-112` groups Cookie client.user_id and Open user_id as strings; pairs matching users; reference permits pools, whereas approved product requires one mandatory matching pair.

### Existing LitePan files/patterns

- internal/domain/account.go:29-62: AuthState/credentials already contain both secrets and expiries. :82 MergeAuthCredentials preserves empty fields.
- internal/auth/credentials.go: SplitConfig/ComposeConfig/CoalesceFields/ApplyUpdate hold secret separation, empty means preserve.
- internal/driver/qrlogin.go:35-46: QR results and optional provider; no purpose currently.
- internal/api/qr.go:12-42,73-154: start receives config; poll recreates ephemeral driver with EMPTY config. Session MUST hold app/client_id/verifier/sign/purpose server-side.
- internal/auth/gate.go:17-34: whole-account status gate blocks failed/expired; :39-67 passive refresh and retry failures likewise whole-account.
- internal/core/driverexec/exec.go:41,54 and internal/auth/retry.go:21-28: outer gate/retry integration must change too, otherwise driver-local capability checks cannot save Open playback after Cookie failure.
- internal/account/service.go:305-346: view exposes one status; auth update reseeding clears BOTH expiries. Dual-provider branch must preserve opposite-side expiry/status.
- internal/store/auth_repo.go:15-79: explicit SELECT/Scan/INSERT/UPDATE require every persisted addition.
- internal/store/migrate.go:12-45 embeds numbered SQL; latest observed migration 0022. Add next available migration; no legacy 115 credential conversion required, but existing other-provider DB upgrades must work.

### Proposed frozen shared contract (requires coordinator adoption)

Keep existing QRLoginProvider and GateChecker signatures for unaffected providers. Add optional PurposeQRLoginProvider with StartQRLoginFor(ctx, QRLoginOptions) and PollQRLoginFor(ctx, token, purpose); options Purpose (`cookie` or `open`), App string, ClientID string. Dual driver requires explicit valid purpose; other providers reject unsupported nonempty purpose. API start adds purpose/app/client_id; poll adds purpose. Response echoes purpose and poll includes token_expires/cookie_expires as RFC3339, verified_uid only after validation. Keep existing status vocabulary waiting/success/failed/expired. Cancel route deletes opaque server session; success consumption/expiry deletes verifier and sign. Do not encode secrets in token.

Domain AuthCapability string values `cookie_api`, `open_api`. Optional auth CapabilityGateChecker: CheckCapability(ctx,id,capability), HandleCapabilityError(ctx,id,capability,cause). Typed CapabilityAuthError wraps cause and capability; only auth failures mark credential unusable, network/429 do not invalidate identity. Add CapabilityAuthState with Status AuthStatus, VerifiedUID string, LastError string; AuthState adds CookieAuth/OpenAuth, plus CredentialGeneration uint64. Persist as one capabilities_json TEXT default '{}' and credential_generation INTEGER default 0 in existing auth row; retain common secrets columns as sole source. Decode absent map as legacy mode. Credential snapshots must not override newer generations.

Gate.Check for dual accounts enforces account enabled/registered invariant and allows dispatch when at least one capability usable; operation executor explicitly selects required capability before upstream call. Failed Cookie cannot poison Open status or trigger Open refresh. Dual operations lacking explicit capability must fail validation (avoid silent gate bypass). Other providers continue old account-wide gate and retry paths. Do not classify generic driver errors as Cookie/Open by guesswork. Media downurl requires open_api; tree/export/events require cookie_api; mixed operations gate each stage. No preflight health HTTP on playback. Existing account IsActive remains user intent.

Mount validates both sides independently and compares nonempty canonical decimal UID; reject missing/mismatch without saving a partially usable account. Cookie UID parsing alone is identity information, not proof credential is live; exact Cookie live-validation endpoint remains evidence gate below. Reconnect validates changed side against retained verified other UID. Account update and refresh share account mutex and load-merge-write critical section; empty fields preserve other credentials. Generation increments on credential replacement; stale refresh/relogin result must not overwrite newer bundle. Only successful refreshed Open credentials update Open expiry/state; Cookie reconnect only updates Cookie state. Both valid healthy; one valid degraded; neither usable unavailable. Unknown/unverified does not count valid.

Frontend account DTO additive fields: credential_health (`healthy|degraded|unavailable`), credential_statuses {cookie_api/open_api: {status,verified_uid,last_error}}, credential_generation optional internal cache use. Preserve auth_status for legacy consumers; derive dual aggregate without using it as operation gate. AddAccountDialog selects purpose, retains successful other-side values, blocks save until server validates complete pair. AccountCard/AccountManagement render two statuses and purpose-specific reconnect. No frontend-submitted health/UID is authoritative.

### File modification map

Shared schema: internal/domain/account.go; internal/store/auth_repo.go; internal/store/migrations/0023_dual_auth_capabilities.sql (reserve next number); internal/auth/credentials.go; internal/account/service.go; internal/api/accounts.go. QR: internal/driver/qrlogin.go; internal/api/qr.go; API route registration for cancel if adopted; drivers/115 auth/session implementation. Gate: internal/auth/gate_checker.go, gate.go, retry.go, refresh_runner.go, service.go, control.go/state transition helpers; internal/core/driverexec/exec.go and capability call sites. Driver: drivers/115 driver/config/auth/client cookie_auth/open_auth. Frontend: web/src/api/accounts.ts, api/qr.ts, components/admin/AddAccountDialog.vue, AccountCard.vue, AccountManagement.vue and actual shared QR component discovered by UI owner. Tests cover cross-provider unchanged behavior, degraded playback, opposite reconnect preservation, mismatched UID, expiry and generation races, exact multipart/PKCE and polling traces.

Related specs: .trellis/workflow.md; .trellis/spec/backend/index.md, database-guidelines.md and error-handling.md (currently templates, no concrete contracts). Task prd.md/design.md/reference-contract.md govern strict source parity and approved deviations.

## Caveats / Not Found

Exact Cookie live-auth identity request inside bundled P115Client has not been traced here; Cookie user_id alone may be parsed locally. This remains a blocking evidence gate before claiming mount validates Cookie validity. App/device default originates from caller route selection; do not replace with guessed web/android default. QR expiry and cancelled numeric status semantics not proven by inspected code. Session cancellation/opaque server storage, mandatory pair and independent health are approved product adaptations, not literal Python behavior. No live upstream verification or external web claims were made. Proposed naming/schema is a coordinator freeze candidate, not implemented code.
