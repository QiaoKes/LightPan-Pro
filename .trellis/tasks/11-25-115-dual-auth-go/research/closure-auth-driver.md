# Closure audit: auth and retained driver capabilities

2026-11-25. Independent current-source audit. No live calls or credentials. Initial findings below are historical; the final source-alignment sections record the corrections and supersede them.

## Requirement → source → verification → result

| Requirement | Current source | Exact offline evidence | Result / limits |
|---|---|---|---|
| R1/A1 replacement | drivers/115/driver.go; drivers/all.go | package builds in `go test ./drivers/115` | Registration 115 retained; deployment/Linux evidence supplied by coordinator, not independently repeated here. |
| R2 pure Go | drivers/115/{auth,transport,upload,offline_download}.go | targeted Go packages compile/test | Inspected implementations are Go; exhaustive dependency/build audit outside this review. |
| R3/A2 dual credentials and UID | drivers/115/driver.go:ValidateCredentialUpdate | drivers/115/driver_test.go:TestPingRequiresLiveCookieAndMatchingOpenIdentity; TestReconnectRejectsOtherIdentity | PASS fixtures; Cookie status then Open identity, rejected Cookie stops before Open. |
| R4/A2 lifecycle | drivers/115/{qr,auth,auth_sessions}.go | qr_test.go:TestQRSurvivesDriverRecreationAndConsumesExchange, TestQRCancelRemovesSession, TestQRUpstreamStatusesAndTransientFailure; driver_test.go:TestReconnectProbesOnlyChangedSide, TestRefreshDiscardsChangedGenerationWithSameRefreshToken, TestRefreshPersistenceFailureKeepsLocalCredentials, TestRefreshAdvancesGenerationForNextPersistence | PASS. QR actual service deferred D10. |
| R4 API and UI | internal/api/qr_purpose_test.go; web/src/components/admin/AddAccountDialog.vue | TestQRPurposeHandlersWithoutAccountInitialization; `npm run type-check` | PASS static/API; UI merges returned fields into existing form and distinguishes purpose. No browser interaction test performed. |
| R9/A7 independent health | drivers/115/cookie_health.go; internal/auth | cookie_health_test.go:TestCookieAuthFailureIsolation, TestLateCookieFailureDoesNotInvalidateRelogin; internal/auth/capability_retry_test.go:TestDegradedCapabilityGate, TestDisabledAccountBlocksDegradedGate, TestCapabilityFailureDoesNotRetryWholeAccount, TestCookieHealthFailurePreservesOpenPlaybackCache | PASS fixtures. |
| R9 non115 regression | internal/auth; internal/account | internal/auth/oauth_integration_test.go:TestOAuthDriversUseUnifiedGuard; internal/account/localfs_mount_regression_test.go:TestLocalFsMountCreateAndListingWithoutDualCredentials | PASS. Not every remote provider live tested. |
| R7/A6 boundaries | drivers/115, internal/driver capability contracts, internal/auth/gate_checker.go | targeted packages and vet | PASS compilation; no complete architecture import-rule test performed. |
| R6/A12 list/info/full list | drivers/115/{driver,full_list}.go | preserved source implementation; targeted build | Interface retained. Dedicated list pagination and info error fixtures not found in drivers/115 root tests. Full wire preservation not established. |
| R6/A12 file mutations | drivers/115/ops.go | deletion_ttl_test.go:TestDeleteFilesTrashesOnlyRequestedIDs; reference_trace_test.go:TestReferenceWireTemporaryCopyOrderAndExactTrash | Trash exact IDs PASS (D9); move/ordinary copy/rename/mkdir retained source. Dedicated ordinary mutation fixtures absent. |
| R6/A12 local uploads | drivers/115/upload.go | source retains init/range SHA1/resume/OSS token/single/multipart/retry/state | Build PASS; no dedicated 115 upload fixture suite found. Cannot certify multipart/restart/header/error order from compilation. |
| R6/A12 offline | drivers/115/offline_download.go | source retains URL add/hash recovery/task pagination/delete/torrent prepare/selection/seed folders | Build PASS; no dedicated 115 offline fixture suite found. Full normal/failure traces absent. |
| R6/A12 profile | drivers/115/profile.go:GetAccountProfile | retained Open user-info mapping | Build PASS; dedicated profile payload fixture absent. |
| R6/A12 cross-transfer | driver.go ProvideHashes sha1; upload.go LocalUploader; ops.go Downloader | capability-inventory.md distinction verified against current source | Source SHA1 and ordinary upload/download retained; generic hash-only RapidUploader was never a retained capability. No actual 115 source→destination transfer integration test independently run. |
| A3/A13 request parity | transport.go; qr.go; client transport tests | TestReferenceWirePathDoubleLookupThenDownurl, TestReferenceWire401DoesNotRefreshOrUseCookie; client TestCookieExchangeMultipartAndResponseCookies, TestOpenQRPKCEAndNoCookie, TestQRStatusExactQuery | Specific covered traces PASS. Full interface parity remains unproven; concrete header gap below. |

## Concrete finding requiring coordinator decision

