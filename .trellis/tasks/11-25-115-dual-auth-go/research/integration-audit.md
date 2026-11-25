# Playback integration audit (2026-11-25)

Snapshot audit while workers are editing. Paths below are absolute relative to `/Users/saki/Desktop/code/ebn/LitePan` unless explicitly reference Python. No live credentials used.

## Executable remaining gaps

1. Temporary-copy downurl must consume the known pickcode. `internal/playback/temporary.go:104` passes only copy.FileID; `drivers/115/temporary_playback.go:36` obtains PickCode but does not populate the driver lookup cache. `drivers/115/ops.go:50` therefore GETs file-info before downurl. Reference `/Users/saki/Desktop/code/ebn/restored-python/router/emby_nginx/emby_nginx.py:349` calls get_115_url_by_pickcode directly with copy_info.pick_code. Add an HTTP trace test that asserts copy/root lookup/rename/downurl without an intervening GET of copied ID.
2. Numeric-source copy performs GET before POST copy (`drivers/115/temporary_playback.go:20,29`). Reference `/Users/saki/Desktop/code/ebn/restored-python/util/wangpan/util_115_open.py:375-401` does POST copy before optional source-name GET for numeric input. Preserve ordering or document the applicable caller only ever passes paths; add numeric-ID trace coverage.
3. Correction: ID file mutation invalidation **is wired**: `internal/file/service.go:507` synchronously calls cache.ApplyMutation; `internal/cache/cleaner.go:32,76` invalidates download prefixes for event file IDs. Remaining gap is path-keyed playback URLs (including directory descendant paths), because an event ID does not match their cache prefix. Emby direct cache also needs generation/invalidation integration. Test through actual file mutation service, then resolve cached path and inspect new downurl. W5 notified directly.


## Confirmed current behavior

- Ordinary Resolve cache occurs before upstream (`internal/playback/service.go:128-143`), including path identities, so sequential seek resolves with same account/path/UA/playback variant need only one downurl until expiry. No dedicated 20-seek HTTP count test was located; add one as integration acceptance, including different-UA isolation.
- Path downurl explicitly performs two path lookups (`drivers/115/ops.go:40-47`). Missing path metadata waits 30s then retries once (`:314-335`), corresponding to Python util_115_open.py:199-218.
- Temporary cleanup uses `trashFiles` directly (`drivers/115/temporary_playback.go:50`), independent of user permanent-delete mode. This avoids permanent-delete configuration affecting temporary cleanup.
- Current ledger persists copy ownership before link request, retries failed cleanup, and sets 10s due after link attempt (`internal/playback/temporary.go:82-114`). Copy creation can still fail after remote copy but before owned ID is discovered, leaving no recoverable ID; reference has same discovery hazard, so any improvement requires explicit parity treatment.
- HTTP 429 sleeps reset+5 and re-enters limiter (`drivers/115/transport.go:139-150`), matching reference util_115_open.py:186-189 recursive retry; cancellation is supported.

## Verification

`go test ./internal/playback ./drivers/115 ./internal/embyproxy` passed (cached). These package tests do not establish complete real-service parity. No real HTTP credentials or upstream requests were used.

Findings 1–3 sent to root for corresponding owner routing. This audit does not cover directory trees/events (separate audit), nor claim exhaustive bundled SDK compatibility.

## Additional real HTTP fixture trace

Added exclusive `internal/playback/reference_trace_test.go`. `go test ./internal/playback -run TestReferenceTrace -v` passes. It exercises the actual Service.ServeHTTP with Executor, cache and an HTTP-backed fixture driver (not the 115 production protocol client):

- 20 alternating GET/HEAD requests with increasing Range, UA player-a: `POST /open/ufile/downurl UA=player-a` once.
- 20 more with player-b: one additional `POST /open/ufile/downurl UA=player-b`.
- Account invalidation then 20 player-a requests: one additional downurl.
- Every request returns 302 with expected CDN Location; fixture rejects any CDN HTTP request, proving this path does not use server range proxy.

This verifies generic playback caching and direct invalidation, not credential-update event wiring, mutation wiring, actual 115 endpoint behavior, temporary-copy order, or Open/Cookie protocol isolation. Those need separate integration fixtures.

## Worker remediation reinspection

W5 revised production implementation during audit. Re-read confirms temporary copy now `rememberPickCode(copied)` before subsequent ResolveDownload; numeric source POST copy precedes necessary source-name GET. File service publishMutation now calls playbackInvalidator for delete/move/rename, and `internal/app/wire_services.go:69` wires it to playbackSvc.InvalidateAccount. Emby key includes playback CacheGeneration. Thus findings 1–3 above describe the earlier snapshot and have production fixes; full actual 115 HTTP trace regression is still desirable.

