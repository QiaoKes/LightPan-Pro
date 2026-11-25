# 115 reference integration contract

## 1. Scope / trigger

Applies to `drivers/115`, dual credential authentication, playback, Emby interception, and tree-based STRM synchronization. The business reference is the locally restored EmbyNginx artifact, not an arbitrary latest SDK. Source mappings, bundled vectors and ordered traces live in `.trellis/tasks/11-25-115-dual-auth-go/research/`.

Preserve evidenced endpoint, query, form, encoding, UA, request order, cache order, wait and retry behavior. An apparently redundant request is not authorization to remove it. Do not introduce extra credential probes, CDN probes, per-leaf listings or fallback credentials. Python is research input only; production is Go.

## 2. Signatures

- Registration: `115`; implementation package `pan115`; protocol package `pan115client`. No `115_Open` compatibility alias.
- `PurposeQRLoginProvider`: `StartQRLoginFor(context.Context, QRLoginOptions)`, `PollQRLoginFor(context.Context, token, purpose)`, `CancelQRLogin(context.Context, token, purpose)`.
- `TreeSnapshotProvider.ExportTree(context.Context, TreeRequest, func(TreeEntry) error) (TreeSnapshot, error)`.
- `LifeEventProvider.ReadLifeEvents(context.Context, LifeRequest) (LifeBatch, error)`; optional `DirectoryEventProvider.ExpandDirectoryEvents(context.Context, string)`.
- `STRMDownloader.ResolveSTRMDownload(context.Context, DownloadRequest)` applies only to marked STRM requests after cache/coalescing.
- `TemporaryPlaybackCopier` creates/deletes exact owned copies; `TrackedTemporaryPlaybackCopier` persists discovered identity before rename.
- `CredentialHealthPatcher.PatchCredentialHealth` changes only supplied health sides at the expected credential generation. `AuthCredentialPatcher.PatchAuthCredentials` atomically changes supplied credential fields; `AuthRuntimeUpdater.UpsertAuthRuntime` updates scheduling/runtime fields without replacing credentials or health.
- Migration 0025: `temporary_playback_copies`; 0026: `monitoring_enabled` and `strm_event_sync`; 0027: task `metadata_sync_mode`.

## 3. Contracts

### Credentials and QR

Mount creation requires Cookie, access token and refresh token. Live Cookie status and Open user identity must establish the same UID. Runtime health is independent: `cookie_api` and `open_api`, each with status, verified UID and safe last error. Reconnect validates only the changed side against verified retained identity. Conditional updates carry `ExpectedGeneration`; late results must not overwrite newer credentials.

Generation checks alone do not prevent two health updates at the same generation from losing one side. Use field-scoped atomic repository patches, not read/replace of the entire state. Runtime bookkeeping must not write secrets or health. A credential patch must preserve a concurrent health update for the unsupplied side and increment generation only for changed secrets.

QR start/poll/cancel bodies include `driver_type`, `purpose` (`cookie` or `open`) and opaque `token` where applicable; start accepts `app`/`client_id`. Poll expiry fields use RFC3339. Do not invent an upstream TTL. Preserve waiting/scanned/success/cancelled/expired states and transient-error recovery. Never log secret-bearing payloads.

### Playback and protocol

Media uses Open only. Shared Open business admission is one request/second, burst one, with reference admission retry bounds. After local capability guards, admission precedes expiry refresh and the business request. HTTP 429 waits reset-header/default60 plus5 seconds and recursively re-enters admission and expiry processing until cancellation; do not reuse a token captured before that wait. Timestamp expiry, not arbitrary 401, triggers refresh. Path download preserves both reference path lookups and their missing-data retry.

Reference Open session defaults are UA `EmbyNginx`, Accept `*/*`, Accept-Encoding `gzip, deflate`, form Content-Type even on GET, and keep-alive. No browser Accept-Language is added. Download changes the account-local session UA before expiry checking; subsequent ordinary calls and refresh inherit it. Fresh Open QR requests use the Open defaults; Cookie QR retains its own source contract. Decode explicit gzip/zlib/raw-deflate responses without double-decoding transport-expanded bodies. Preserve existing body-size semantics; adding compression does not authorize a new Cookie JSON size cap.

