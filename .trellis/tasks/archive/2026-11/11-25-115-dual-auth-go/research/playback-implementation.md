# W5 implementation status

Implemented a first integration slice in internal/embyproxy/direct_cache.go and
service.go: final 302 lookup before media-source/path/upstream work, with a
1200-second fixed lifetime, exact UA and device identity, IP fallback and
serialization of concurrent misses. Reference source: emby_nginx.py:39,109.
Only returned 302 Locations enter this cache; stream bodies are not buffered.

Validation: go test ./internal/embyproxy ./internal/playback passed before the
additional explicit-device/IP regression test; repeat run recorded by worker.

W5 is NOT complete. Required follow-up: move cache ownership into Service and
connect credential/account invalidation, bound expired slot retention, implement
session retrieval and exact list predicate, temporary-copy optional interface,
reference driver adapter, durable cleanup inventory and restart recovery,
10-second cleanup, upstream fallback and metadata eager-resolution removal.
No parity claim or live-account verification is made. Current cache should not
be treated as release-ready until ownership/invalidation is integrated.

Second slice adds driver.TemporaryPlaybackCopier and the 115 adapter, temporary
playback Intent, reference session endpoint/list predicate and ten-second
best-effort cleanup. Rename failures return their copy handle for cleanup.
Emby resolution failures use existing proxyRequest rather than an invented
external /embydirect route (observable proxy-vs-302 difference).
Final cache now belongs to each Service, avoiding cross-instance contamination.
Tests added for duplicate device-list semantics. Final package recheck temporarily
blocked by concurrently introduced internal/file/tree_snapshot.go calling
Executor.Inspect before that method exists, plus CookieDownload wiring.
Durable cleanup inventory/restart recovery remains required and unimplemented.

## Integrated W5 update

The earlier incomplete-status paragraphs describe intermediate slices. Current
implementation includes:
- Service-owned final direct cache, fixed 1200s lifetime, GET/HEAD reuse,
  exact UA/device/IP identity, serialized misses, idle-slot retention bound,
  account-enable gate check and playback invalidation generation.
- Verified delete/move/rename file mutations invalidate all account link keys,
  including path references. NotifyExternalMutation exposes the same invalidation
  to the event owner without recursively publishing file mutations.
- Session endpoint and raw MediaSourceId comparison; exact singleton-list
  equality, including duplicate-device behavior; unsupported drivers retain
  ordinary playback instead of attempting copies.
- Reference source-path lookup -> copy-to-root -> original-name lookup ->
  rename, numeric copy preceding necessary metadata lookup, copied pickcode
  retained to avoid an additional file-info request, and timestamp+basename.
- Durable copy ledger (migration0025), request ownership UUID, recorded remote
  ID, deferred cleanup after link attempt +10s, failure state and retry due,
  App.Run recovery and cancellation. TrackedTemporaryPlaybackCopier records
  known copy ownership BEFORE rename. In-flight ownership is excluded from the
  cleanup worker; after process restart it is eligible for recovery.
- Exact Open trash deletion, independent of user permanent-delete preference.
- Emby original-upstream proxy fallback before response commit; successful
  LitePan redirects alone enter direct cache. Proxy-vs-external-302 remains the
  documented architecture mapping difference.
- PlaybackInfo metadata inference uses STRM filename only and never downurl.

Validation: targeted tests pass in playback, embyproxy, store, drivers/115 and
app. Added persistent SQLite reopen ownership test, device/list tests and
21-entry seek trace (first+20 seeks) with one path/link resolution and zero
upstream metadata calls, plus different-device isolation. Independent audit
adds actual HTTP GET/HEAD trace and recovered-cleanup exact-ID/failure tests.
Race tests passed before the final tracked-before-rename refinement; final
race rerun is recorded in agent report.

Remaining limits: reference copy API does not provide a directly usable created
ID to this wrapper. A crash or failed root lookup after POST copy but BEFORE
identifying the new object cannot safely authorize deleting an unknown object.
Known-ID ownership is durable before rename; the earlier unidentified interval
remains a protocol/reference limitation, not a claim of leak-free atomic remote
creation. No real credentials/player/CDN acceptance was performed. Authentication
refresh/relogin must invoke playback.InvalidateAccount to advance final-cache
generation; main coordinator owns that cross-layer integration.

## STRM Lavf queue fidelity

Source constant.py:16 constructs a process-shared TaskPriorityQueue(2), separate
from the one-request/second Open business limiter. task_queue.py:104 orders
priority then monotonic insertion count. strm.py:232 submits Lavf Open tasks at
priority2; non-Lavf takes the direct branch. strm_get_result observes immediately,
then sleeps one second before each further result observation.

New driver.STRMDownloader is invoked only for marked ordinary STRM entrypoints,
after playback cache lookup and miss coalescing. drivers/115/playback_queue.go
uses two process-shared workers, a priority/FIFO heap, priority2 and the reference
result-observation interval. Cancellation interrupts queued waits and canceled
pending work does not issue cloud requests. Existing limiter alone controls Open
API cadence; Cookie attachments and Emby playback do not enter this STRM queue.
STRM resolution cache keys are partitioned from ordinary/Emby resolution keys,
preventing a warm ordinary resolver from bypassing STRM branch semantics.
Tests cover two workers, canceled pending jobs, result observation timing,
concurrent miss coalescing, twenty warm entries and ordinary/Emby isolation.
