# W2 authentication implementation — final integration record

## Source mapping and boundaries

Approved product adaptations R3/R4/R9 map to independent credential health, purpose QR API, generation-conditional persistence and capability-aware gates. research/auth-contract.md maps protocol source anchors in util_115.py and util_115_open.py; driver/client workers own those requests. This generic framework adds no upstream requests, waits or retries. Health splitting and opaque cancellation are approved product adaptations, not literal Python functionality.

Ordered framework trace: QR start -> purpose provider start; poll -> purpose provider poll -> credentials and RFC3339 expiry DTO; cancel -> provider cancellation. Empty-purpose legacy providers retain original dispatch. Ephemeral QR never calls account Init/Ping. Initial account creation -> Init/Ping -> both active verified credentials with matching UID -> complete health persistence. Reconnect -> optional CredentialUpdateValidator(previous credentials/verified identity) -> changed-side validation -> resulting health persistence; W4 implements the 115 validator.

## Final persistence and authentication behavior

Migration 0023 stores capabilities_json and credential_generation. Manager compares credential secrets and expiries before invoking the credentials-persisted callback. Health-only observations persist without refresh-success handling, LastRefreshAt changes, synthetic expiry changes or playback invalidation. Changed secrets/expiries retain synchronous playback invalidation before persistence returns. Account reconnects share the authentication account lock and preserve opposite credentials/expiries. Production SQL conditional persistence rejects stale generation explicitly. Existing-row credential writes use optional AuthCredentialPatcher: one conditional SQL UPDATE merges supplied nonempty secrets, nonzero expiries and supplied health pointers only. Generation advances only when a submitted secret differs. This preserves a concurrent opposite-side health patch that arrived after Manager loaded its snapshot.

The optional domain.CredentialHealthPatcher performs one generation-conditional SQL UPDATE using json_set for supplied health sides only. Manager health-only persistence and Gate.HandleCapabilityError use this path, preserving other health, all secrets and timestamps. It avoids a manager mutex around auth callbacks. Runtime bookkeeping uses optional AuthRuntimeUpdater: success/failure/retry/scheduling updates change runtime and scheduling columns only, preserving capabilities, secrets and generation. For rows with either active/nonempty capability health status, credential persistence exclusively owns both expiry fields: runtime SQL preserves the current row expiries. Legacy rows without capability health retain generic inferred-expiry scheduling. This predicate inspects current SQL state, so same-generation expiry-only patches cannot be reverted by older runtime snapshots. Its generation predicate rejects bookkeeping from before a reconnect. The account lock alone did not prevent races with driver inline health persistence. Repositories without the optional patch interface retain the existing fallback; the production SQL repository implements the atomic patch.

Gate.Check enforces manual IsActive, then admits a dual account if any capability is active. CheckCapability checks the requested stage. Typed capability failures bypass whole-account retry and refresh failure handling. Only auth failures expire the affected capability; network errors do not invalidate identity. Cookie health failure preserves Open playback cache. Gate handling of Open failure invalidates playback. Driver operations implement their stage checks and runtime health persistence; shared types and generation contracts are integrated with W4.

## Playback invalidation and refresh completion

Application wiring connects auth.SetCredentialInvalidator to playback.InvalidateAccount synchronously. Credential material/expiry persistence invalidates even inside an active refresh scope; actual refreshed completion and account updates retain invalidation. Health-only writes do not call that callback, and Cookie-only health failure does not evict Open links. W5 owns the Emby final-cache account check and bypass behavior; shared Gate rejects disabled accounts.

Config.RefreshAtExpiry bypasses generic early-refresh defaults and synthetic TokenLifetime updates. W4 sets it for 115 and prevents pre-expiry HTTP refresh. finishRefresh skips refresh-success mutation when such a provider reports success without material/expiry persistence in the refresh scope. Active/passive no-change completion therefore preserves cache, LastRefreshAt and expiry; actual persisted refresh completes normally. Other providers retain existing completion behavior.

## R12 per-task metadata policy

StrmTask.MetadataSyncMode, API metadata_sync_mode and migration 0027 accept empty inheritance, cloud_primary, local_primary and bidirectional. New 115 tasks default cloud_primary through the injected local account repository; other drivers retain global inheritance. Task edits accept explicit inheritance, and omitted API fields preserve the previous mode. Default selection introduces no cloud request.

Scanner overrides final ScanDeps.Settings before attachments. scanSettingsForTask applies the same policy to events (W6 integration); W7 attachments consume final settings. Frontend owner added explicit modes and preserves editing values.

## Verification

Targeted tests and race suites passed during integration for domain, auth, account, store, driver, driverexec, API, playback, app and drivers/115. Targeted vet passed. Latest atomic-health race/vet rerun is recorded in the coordinator handoff.

Regression coverage includes:
- Real ephemeral QR handlers: start/poll/cancel, purpose options/echo/isolation, cancelled-session refusal, RFC3339 UTC expiry and no account initialization.
- Opposite credential preservation, stale merges and SQL CAS rejection.
- Degraded/disabled gates and typed failure retry isolation.
- Real Manager/auth callback health-only persistence without refresh/cache mutation; actual replacement still invalidates and records refresh.
- Active/passive pre-expiry no-change completion followed by actual expiry replacement; real 115 fixture independently verifies zero pre-expiry HTTP.
- A barrier forces concurrent Manager health reads at one generation; both independent health failures survive and stale-generation patches cannot alter relogin credentials.
- Another barrier pauses an Open credential refresh after Manager reads, then persists Cookie failure before the credential SQL patch; Cookie health survives while Open credentials/health/expiry update and generation advances. Stale credential patches are rejected.
- Deterministic barriers pause success/failure runtime writes after snapshot load, apply a concurrent health patch, and verify both health and runtime results survive; stale runtime snapshots cannot overwrite reconnect state.
- An expiry-only patch with unchanged secrets/generation is applied while dual runtime completion is paused; both new expiries survive. A SQL regression verifies legacy runtime scheduling still persists expiry.
- Per-task metadata SQL/API roundtrip, defaults, inheritance, edits and invalid values.

No real-account verification was performed by this work package. Offline fixtures do not establish live service stability or eliminate account risk.