Added `internal/embyproxy/reference_trace_test.go`: a warm direct-cache entry precedes all Emby upstream HTTP for both GET and HEAD Range requests. An initial HEAD run exposed unconditional upstream HEAD forwarding; reported to W5, who removed that path. Rerun passes, with upstream HTTP trace empty for GET and HEAD and expected CDN 302.

Added `TestReferenceTraceCleanupRecovery`: recreated playback service receives pending copy ownership, invokes delete only for exact `copy-only` ID, removes ledger after success, retains LastError and future Due after failure. This uses a fixture repository; the store restart persistence test belongs to W5.

Final targeted rerun: `go test ./internal/playback ./internal/embyproxy -run TestReferenceTrace -v` PASS. Actual 115 protocol transport fixture for temporary-copy/downurl ordering, Open-error/no-Cookie and credential-update-to-generation invalidation remain outside these new test fixtures; generic tests must not be represented as proving those protocols.

## Actual 115 driver transport fixture (final)

Added exclusive `drivers/115/reference_trace_test.go`, using real Driver.ResolveDownload/CreateTemporaryPlaybackCopy/DeleteTemporaryPlaybackCopy and intercepting d.client transport at fixed `proapi.115.com`. It asserts bearer identity, absence of Cookie headers, exact form/query values, endpoint sequence, final URL decoding, and caller downurl UA. Production process-shared gate is preserved (no clock/reset override).

`go test ./drivers/115 -run TestReferenceWire -v` PASS (15.4s).

Observed path download trace:
```
POST /open/folder/get_info path=%2Fmovie.mkv
POST /open/folder/get_info path=%2Fmovie.mkv
POST /open/ufile/downurl pick_code=source-pick
```

Observed Open HTTP 401 trace (error returned, no token refresh or Cookie host/request):
```
POST /open/ufile/downurl pick_code=source-pick
```

Observed temporary copy for path source:
```
POST /open/folder/get_info path=%2Fmovie.mkv
POST /open/ufile/copy file_id=10&pid=0
POST /open/folder/get_info path=%2Fmovie.mkv
POST /open/ufile/update file_id=20&file_name=timestamp_movie.mkv
POST /open/ufile/downurl pick_code=copy-pick
POST /open/ufile/delete file_ids=20
```

Observed numeric source substitutes first two requests with:
```
POST /open/ufile/copy file_id=10&pid=0
GET /open/folder/get_info file_id=10
```
Remaining root lookup/rename/downurl/delete are identical. No GET of copied ID occurs. Tests explicitly set permanent delete configuration and still see only exact copied ID trash, with no recycle-list or permanent purge requests. These actual driver traces close earlier fixture gaps for copy order, pickcode use, exact trash and Open401 no fallback. They complement generic 20seek HTTP tests; they do not claim a single end-to-end actual-driver 20seek fixture, live-service validity, all auth expiry codes or full shared credential event propagation coverage.

## Final shared-layer acceptance audit

Re-read callable wiring (not protocol trace repetition):

- `internal/app/wire_services.go` configures file mutation and auth credential invalidators before serving; account update/toggle also invalidates. `internal/auth/service.go:74` installs manager persistence hook; refresh_runner invalidates synchronously before return. Emby direct slots carry accountID and check account state before a warm redirect; generation invalidates old slots.
- Added NEW `internal/playback/crosslayer_disabled_acceptance_test.go`: warms the actual Service/Executor/cache using HTTP-backed fixture driver, flips gate disabled, then retries the identical cached identity. It returns an error without Location, without another HTTP request and without generic refresh. `go test ./internal/playback -run TestCrosslayer -v` PASS.
- STRM application wiring passes Accounts, EventStore, Files and Playback (`internal/app/wire_strm.go`). Service.Start launches monitor when eventStore exists. Monitoring defaults false, requires active task plus completed baseline; grouped account read occurs through file.Executor, so disabled-account gate applies. NeedsReconcile runs full scan outside task lock; Poll rechecks durable state under lock. Incomplete/error/gap batches cannot advance cursor or imply deletion. Failed output stores reconcile requirement before returning; receipts/cursor saved after output.
- Metadata mode reaches API DTO -> task -> repository -> taskScanSettings -> event/full-scan paths. API update distinguishes omitted field from explicit empty inheritance (`internal/api/strm_admin.go:164,734`). Service create defaults new 115 tasks cloud_primary while preserving explicit local/bidirectional; updates allow empty inheritance. Store migration 0027 persists field. Existing API/store/service tests cover these boundaries.
- Ran `go test ./internal/auth ./internal/api ./internal/strm ./internal/file ./internal/account ./internal/app`: PASS. No newly identified severe wiring gap in this audit snapshot.
- Production Go/source/build scan found no old `115_Open` reference or subprocess Python invocation. Only Python references found in relevant production driver files are source provenance comments. This is code/build inspection, not a container smoke run.