Response handling is endpoint-specific. File-info, downurl, user-info and listing consume returned data even when state is false. Missing file-info data still gets its single reference 30-second retry. Delete ignores the JSON success state after a successful HTTP response and does not invent a deletion retry. Copy, rename and create-folder retain explicit state checks. Refresh accepts usable data regardless of state type/value or nonzero code, as the reference does; invalid/missing credentials still fail validation. Do not apply one generic state predicate to every endpoint.

115 URL cache lifetime is 1200 seconds. Cache lookup precedes metadata, queue, PlaybackInfo and Sessions work. Cache identity separates account, UA, playback intent and Emby effective device/source; GET/HEAD/Range offsets do not create new ordinary URL identities. File mutations, external verified events and credential changes invalidate playback generations. Disabled accounts cannot use cached cloud redirects. A Cookie health-only update is not a credential refresh: persist health without advancing LastRefreshAt or invalidating valid Open playback. Likewise a pre-expiry RefreshAtExpiry no-op must not be recorded as a real refresh.

STRM `Lavf` cache misses use two workers, priority2/FIFO and reference one-second result polling. Emby requests do not enter this queue. Preserve exact session predicate, copy-to-root, rename, copied pickcode downurl and ten-second cleanup after link attempt. Cleanup uses only known copied IDs. D9 removes permanent-delete configuration; ordinary deletion also uses exact-ID trash only. Persist ownership before subsequent mutation. The remote-copy-to-ID-discovery crash window cannot be recovered by guessing an ID.

### Trees, events and local output

An export tree has paths, not fabricated stable IDs. Retain a cloud-absolute root for generated playback references; mount-root mapping is per snapshot, never per seek. Stage entries until the snapshot is complete. Export object removal follows successful body download and precedes parse, as reference.

Output directories are dedicated and nonoverlapping. Inventory fingerprints grant ownership only to unchanged generated files. Use `os.Root` for confined reads, publication and deletion; path validation alone is not filesystem confinement. Publish intent before output so interrupted writes can be recovered. Preserve unknown or modified files. Failed/incomplete scans cannot authorize missing-file deletion.

Monitoring defaults off. One source fetch per account per 30-second tick is dispatched to tasks. A complete scan records its start time and absolute root as baseline. Persist per-task overlap receipts, cursor/watermark and reconciliation requirement; advance progress only after successful application, including empty complete batches. Root/account changes invalidate baseline. Directory events use the verified expansion provider. Same-target rename must not remove the newly written STRM.

The reference uses separate transfer mechanisms: ordinary Cookie attachments have one shared priority3/FIFO worker; special subtitle returned URLs use ten workers without injected Cookie. Directory nodes and offset listings use their bundled pagination schedulers, not an invented generic retry or limiter. Known pickcodes must be consumed directly rather than repeating a skim lookup.

Monitor `get_path` and skim traversal use the bundled component escape: prepend backslash to standalone dot/dotdot; otherwise double backslashes, then escape component slashes. Cookie path resolution decodes components before choosing the getid prefix and listing any remaining escaped-slash components; do not use `path.Clean` on that representation. Subtitle traversal explicitly uses raw names (`escape=None`). Ordinary tree export also joins raw names and cannot recover slash-name boundaries that its producer discarded. Preserve these distinct representations instead of applying a global normalizer.

Tasks processing the same directory event share expansion results only within the current account poll; later polls fetch anew. Invalidate playback once per account/poll only after an owned output actually changes. No-op or out-of-scope events must not discard useful cached media URLs.

`metadata_sync_mode` accepts `cloud_primary`, `local_primary`, `bidirectional`, or empty inheritance. New 115 tasks default to cloud-primary; other drivers inherit existing global behavior. Omitted update fields retain stored values. Scanner and event handlers use the same task-effective policy.

## 4. Validation and error matrix

