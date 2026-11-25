# W4 driver implementation evidence

2026-11-25. No real-account testing performed.

## Source mapping

- Existing `drivers/115_Open/*.go` moved into `drivers/115/*.go`, package `pan115`, registration `115`. Existing upload/OSS multipart/resume, offline torrent, recycle, file mutations, profile and full-list implementations retained.
- `restored-python/util/wangpan/util_115_open.py:176-195` -> transport.go: process-shared Open admission, response 429 reset header/default 60 plus 5 seconds, re-admission before retry. `util/queue/rate_limit.py:33` permits initial attempt plus 15 one-second retries. Context cancellation interrupts waits.
- `util_115_open.py:99-157` -> transport.go/auth.go: expiry timestamp before now+1 triggers refresh. Removed generic immediate refresh/retry on arbitrary 401. Existing centralized refresh control is retained.
- `util_115_open.py:197-215,241-255` -> ops.go: path lookup uses POST path, missing item waits 30 seconds and tries again, normal path playback performs both get_item calls before downurl.
- SDK `p115client/client.py:21390-21410` -> driver.go Ping: GET my.115.com with ct=guide/ac=status is live Cookie validation; success state is required. Cookie UID is parsed only after validation; GET proapi /open/user/info yields Open UID, compared before healthy state persistence. This initial dual check implements PRD R3, an approved adaptation.

## Ordered traces

- Initial connection: require Cookie/access/refresh -> GET Cookie guide/status -> reject false/malformed -> parse Cookie UID -> Open bucket -> GET /open/user/info Bearer token -> compare UID -> publish both active health states for account service persistence. Cookie UID alone never establishes validity.
- Runtime Init: construct HTTP clients -> restore tokens; no Cookie validation probe. Failed Cookie health does not block Open operations.
- Path media cache miss: POST /open/folder/get_info {path}; if empty wait 30s and repeat -> second get_item with same missing-data policy -> POST /open/ufile/downurl {pick_code}, caller UA. Every Open business request acquires shared bucket. No Cookie media fallback.
- HTTP 429: consume response -> wait X-RateLimit-Reset+5 (default65) -> acquire shared bucket -> repeat identical request, unbounded as reference. Cancellation exits.
- Expired timestamp: centralized refresh -> update returned token/refresh/expiry -> conditional persistence generation when supplied -> business request. Arbitrary 401 does not trigger this refresh path.

## Validation and limits

`go test ./drivers/115` and `go test -race ./drivers/115` pass. Fixtures prove live Cookie failure prevents Open probe, UID mismatch rejection, matching health, cancellation and independent capability gating.

Other owners supply QR sessions, protocol client, tree/events and temporary-copy adapters. Real QR, identity responses, player testing and full integration remain required. Exact legacy upload/offline request parity beyond preservation was not re-established here. Runtime Open authentication errors persist capability-specific health with the request generation. Shared framework restores health and ExpectedGeneration.

## W4 completion: reconnect and generation

`ValidateCredentialUpdate(ctx, previous)` probes only a changed credential side, compares against retained verified UID even when retained status is expired, and preserves retained health. A missing verified identity causes live validation rather than trusting unverified data. First creation still calls Ping and validates both sides online. UID numeric strings are canonicalized.

Refresh captures access/refresh/generation atomically before HTTP. A changed generation or token causes the late result to be discarded, even when refresh token text is unchanged. Conditional persistence precedes local replacement; persistence errors leave local tokens intact. Successful secret changes advance local generation exactly once; unchanged token refresh does not advance it. A second stale check prevents concurrent reinjection from being overwritten after persistence. Open failure health updates also retain the request's captured generation and cannot poison a newer local credential snapshot.

Additional fixtures cover Cookie-only and Open-only reconnect with the opposite expired side, mismatched reconnect identities, same-refresh-token relogin races, persistence failure, and generation advancement across sequential refreshes. `go test ./drivers/115/...`, race suite and vet are the final package gates; no live-service claims are made.

## Path-reference integration

`drivers/115/path_resolver.go` implements the optional generic path adapter. A tree task supplies its absolute cloud root path; the adapter joins the relative media path and returns an explicit `domain.IDPath` reference without HTTP. This prevents generic per-component directory listings before the already-preserved reference `download_url_path` double lookup. Numeric root identifiers are rejected rather than mapped speculatively. File service owns relative-path validation; this adapter preserves cloud path semantics and honors cancellation. Fixtures assert zero HTTP for root/subtree/Unicode paths and reject numeric root guessing.

## Runtime Cookie errors and mounted tree roots

Cookie protocol boundaries use `cookieErrorHandler()` captured before I/O. `client.KindOf` marks only confirmed AuthExpired as Cookie token_expired, and VerificationRequired as Cookie failed with a safe verification message. Permission errors, ordinary upstream errors and timeouts do not imply expiration. Persistence carries the captured ExpectedGeneration, and late responses cannot poison a newer Cookie login. Open remains untouched. W6/W7 own applying this boundary to their protocol calls; raw Request callers explicitly use CheckResponse.

