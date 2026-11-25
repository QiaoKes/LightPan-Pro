# 115 implementation acceptance — 2026-11-25

## Result and boundary

Core implementation and offline integration verification have passed on `codex/115-dual-auth-go`. All implementation/review agents used gpt-6.1-sol; the coordinator integrated changes and ran aggregate verification. D9 approves trash-only deletion; D10 explicitly scopes this round to code and offline acceptance, with real-account acceptance deferred. D11 is implemented by W9: default reference character detection and final text decoding are ported to pure Go and verified with pinned Python differential fixtures. User subsequently authorized commit, push and main merge after CI success. Inherited export-name ambiguity, finite differential coverage and distribution-license limits remain visible. Full115 implementation and Docker CI have now been committed and pushed; no actual115 deployment or live-account test was performed.

The primary behavioral source is the restored EmbyNginx artifact and its actual bundled SDK/helpers. The separately cloned p115client checkout is pinned and compared with that artifact, not substituted for it. Runtime code is Go; Python scripts under research are offline evidence tools only. The obsolete `115_Open` implementation and registration have been replaced by `drivers/115` / registration `115`, without a compatibility alias.

## Integrated delivery

| Area | Delivered behavior | Evidence |
|---|---|---|
| Protocol | Cookie encrypted request transport, RSA/AES/LZ4/token vectors, pickcode conversion, SDK/raw-query distinctions | protocol-baseline.md and protocol-vectors; client/cipher wire tests |
| Authentication | Separate Cookie/Open QR, same-UID mount validation, independent runtime health, refresh generation CAS, field-scoped atomic persistence | auth-implementation.md, qr-implementation.md; API/manager/store concurrency tests |
| Driver | Browse/path/file mutations plus retained upload/offline/profile/cross-transfer interfaces; path resolution without component-by-component listings | capability-inventory.md, driver-implementation.md; native-path and driver tests |
| Playback | Open-only media, reference admission/429 waits, path double lookup, 1200s cache, coalescing, UA/intent separation, GET/HEAD 302, STRM queue | playback-implementation.md, playback-parity.md; driver and service traces |
| Emby | Cache before PlaybackInfo/Sessions, effective-device/session predicate, copy-to-root/rename/copied-pickcode link, exact-ID ten-second cleanup and durable known-ID recovery | reference_trace_test.go in driver/playback/embyproxy; migration 0025 |
| Trees | Export/poll/body/delete/parse ordering, staged complete snapshots, mount-absolute paths, explicit incomplete state, confined owned outputs and pending-write recovery | tree-implementation.md; tree/ownership/parser tests |
| Character detection | Pinned charset-normalizer3.5.1 default flow, Unicode15/Python3.12 codec and scoring semantics, CPython bounded sorting, local-only tree decode | charset-implementation.md / charset-final-check.md; 2,690 detector cases, 99 tree cases and component oracles |
| Attachments | Ordinary priority3/FIFO Cookie transfer; special subtitle URLs without injected Cookie; bundled pagination/retry boundaries, root-child streaming with known pickcodes, required monitor escaping and Cookie inverse | events-implementation.md; bundled helper disassembly and request trace fixtures |
| Events | Opt-in per-account polling, task baselines/overlap/checkpoints, restart/reconcile, per-poll directory expansion sharing, invalidation only after output changes | service_events/event_sync tests; migrations 0026/0027 |
| UI | Required dual credentials, separate QR/reconnect/health, monitoring switch, per-task metadata policy defaulting new 115 tasks to cloud-primary | frontend type/build; API policy persistence tests |

Module adapters are explicit optional driver capabilities; generic file/auth/playback/STRM layers do not import the 115 implementation. Credential state persists in store; orchestration remains in services; protocol/request mechanics remain in the driver/client.

## Aggregate verification

All commands below passed after final production changes, except frontend commands which passed earlier and whose sources have not changed since:

```text
go test ./...
go vet ./...
go test -race ./drivers/115/... ./internal/auth ./internal/account ./internal/driver ./internal/core/driverexec ./internal/api ./internal/app ./internal/playback ./internal/embyproxy ./internal/file ./internal/strm ./internal/store
cd web
npm run type-check
npm run build
```

After the frontend build completed, the following pure Go build passed (frontend asset generation and Go embedding must not run concurrently):

```text
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/litepan-115-acceptance-linux-amd64 ./cmd/litepan
```

