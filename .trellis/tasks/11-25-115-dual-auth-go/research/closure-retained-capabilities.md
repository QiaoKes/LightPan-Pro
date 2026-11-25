# Retained capability closure review

New tests in drivers/115/retained_capability_reference_test.go invoke public entry points through an isolated per-driver RoundTripper; no global transport or wait hook, credentials, or network access.

- TestRetainedMutationWire: CreateFolder → add (decoded folder ID/name); RenameFile → update; MoveFiles → move; CopyFiles → copy; DeleteFiles → delete. Exact ordered POST fields, escaping, trimming and request count asserted. Existing normalization preserves duplicate move IDs; no new deduplication requirement inferred.
- TestRetainedProfileOfflineWire: GetAccountProfile → user/info (identity/storage/member mapping); AddOfflineURLs → add_task_urls (blank filtering); RefreshOfflineTasks → get_task_list (case-insensitive hash match, decimal-string progress); DeleteOfflineTask → del_task (source-file flag). Exact sequence and serialization asserted.

Inventory accounting: driver registration/list/info/download/full-list, refresh and shared transport have existing reference/driver tests outside this file. New fixtures above cover ordinary mutations/profile/URL offline paths. upload.go local initialization, secondary range verification, instant upload confirmation, OSS single/multipart/resume/token-refresh and BT seed preparation/submission remain source-retained but NOT verified by these new fixtures. Generic cross-transfer advertises SHA1 source capability and ordinary local destination upload; no generic RapidUploader destination existed in the baseline. End-to-end transfer use remains unverified here. These are limits of newly added behavioral fixtures; unchanged source establishes preservation. Source preservation is independently verified below; additional branch fixtures are optional coverage, not evidence of removed capabilities.

No production changes. Scoped go vet passes. Scoped race tests recorded by reviewer report. Live OSS/cloud behavior remains unverified.

## Independent normalized source identity

- `upload.go` SHA256 `d313c08c8feee48af00936f3d82d18bb85db93e4fa7af942508e0c5bd67b52c3`: baseline package-normalized bytes equal current bytes. All function bodies, including private retry/resume/BT helpers, preserved.
- `offline_download.go` SHA256 `943adc7d809137bd0a5c24996e02bb5bb7a9ca141dd39b2efc73cd8d1a856aa7`: baseline package-normalized bytes equal current bytes. All function bodies, including private retry/resume/BT helpers, preserved.
- `full_list.go` SHA256 `551d31c572396a4ce136f87f8f758e58f54eb6592c85ec08797ca364d4538a46`: baseline package-normalized bytes equal current bytes. All function bodies, including private retry/resume/BT helpers, preserved.
- `profile.go` SHA256 `d97b816ffafc49ffcaf4128db92893b9d1171dec2fa1f19a1fd7a7a75a49c3ed`: baseline package-normalized bytes equal current bytes. All function bodies, including private retry/resume/BT helpers, preserved.

Public entry points preserved: UploadLocalFile; OfflineDownloadCapabilities, AddOfflineURLs, RefreshOfflineTasks, DeleteOfflineTask, PrepareOfflineTorrent, AddOfflineTorrent; ListAllFiles; GetAccountProfile. TestRetainedPublicUploadInstantAndSinglePUT additionally invokes local upload with real hello bytes, validates SHA1/preid/size/target initialization through shared Open transport, verifies instant result and no OSS request, and signed single PUT content/length/token plus persisted hash and confirmation result. This proves offline integration, not live cloud acceptance.

Generic destination call chain: internal/upload/worker.go:304 → Manager.runLocalUpload → file.Service.UploadLocal when configured, otherwise executor.Run → driver.LocalUploader.UploadLocalFile; internal/file/service.go:469 calls the same interface. `git diff --name-only HEAD -- internal/upload` is empty, independently verifying unchanged generic upload/transfer manager source. FullListLister also preserves ResolveDirPath. No new hash-only destination capability is necessary. Source identity plus new public transport integration fixtures establish preservation; real account/network outcomes are D10 deferred. Additional multipart/BT simulation is optional regression coverage.

Final verification: `go test -race ./drivers/115 -run TestRetained -count=1` passes (17.016s); `go vet ./drivers/115` passes. Race invocation compiles/type checks all package tests. No production changes.