`absoluteTreeRoot` maps a mount-relative task path to a cloud-absolute export root. Mount 0 requires no mapping request; a nonzero mount performs one Open GET `/open/folder/get_info?file_id=<mount>` per scan and builds the absolute path from returned Paths and name. Missing path evidence is rejected. The generated playback path root is retained, so seeks do not re-query the mount. This is an architecture mapping step, not a change to reference media double lookups.

115 Config now uses RefreshAtExpiry, and RefreshAuth skips wire refresh before the known expiry's reference now+1-second threshold. Refresh authentication errors are typed Open capability errors, preventing generic whole-account retry/failure. Tests assert early refresh produces no HTTP, capability isolation, late Cookie-error rejection, and zero/one mount-mapping request. Final `go test -race ./drivers/115/...` and `go vet ./drivers/115/...` passed after these additions.

## Cache TTL alignment and permanent-delete safety

Reference `router/strm/strm.py:36,219` calls the shared `query_direct_cache`; `router/emby_nginx/emby_nginx.py:delete_invalid_cache` removes records older than 1200 seconds. The former Go driver link Expiration was five minutes, causing ordinary STRM cache misses earlier than this reference. `downloadLinkTTL` now equals 1200 seconds only for 115. A ResolveDownload fixture verifies the actual returned link lifetime.

Necessary safety correction (coordinator requested): legacy `permanentDelete` first trashed requested source IDs, then selected the newest N recycle TIDs by deletion time. Concurrent deletions by another client could make unrelated records newer and permanently purge those files. Names/time/order do not prove original-file ownership. Local pinned SDK `p115client/client.py:4171-4195` documents Open `/open/rb/list` limit/offset but does not establish a trustworthy original-ID-to-recycle-TID response association; no verified response fixture was available. Therefore no guessed fields or newest-N fallback were added. Permanent-delete mode still trashes exactly the requested IDs, then returns an explicit partial-outcome error saying permanent purge was not performed and manual verification is required. The new fixture asserts no recycle-list/purge request occurs. Reliable purge may be restored only after original-ID association is evidenced and tested against unrelated/concurrent recycle entries.

## L5 bounded permanent-purge evidence review

Additional local evidence inspected (no remote probes):

- Pinned SDK `p115client/client.py:4118-4147`: Open purge accepts `tid` as comma-separated recycle-record IDs. Omitting tid means clear the bin; therefore neither empty nor guessed TIDs may be submitted.
- SDK `:4171-4195`: Open recycle list exposes limit/offset inputs and returns the raw response. It does not document or normalize an original file ID field or its relationship to TID.
- SDK Cookie `:25565-25652`: rb_info takes rid; list filters/sorts support aid/cid/dtime/file_size. These inputs do not prove a record's original source identity.
- SDK pending recycle wrapper `modules/@p115/p115/@pending/recyclebin.py:335-365`: permanent remove accepts explicit recycle IDs, passes them to recyclebin_clean; no conversion from original source file IDs is implemented.
- Reconstructed business `util/wangpan/util_115_open.py:279-288`: delete only POSTs `/open/ufile/delete` with file_ids. No permanent-purge mapping exists in that reference branch.
- Reconstructed Cookie monitor `util/wangpan/util_115_monitor.py:272-284` matches file_name and dtime to reconstruct a parent path from cid. This weak association supports its local event behavior but cannot authorize irreversible remote purge.
- Available synthetic boundaries `image-analysis/tools/compare_monitor.py:131-134` and `smoke_115_sync.py:86-89` return only file_name/cid/dtime, with no original-ID/TID pair. They are synthetic Cookie responses, not verified Open recycle evidence.

Exact missing contract: a trustworthy Open recycle response field carrying the original source file ID, the separate purge TID field, and documented uniqueness/retention semantics linking the pair. Current fixtures cannot establish that contract. A name, deletion timestamp, parent cid, list position, or response-map key is not an acceptable substitute.

Permanent purge remains incomplete under A12. Safe product options reported to coordinator: remove the permanent choice while retaining trash; expose it as unavailable and reject before any mutation; or explicitly label the existing trash-plus-partial-error behavior (which still does not complete permanent purge). No implementation change was made during this evidence-only follow-up, and no capability-complete claim is made.

## D9 final disposition — 2026-11-25

The user explicitly selected reference trash-only behavior. The preceding partial-error implementation and L5 capability gap are historical evidence, superseded by D9. Removed DeleteMode, the permanent helper/branch and unused recycle endpoint models/constants. DeleteFiles moves exactly the requested IDs through one Open `/open/ufile/delete` call and returns success on success. Tests verify no deletion option in registration/config, exact IDs, no recycle list/purge and preserved cache invalidation. Independent review passed driver race/vet and related file/playback tests. Permanent purge is outside the approved product scope.