| Condition | Required behavior |
|---|---|
| Missing credential / UID mismatch | Reject usable mount creation |
| Cookie expired, Open healthy | Pause Cookie operations; retain Open playback |
| Open expired, Cookie healthy | Block Open operations; retain independent Cookie operations |
| Timeout / permission failure | Do not mark both credentials expired |
| Stale credential generation | Reject/discard late update |
| Open media failure | Return failure or existing original-Emby bypass; no Cookie media fallback |
| Incomplete export / unverified event / apply failure | Preserve output; require reconciliation; no false checkpoint |
| Output overlap / traversal / symlink escape | Reject before publication or deletion |
| Unknown or modified local file | Retain without claiming ownership |
| 115 deletion (user decision D9) | Only exact-ID Open trash, one call; no permanent-delete option, recycle probe, purge or trash-plus-partial-error branch |

## 5. Good / base / bad cases

- Good: 20 GET/HEAD Range seeks with one account/path/UA and unchanged valid cache use one downurl, no CDN proxy and no Cookie request.
- Base: a new UA obtains its own URL; a complete snapshot reconciles only its unchanged owned outputs.
- Bad: invalidating every seek, listing each path component before download, refreshing on any401, or selecting newest recycle entries to purge.

## 6. Required tests

- Bundled RSA/AES/LZ4/token and encrypted request vectors; raw QR versus SDK-default query distinction.
- Initial UID validation, independent health, reconnect, generation race and API expiry/state serialization.
- Ordered actual-driver HTTP traces for two path lookups, Open401 isolation, temporary-copy order and exact-ID trash.
- Cache-first 20seek, concurrent miss coalescing, GET/HEAD, UA/device separation, invalidation and queue cancellation.
- Export timeout/cancellation/incompleteness, absolute mount root, Unicode paths, owned-only cleanup, symlink confinement and crash recovery.
- Per-account event fetch grouping, empty-batch watermark, replay/restart, failed-task isolation, scan/event ordering and same-target rename.
- Metadata policy persistence, default/inheritance and attachment path/directory/subtitle wire fixtures.

Offline fixtures establish these assertions only. Live QR, refresh, CDN/player Range behavior and account risk-control outcomes require separate real-account acceptance.

## 7. Directory-tree character detection

The tree reader calls `drivers/115/client/charset.Decode` once after acquiring the export body. Follow the bundled charset-normalizer 3.5.1 default `legacy.detect` / `from_bytes` algorithm and final Python text-open behavior: strict candidate probing, ignore errors on final decode, universal newlines, and UTF8 locale fallback when no encoding is detected. Do not replace it with a UTF8/GB18030 shortlist or an unrelated detector.

Keep Python detector names separate from Python codec lookup names. Declaration lookup uses the exact pinned `_IANA_NAMES` table and lower/hyphen replacement, including the byte-level coding/charset prefilter. Codec lookup normalization alone is not equivalent. Preserve candidate order, BOM rules, similar-codec skips, scoring thresholds, fallback and early-return boundaries.

Unicode properties, language profiles, codec mappings and rounding follow pinned CPython 3.12.13 / Unicode 15.0.0 evidence. Go Unicode properties, ordinary rune iteration over WTF8 surrogates, generic decimal rounding and Go stable sort are not interchangeable. The model comparator is nontransitive; the local CPython sorter is safe only for the default candidate bound: 99 entries contain 98 distinct names, signatures/ASCII/UTF8 are already included, and one declaration adds at most one, so at most 99 results reach sorting. Extending the candidate API requires revisiting that proof.

At 10,000,000 bytes, preserve the reference lazy single-byte validation and payload-release behavior; avoid retaining a decoded copy for every candidate. Generated immutable mapping assets belong in runtime embeds; differential oracle fixtures do not. Python generators/interpreters remain offline research tools and must never be called by runtime code or Go tests/builds.

Required verification: pinned observed detector/text-reader output, legacy/stateful/malformed/declaration cases, model-order vectors, component scoring/codecs, large-input boundaries, tree leaf paths and existing export request traces. Test observed Python output even when it differs from the intended source encoding. Finite differential coverage is not exhaustive equivalence or an account-stability guarantee.

## 8. Wrong versus correct

```go
// Wrong: treating every player seek as a new control-plane transaction.
probeCredentials(ctx)
listAllParents(ctx, mediaPath)
resolveDownurl(ctx, mediaPath)

// Correct: honor the existing playback cache and provider request contract.
if link, ok := cacheLookup(intent); ok {
    return link, nil
}
return resolveCoalesced(ctx, intent)
```