The final full-scope broad race gate includes charset, codecs and all affected integration packages; see research/completion-audit.md for the source-closure checkpoint. The five subsequently added >10MB oracle fixtures also pass a separate focused race run (110.682s); no production changes followed. `git diff --check` passed. `gofmt -l` reported no files for the affected driver/auth/account/domain/store/API/app/playback/Emby/file/STRM packages. Production source inspection found no old `115_Open` entrypoint or Python subprocess integration. This is a cross-build, not a running-container or real-account smoke test.

## W9 character-detection acceptance

The former three-encoding approximation is removed. All 2,690 independent inputs agree on detected encoding, language, confidence and final Python text-reader output. Same-tag native and pure Python reference results agree. The 61 declaration cases additionally cover exact detector aliases, byte prefilter and invalid normalization boundaries. All 99 independent tree cases agree on leaf paths. Component evidence and source anchors are in research/charset-implementation.md; the independent check is in research/charset-final-check.md.

Five inputs exceeding10MB exercise legacy single-byte and multibyte decoding, including undefined bytes late in the body. Python execution traces prove the500000-byte prefix and suffix-from50000 validation/hard-rejection branches are reached; Go outputs match those observed fallbacks. Model sorting is validated with3000 full-order cases through100 entries; the actual default candidate bound is99. Tests and builds run without Python; embedded immutable tables supply codec/Unicode data. Production embeds exclude oracle fixtures.

Source review caught and fixed detector declaration naming versus codec normalization. Scoring checks fixed operation order, Python decimal rounding and UTF7 lone-surrogate handling. None changes export/download/delete sequencing or adds115/CDN requests. Finite coverage does not establish all conceivable malformed streams or native Linux binary equivalence.

## Local runtime smoke

An isolated host process with a fresh temporary database started successfully, served `/api/health` and the embedded HTML/JS/CSS assets with HTTP200, and accepted bootstrap login. Auth status correctly required the initial password change. It was shut down gracefully. `research/runtime-smoke.json` records the results without session secrets. No mount, password-change UI, actual115 account or container was exercised. The final smoke was rebuilt from current source after D9, D11 and the final Open transport corrections. The process exited with code0 after SIGTERM; its binary SHA256 and successful responses are recorded in the report.

## Request-count and failure assertions

- Service HTTP fixture: twenty alternating GET/HEAD Range requests for the same account/path/UA use one successful downurl. A changed UA obtains a separate URL. Account invalidation causes one new resolution. The fixture rejects server-to-CDN requests.
- Actual 115 driver transport fixture: path playback performs two POST path lookups, then one Open downurl. An Open401 performs no Cookie media request or arbitrary refresh. These actual-driver fixtures and the service twenty-seek fixture are complementary tests, not a single live end-to-end trace.
- Actual copy traces preserve path/numeric input request order, use the already returned copied pickcode and trash only the known copied ID. D9 removes the permanent-delete configuration entirely; ordinary deletion is one exact-ID Open trash call, with no recycle listing or purge. Warm Emby GET/HEAD redirects issue no PlaybackInfo/Sessions request.
- A real registered LocalFs driver can be created, saved, reloaded and listed through the account/manager/auth gate without Cookie/Open credentials or dual-auth state. Invalid file-as-root mounts are rejected before persistence; focused race and aggregate tests pass.
- Same-generation Cookie/Open health failures survive concurrent writes. Credential patches preserve the unsupplied side; runtime bookkeeping preserves dual credential expiries, including expiry-only changes without a generation increment. Legacy auth runtime scheduling remains covered separately.
- Two tasks sharing one directory event share one expansion during that poll; the next poll expands again. Known pickcodes avoid another skim/probe. Out-of-scope/no-op events do not evict playback cache.
- Root hierarchy consumes returned child pickcodes while iterating roots. Missing required `data.has_next_page` fails traversal without retrying that page, matching the bundled stop-check exception boundary. Ordinary request retries retain their separately evidenced predicate.
- Monitor component escaping and inverse follow bundled slash/backslash/dot vectors. A slash-containing decoded component limits the getid shortcut, followed by directory-only `nf=1` listings; ordinary paths retain full-path then parent-path lookups. Subtitle hierarchy uses raw cache rendering rather than monitor escaping.
- A separate generic STRM event fixture verifies escaped playback identity survives publication and URL parsing, owned deletion remains confined, and media events do not add remote queries. Local output names continue the existing filename policy; playback identity is not globally decoded or normalized.
- Incomplete scans, failed event application, output symlink escapes, changed/unknown local files and interrupted publication cannot authorize unrelated file removal.

## Coordinator review and deliberate differences

