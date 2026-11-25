# Research: Existing 115 capability inventory

- Query: Preserve current Go file/upload/offline/cross-transfer capabilities when replacing 115_Open.
- Scope: internal
- Date: 2026-11-25

## Findings
| Source | Existing responsibility / preservation requirement |
|---|---|
| drivers/115_Open/driver.go:35,137,198 | Registration metadata, SHA1 source support, lifecycle, paginated listing, info lookup; :54 init registers driver |
| drivers/all.go:6 | Blank import activates registration; replace import together with package/identifier, no old-config migration required |
| drivers/115_Open/ops.go:30 | Download link and pickcode memory cache; caller UA and metadata |
| drivers/115_Open/ops.go:141,162,167 | Delete/trash retained; historical permanent option explicitly removed under D9 to match reference trash-only behavior |
| drivers/115_Open/ops.go:236,248,261,277 | Move, copy, rename, create folder. Copy returns error only, insufficient for temporary-copy ownership |
| drivers/115_Open/full_list.go | FullListLister + directory-path resolution, used by existing enhanced STRM listing; tree replacement must explicitly map consumers |
| drivers/115_Open/profile.go:10 | Account nickname/member/storage profile |
| drivers/115_Open/auth.go:76,96 | RefreshAuth, token exchange; integrate capability health and generation-safe persistence |
| drivers/115_Open/transport.go | API transport, error decoding and interval gate; replace only after protocol comparison |
| drivers/115_Open/upload.go:52 | LocalUploader; target conflict policies :208; initial upload :253; secondary range-hash verification :278; resume :301/:315 |
| drivers/115_Open/upload.go:352,366,406 | Rapid result from local upload init, callback parsing and uploaded-item confirmation/pending state |
| drivers/115_Open/upload.go:953,978,1025 | OSS temporary token, credential refresh/error handling, single-part upload |
| drivers/115_Open/upload.go:1068,1095,1144,1185,1223,1254 | Multipart create/list/upload/retry/complete/resume with progress and persisted state (:1508); preserve restart behavior |
| drivers/115_Open/offline_download.go:88,99,168,227 | Offline capability declaration, URL submission, refresh and delete with source-file option; duplicate hash recovery :131 |
| drivers/115_Open/offline_download.go:242,306,343 | Torrent upload/preparation, file selection and submission, managed seed directory |

Cross-transfer distinction: config declares ProvideHashes=[sha1] (`driver.go:49`) but no RapidUploadHashes and no RapidUploadByHash implementation exists in this package. Upload init may instant-upload a local file; that is not evidence of implementing generic driver.RapidUploader. Preserve source SHA1 and ordinary transfer through download/local upload; do not claim hash-only destination cross-transfer already exists. Generic contracts are `internal/driver/transfer.go:11,27,53`; additional destination capability would be new scope/protocol work.

Public interfaces to preserve: Meta/Lister/InfoGetter/Downloader and FullListLister (`internal/driver/driver.go:51-100`), optional file mutation and LocalUploader contracts in same package, offline interfaces (`internal/driver/offline_download.go:60-103`), auth persister/request interval injection and AccountProfileProvider. Replacement should retain registration metadata and capability detection consistency. Config/schema/UI logo references require integration search when identifier changes.

Related specs: backend/index.md (template), workflow.md; PRD R1/R2/R6/R7, reference-contract.md. No external docs consulted; inventory describes local code, not promised live upstream support.

## Caveats / Not Found
No live upload, offline, cross-transfer or profile verification. No 115-specific transfer.go found; generic cross-transfer service call graph needs separate integration validation. Existing OSS implementation is substantial and must not be discarded merely because protocol playback is replaced. Full request-by-request upload/offline protocol equivalence remains W4 evidence work.
