# Research: Playback contract

- Query: Reference playback predicates, waits, fallback and minimal Go integration.
- Scope: internal
- Date: 2026-11-25

## Findings
Reference root: `/Users/saki/Desktop/code/ebn/restored-python`.

### Exact reference flow
- `router/emby_nginx/emby_nginx.py:109`: missing device becomes client IP; cache `(server_name, video_id, UA+device)` precedes PlaybackInfo and session/cloud work. Helpers at :39 expire records older than 1200 seconds.
- :116 obtains PlaybackInfo only when not provided, chooses numeric ItemId or string MediaSource Id, then maps HTTP STRM/local mount paths at :138. Preserve media-source selection before account/path resolution.
- :320 Open branch chooses configured Open client; :330 gets playing sessions. Without balance config count is zero; configured balancing adjusts actual count with max-main. Balancing/rotation are excluded by PRD R3, an explicit product difference.
- :338 original file iff media source absent from session map OR device list equals exactly `[device_id]`. Duplicate identical devices still cause copy; do not replace list equality with set membership. Session helper `util/emby/emby_util.py:223` GETs `/emby/Sessions` with IncludeAllSessionsIfAdmin=true, IsPlaying=true and api_key, uses PlayState.MediaSourceId and DeviceId without normalizing prefix; empty/error returns ({}, []). Preserve raw comparison evidence.
- :343 temporary name is Unix-seconds + underscore + basename; `copy(path, 0, temporaryName, basename)` resolves source, POSTs `/open/ufile/copy` with integer file_id/pid, resolves copied original-name root path, then renames (`util/wangpan/util_115_open.py:375`). Return contains created file_id/pick_code; rename does not refresh returned metadata. Root-path collision behavior must not silently be redesigned.
- :347 gets temporary pickcode link, then starts deletion thread. `delete_cache_video` :67 sleeps 10 seconds then invokes Open delete(exact ID). A returned non-string link still schedules cleanup; raised exception before scheduling can leak. Durable recovery is R14-required difference, not claim of reference parity.
- `util/wangpan/util_115_open.py:241` normal path download intentionally calls get_item(path) twice before download_url; do not optimize away. get_item :197 waits 30 seconds and retries once if missing data. download_url :233 updates session UA and sends explicit UA on POST `/open/ufile/downurl`, pick_code form field; extracts first data value.url.url.
- `_request_api` :176 uses shared decorator bucket rate=1/capacity=1, up to 15 one-second acquisition waits. This is admission retry, not HTTP retry. 429 recursively repeats after X-RateLimit-Reset(default 60)+5 seconds; no explicit recursion bound. Decorator bucket is created once at decoration, thus shared across instances. Any bounded replacement requires difference approval.
- :516 reads MediaSourceId (removes mediasource_), DeviceId and PlaySessionId; PlaySessionId is read but not used in resolver. Hills UA selects synchronous invocation. :559 missing/exception link returns 302 to X-Original-Url + /embydirect + original path/query. Original-service fallback is distinct from Cookie fallback; ordinary STRM has none.

### Proposed integration contract (proposal, not approved implementation)
- Emby owns `PlaybackIdentity{ServerID, ItemID, MediaSourceID, DeviceID, ClientIP, PlaySessionID}` and session retrieval/predicate. Do not put Emby wire fields in generic driver or infer device from UA. Keep PlaySessionID diagnostic; reference does not key cache with it.
- Emby direct-result cache must run before resolving path/file metadata/session state: structured `(server, selected video/source, UA, effective device)` with 1200-second reference policy. Account/credential generation invalidation can be carried internally; altered expiry needs explicit difference accounting.
- Optional driver `TemporaryPlaybackCopier` proposal: CreateTemporaryPlaybackCopy(ctx, {source ref,target parent=0,newName,oldName}) -> {FileID,PickCode,OwnershipID}; ResolveDownload remains ordinary Open link resolution; DeleteTemporaryPlaybackCopy(ctx, owned handle) deletes exact recorded copy. Record remote ID immediately; schedule 10-second cleanup after link attempt and recover pending owned copies after restart. Never call generic CopyFiles then guess destination ID.
- Playback add optional resolve options/cache partition for Emby copies; retain existing Resolve signature as ordinary wrapper. Distinct devices must not coalesce through ordinary account/file/UA key. Emby chooses copy, driver executes it, cleanup coordinator persists ownership. Ordinary STRM uses no copy option.
- Resolve first, then write 302; on Emby resolution error invoke existing original-upstream proxyRequest with original path/query and explicit internal bypass routing, avoiding the interception handler. This preserves original-service outcome but proxy versus reference external 302 is an observable difference to approve. Never fallback after response committed.

### Existing Go source map and differences
- `internal/playback/service.go:116` cache is account/file/UA/playback flag, coalesced; TTL is link expiration else 5 minutes (`cache.go:12`). No device dimension. :159 driverexec wraps resolve; :194 missing size/name causes InfoGetter extra query. Return complete metadata for reference links and evaluate extra-query difference.
- `internal/playback/pick.go:14` Intent has no Emby context. DownloadRequest in `internal/driver/driver.go:95` has only FileID/UA.
- `internal/embyproxy/service.go:910` media source cache stores play URL, not final direct URL. :975 stream handler forwards HEAD directly; cached URL still goes through STRM path resolution before playback cache. :1290 inferLitePanMediaInfo calls Resolve during metadata inference, potentially obtaining link before stream entry. All need ordered trace fixtures, not just downurl assertions.
- :836/:881 playback Serve errors are handled within current adapters; add an explicit pre-response error result to preserve fallback. :1217 requestUpstream buffers bodies and is unsuitable as media streaming fallback; use proxyRequest.

### Related specs / references
Read `.trellis/workflow.md`, backend/index.md (guidelines currently templates), PRD R5/R8/R14/R15 and reference-contract.md. Local reference is reconstructed baseline, not verified live service; no external API version asserted.

## Caveats / Not Found
Nginx emb ydirect deployment wiring, full PlaybackInfo rewrite parity, auth refresh details and queue scheduling beyond cited decorator remain evidence gates. No live credentials/player testing performed. Exact Cookie/share/balancing branches exist but dual-auth media policy chooses Open (approved R8 difference); do not describe reference as universally Open-first.