Reference `/Users/saki/Desktop/code/ebn/restored-python/util/wangpan/util_115_open.py:53-57` initializes Open session headers with `User-Agent: EmbyNginx`, `Accept-Encoding: gzip, deflate`, and form Content-Type. Current `drivers/115/transport.go:rawRequest` uses Chrome defaultUA for non-overridden business calls and does not explicitly preserve reference Accept-Encoding. `postPassport` likewise uses Chrome UA. Caller downurl UA is correctly overridden. No approved header deviation was found in decisions.md. This is observable behavior under R5/R15, so not silently rewritten by reviewer. Recommend restore reference defaults and assert both ordinary Open and refresh wire headers, preserving explicit player UA.

429 reset+5 and shared admission/retry15/1s are visible in both reference `_request_api` and Go transport. Reference recursive retry invokes `_check_session` again; Go rawRequest recursion reuses captured token. Expiry crossing during a long 429 wait is therefore additional unverified parity behavior; needs a targeted clock/trace test before full exception-path parity can be claimed.

## Verification

- PASS `go test ./drivers/115 ./drivers/115/client ./internal/auth ./internal/account ./internal/api` (cached).
- PASS `go vet ./drivers/115 ./internal/auth ./internal/account ./internal/api`.
- PASS `cd web && npm run type-check`.
- No web lint script is defined. No extra broad suite repeated; coordinator already ran broad checks.
- D10 real-account/player acceptance remains explicitly deferred.

This audit does not claim completed exhaustive reference equivalence for upload/offline/profile/cross-transfer. The missing focused evidence above must remain visible in the final acceptance decision.

## Reference corrections applied after coordinator confirmation

Coordinator authorized restoring the already approved R5/R15 reference behavior. Fixed `drivers/115/transport.go`, `driver.go`, `ops.go`, `qr.go`, and client `open_auth.go`, `transport.go`, new `response_body.go`.

- Reference util_115_open.py:53-57 session defaults now preserved: EmbyNginx, requests Accept */*, gzip/deflate, keep-alive, form Content-Type even GET, no Accept-Language.
- Reference download_url session UA mutation precedes expiry refresh, stays account-local under mutex, and later metadata/refresh inherit it. Explicit downurl UA remains request-local as well.
- Reference :176-196 429 reset+5 retry reenters expiry/capability/admission processing and captures refreshed token/generation.
- Fresh Open QR token/exchange/status use Open session headers. Cookie QR keeps its original path.
- Explicit compression is decoded locally to 115 with compressed and expanded byte limits, gzip/zlib/raw-deflate support, and transport-Uncompressed protection.

Added actual wire tests in `drivers/115/open_session_reference_test.go`: `TestOpenReferenceStickySessionHeadersAndCompression`, `TestOpen429RechecksExpiryBeforeRetry`, `TestOpenDownloadUpdatesSessionBeforeExpiryRefresh`; client `open_session_reference_test.go`: `TestOpenQRStatusFreshSessionHeaders`. Retry test uses per-driver wait injection (not global hook), checks exact metadata→refresh→metadata trace with new bearer token and no Cookie calls. Existing waits/admission remain unchanged.

The header and 429 defects listed above are now fixed. Retained upload/offline/profile/cross-transfer evidence limits remain unchanged; no broad evidence claim is added by these corrections.

Refresh envelope correction: reference util_115_open.py:127-139 logs nonzero code but returns data regardless of `state`. `postPassport` now ignores state type/value and accepts usable token data even for nonzero code, matching that flow. `TestOpenRefreshReferenceCodeEnvelope` covers code0 without state, boolean states, nonzero code with token data, and null data with unchanged credentials. Business response envelope remains an explicitly reported distinction: Go rejects state=false; reference `_request_api` logs code then returns selected data. No new speculative change was applied to business errors.

## Final source alignment

Reference util_115_open.py:176-196 admission precedes `_check_session`: Go now performs business admission before expiry refresh, verified by `TestOpenAdmissionPrecedesExpiryRefresh` (admission→refresh→business). Local capability guards remain before remote work.

Endpoint-specific source behavior (get_item/download_url/user_info/list/delete versus copy/rename/create_folder) now controls envelope checks. Data consumers use returned data despite false state; delete treats HTTP-success JSON envelope as success regardless of state. Explicit state checks remain for copy/update/add and retained other endpoints. `TestOpenReferenceEndpointEnvelopePolicies` covers the eight endpoint policies; `TestOpenPathEmptyFalseStateRetriesAfterReferenceWait` verifies empty false-state data still triggers precisely one 30-second wait and retry; `TestOpenDeleteFalseEnvelopeIsOneHTTPCall` verifies exact-ID deletion has one HTTP call and no retry. Initial UID validation still rejects missing/invalid UID locally without extra HTTP. This resolves the previously reported business-envelope distinction for the specifically mapped reference endpoints.

Client JSON reading preserves its original unbounded semantics (`limit=0`), while driver Open transport retains its existing httpx body bound. New `client/response_body_test.go:TestResponseBodyCompressionBoundsAndTransportDecode` verifies expanded size limit, unbounded compressed decode, and no double decode after transport decompression.

## Coordinator final accounting

The separate `closure-retained-capabilities.md` review supersedes the initial retained-capability evidence gaps: public mutation, profile, URL-offline, instant-upload and signed single-PUT fixtures were added. All four retained implementation files are byte-identical to baseline after package normalization; multipart/resume/BT function bodies were preserved. This is preservation and finite offline integration evidence, not live OSS/BT acceptance. Driver/client race tests and vet passed after all corrections. Aggregate verification belongs `completion-audit.md`.