No production files changed by this audit. W7 attachment protocol finishing remains its owner's active work and is not certified by these shared-layer checks.

## Final delta review: atomic auth, per-round expansion, streaming subtitles

Read backend index and `115-reference-contract.md`; `get_context.py --mode packages` reports single repo/no packages, backend layer only. Other backend quality index entries remain generic placeholders; executable 115 contract requirements apply: generation-safe writes, health independence, exact protocol ordering, per-account events, metadata persistence, fixtures/race checks and no secret logging.

Root identified a post-audit defect: health-only CookieAuth persistence invoked generic refresh success/invalidation, causing a valid Open link to be fetched again. Reinspection of current fix confirms Manager.persistAuth now returns directly through CredentialHealthPatcher for unchanged auth material, without onAuthPersist; Gate.HandleCapabilityError invalidates only Open failures. Atomic SQL json_set updates only supplied health side with credential-generation CAS. AuthCredentialPatcher merges supplied secrets/health against SQL state, preserving concurrent unsupplied health, and bumps generation only for changed secret values. AuthRuntimeUpdater excludes health and secrets from stale runtime bookkeeping updates.

One remaining edge was sent to root for classification/fix: auth_repo.go runtime-only assignment still includes token_expires/cookie_expires, while expiry-only credential patches do not bump generation. A stale same-generation runtime write could replace a concurrent expiry-only update. No production edit made without coordinating W2.

W6 reinspection: directory expansion cache is created once inside each account poll loop, keyed by file ID, and passed via round context to task Poll calls; failures also cached for that round. It is neither cross-account nor cross-poll. Actual output changes trigger a single coalesced external invalidation after task applications; ignored events do not trigger it.

W7 reinspection: root subtitle listing handles each root page callback synchronously and consumes provided pc via loadDirs before requesting next root page; no known-pickcode skim. OS download node parser now explicitly rejects missing has_next_page before deciding pagination completion. Existing tests include streamed root interleaving and stable-point non-probing.

Targeted delta checks passed: `go test ./internal/driver ./internal/auth ./internal/store ./internal/strm ./drivers/115/client -run 'Health|Runtime|Credential|Directory|Root|Subtitle'`. Root owns final aggregate/race validation. No new live request used.

Final expiry-race resolution: W2 changed runtime-only SQL assignments to inspect current capabilities_json within the UPDATE. Current dual-health rows retain existing token/cookie expiries; legacy rows without health retain generic expiry scheduling. Re-read exact CASE expressions confirms predicate is evaluated against SQL state, not stale Go snapshot. Independent rerun `go test ./internal/auth -run TestRuntimeBookkeepingPreservesExpiryOnlyCredentialPatch -count=1` PASS. This closes the final audit edge noted above; no remaining blocker identified by this delta review (root aggregate validation still authoritative).

## Bounded R15 monitor path grammar delta

Reviewed only new path grammar integration: FolderPath applies monitor component escaping after upstream name decoding; raw cache nodes remain raw, with escaped rendering kept distinct. DirectoryEventFiles emits escaped directory and leaf components. AttachmentID now splits/inverts monitor grammar before choosing direct getid versus per-component enumeration, including embedded slash components. No global monitor unescape was introduced in generic STRM publication.

Added exclusive `internal/strm/crosslayer_monitor_path_acceptance_test.go`. Three provider-emitted paths containing escaped backslashes, escaped dot/dotdot, embedded slash escaping and Unicode pass generic applyLifeEvent -> confined publication -> parsed STRM reference -> owned deletion. Root/RelativePath bytes round-trip unchanged in URL identity; existing SafeName policy still sanitizes local filename characters. A nil remote file service ensures media event publication makes no extra remote identity request. os.Root publication and safeOwnedPath checks remain in place.

`go test ./internal/strm -run TestCrosslayerMonitor -count=1 -v` PASS. `go test ./drivers/115/client -run 'EscapedAttachment|FolderPathEscapes|DirectorySkim|MonitorPath' -count=1` PASS. No new cross-layer blocker found, no production changes or live requests made.

## Final D9/D10 review — 2026-11-25

An independent trellis-check agent reviewed the trash-only implementation with no findings or code changes. Registration schema and Addition expose no delete mode; runtime ops/models/transport contain no permanent/recycle helper. Ordinary DeleteFiles issues one exact-ID Open trash call and clears successful pickcode cache entries; generic file mutation invalidation remains intact. Temporary-copy cleanup retains exact copied ID and ordered known-pickcode trace. Driver race/vet and internal/file/internal/playback tests passed. Historical permanent-mode fixture descriptions above are superseded by D9; that option and interim partial-error branch are now removed. D10 explicitly defers live-account acceptance; no synthetic test is claimed as a live pass.