Independent reviews and coordinator follow-ups fixed request order, copied-pickcode reuse, HEAD cache order, account/mutation invalidation, Cookie-health cache isolation, auth lost updates, absolute mount roots, tree/event ordering, per-task metadata policy, paging/queue behavior, duplicate directory expansion and required monitor path escaping. A concurrent synthetic retry fixture counter was made atomic after the final race run exposed its test-only race; the corrected suite and independent aggregate rerun passed. Detailed findings and resolutions remain in research/integration-audit.md and the area implementation maps.

Approved product differences are explicit: mandatory dual credentials and UID agreement; independent health gates; first-deployment-only registration; owned-output manifests with filesystem confinement; durable event checkpoints/reconciliation and known-copy recovery. These local correctness mechanisms do not authorize extra periodic115 health/CDN probes, per-seek metadata calls or guessed remote cleanup.

D9 explicitly adopts the reference trash-only product behavior. Permanent-delete configuration and the interim trash-plus-partial-error branch have been removed. A successful deletion returns success after moving exactly the requested IDs to trash through one Open call. No recycle-record probing or permanent purge is provided. Independent review confirmed the registration schema, driver call count, mutation invalidation and exact temporary-copy cleanup; driver race/vet and related file/playback tests passed.

## Remaining acceptance and limitations

Per D10, L1–L3 below are deferred follow-up acceptance, not unfinished requirements of this offline delivery. See roadmap.md and runbook.md for independent ownership and prerequisites:

1. L1 real dual QR, cancel/expiry/reconnect, same-UID rejection, restart and real refresh.
2. L2 real players/CDN/Emby devices, twenty seeks and passive control-request accounting. No account risk-control guarantee follows from offline tests or source parity.
3. L3 small live tree/metadata/event scenarios, rename/move/delete and restart while preserving manual/modified outputs.
4. L4 / W9 is closed by D11: the default reference detector and final reader are ported and verified against 2,690 independent Python inputs, 61 focused declaration cases, 3,000 model-order cases and component codec/scoring vectors. This is finite evidence, not exhaustive equivalence or every configurable SDK API. Raw slash-name boundaries lost by the reference export producer remain ambiguous. Required monitor escaping/inverse is implemented; unused generic SDK path options are outside the exercised business scope.
5. L5 is closed by D9: trash-only behavior is delivered and verified offline. Permanent purge is outside the approved product scope.

A remote copy can succeed before its ID is discovered/persisted. The unknown-ID crash window cannot safely be recovered by guessing; known-ID recovery is implemented. Cipher source/header and package metadata disagree on licensing; original attribution/license are retained, and public distribution requires resolving applicable authorization/compatibility. These items must remain visible in release decisions.

No real credentials were used or recorded in this session. No live QR/player/CDN outcome, container execution, account stability or complete SDK capability parity is claimed.

## Final requirement-by-requirement closure

`research/completion-audit.md` maps all R1–R15 / A1–A13 to production boundaries and actual evidence. Four final 6.1-sol reviews covered auth/transport, playback/Emby, trees/events/attachments, and retained capabilities. The final fixes restore Open session headers and sticky UA, admission-before-refresh, 429 expiry re-entry and endpoint-specific response handling. They add no credential/CDN probes or Cookie media fallback. Compression handling preserves existing size semantics.

Additional fixtures cover cold Emby Items/Sessions with the real playback service, warm GET/HEAD reuse, expired-cache original-Emby bypass, post-link ten-second cleanup, surviving queue FIFO/priority, export cancellation/exact3000-second timeout boundary, rename collision preservation, and retained public upload/offline/mutation/profile entry points. Retained upload/offline/full-list/profile implementation bytes match baseline except the package declaration. Multipart/BT outcomes remain live-unverified under D10, rather than being inferred from source identity.

The final full-scope coordinator rerun passed after the Open response/header/order corrections and tree timing seam: all Go tests, vet, broad race and CGO-disabled Linux amd64 build. Fresh current-source host smoke also passed. No production edits followed that gate. The complete requirement matrix and final disposition are in research/completion-audit.md.

## GitHub Docker delivery

User authorized commits/push and conditional main merge. Implementation89fc7c4, initial CI d8ddcb3 and Docker-only simplification9b65537 are committed. Initial full-validation/image run45641655967 passed. Per the superseding user instruction, subsequent CI runs only build/push Docker; run45644010263 passed and published public ghcr.io/qiaokes/lightpan-pro:sha-9b65537 for linux/amd64. Image digest and source metadata are in research/ci-delivery.json. No further Go/frontend verification suites were added to the minimal pipeline. Main merge/push follows this successful feature-branch image build.
