# Research: STRM tree, attachment and life-event contract

- Query: Freeze reference call order, path snapshot/event interfaces and managed output integration.
- Scope: internal (local restored business sources and pinned SDK source)
- Date: 2026-11-25

## Findings

### Sources and related specs
Python paths below are relative to `/Users/saki/Desktop/code/ebn/restored-python/`.
- `util/wangpan/util_115_tree.py`: export, parsing, attachment and reconciliation business flow.
- `util/wangpan/util_115_monitor.py`: life records, deletion evidence, event application and backup.
- `util/wangpan/util_115.py:63,107`: attachment path-to-ID-to-pickcode and download resolution.
- `config/constant.py:7-20`: media/attachment extensions and shared priority queues.
- `util/queue/task_queue.py:81`: priority worker ordering and swallowed errors.
- `internal/strm/scanner.go`: generic recursive scanner and destructive cleanup helpers.
- `internal/strm/enhanced.go:58`: FullList scan requires IDs/parent path resolution; unsuitable for ID-less tree entries.
- `internal/strm/reference.go:15,42`: existing path references already distinguish RootID/RelativePath from FileID.
- `internal/strm/writer.go:70,146`: task output layout to reuse.
- `internal/strm/metadata_reconcile.go:117-150`: download before cloud-primary deletion; explicit local-primary/bidirectional semantics.
- `internal/strm/coordinator.go:37,63`: auth and FileMutated forwarding, not a durable upstream life consumer.
- `internal/domain/strm.go:39,110`, `internal/store/strm_repo.go`: task persistence without upstream event checkpoint/output inventory.
Read `.trellis/workflow.md`, `.trellis/spec/backend/index.md` (guidelines remain placeholders), task PRD, design and mandatory reference-contract. R10–R13 control approved output safety and monitoring differences.

### Request / call-order matrix
| Scenario | Ordered reference calls and actions | Source |
|---|---|---|
| Root export | fs_files() once; collect fid otherwise cid in returned order; comma join; fs_export_dir(ids) | tree:133-149 |
| Subtree export | trim trailing slash; fs_dir_getid(path)[id]; fs_export_dir(id) | tree:142-149 |
| Export wait | require data/export ID; check elapsed timeout before each status call; fs_export_dir_status(exportID); empty data sleeps 2 seconds; default timeout 3000 seconds | tree:150-163,319 |
| Export transfer | Cookie download_url(pick_code, headers User-Agent empty); GET returned URL with empty UA; initial attempt plus five immediate recursive retries; successful download then fs_delete(export file_id), BEFORE parse | tree:166-177 |
| Parse/write | detect encoding; decode ignoring errors; split newline-pipe; stack pipe depth; root cut1/subtree cut2; yield leaves, not authoritative directory IDs; media extension lowercase; exclusive create, existing skipped | tree:32-67,237-273 |
| Ordinary attachments | skip existing; image/NFO/non-special subtitle selection; CD2 executor max10, otherwise shared priority3 queue Cookie worker1/Open worker2 with global monotonic FIFO tie breaker | tree:178-232,274-285; constant:14-20 |
| Cookie path attachment | get_id_to_path(refresh=True), get_attr(id,True), download_url(pickcode,user_agent empty); sleep1; GET empty UA; sleep0.5; write bytes | util_115:63-82; tree:207-224 |
| Final tree reconciliation | delete local tree file; remove unseen STRM; optional directory pruning; THEN subtitle enumeration; wait executor futures; refresh Emby if created/deleted | tree:286-318 |
| Special subtitles | for suffix srt,ass,ssa in order: iter_files_with_path(cid root0 or resolved subtree, escape=None,cooldown1); fs_video_subtitle(pickcode); submit each returned file URL; direct GET does not explicitly set empty UA | tree:294-312 |
| Monitor batch | iter_life_behavior_once(today,first_batch_size1000,cooldown2); load backup if memory empty; skip Done record IDs; process; persist record status map; refresh changed folders | monitor:230-303 |
| Event create types1,2,5,6,14,18 | directory get_path(file_id,refresh=True), skip subtitle temp dirs; sleep3 then iter_files_with_path_skim; file get_path(parentID,refresh=True) with per-batch parent cache; create STRM or queue attachments | monitor:138-201,250-270 |
| Event delete type22 | recyclebin_list cached by update time; match filename and dtime-update_time <=3 (no lower bound); get_path(recycle cid,refresh=True); delete local | monitor:271-284 |
| Rename types20,24 | get_path(parentID,refresh=True); construct old/new names; delete OLD then create NEW; directories traverse after sleep3 | monitor:285-292 |

Do not confuse unused helper `get_life_list` (monitor:50-69: HTTP life_list limit1000/show_type0/start_time minus30, timeout10, sort update_time) with active monitor entry using SDK iter_life_behavior_once. R13 overlap windows are approved requirements, not proof that this helper drives production monitor.
Queue futures only cover executor tasks; reference does NOT wait shared QUEUE completion before reporting tree completion. TaskPriorityQueue workers swallow exceptions and call task_done; tuples order by priority then counter. Preserve scheduling trace; durable completion semantics required by R13 need explicitly registered differences.

### Proposed exact reusable capability shapes
These are recommendations for final review, not implemented declarations:
```go
type TreeSnapshotProvider interface {
    ExportTree(context.Context, TreeRequest, func(TreeEntry) error) (TreeSnapshot, error)
}
type TreeRequest struct { OperationID, RootID, RootPath string }
type TreeEntry struct { Path, StableID, Pickcode string; IsDir bool; Size *int64 }
type TreeSnapshot struct {
    OperationID, RootID, RootPath string
    Complete bool
    Warnings []string
}
type LifeEventProvider interface {
    ReadLifeEvents(context.Context, LifeRequest) (LifeBatch, error)
}
type LifeRequest struct { Cursor string; SinceUnix int64 }
type LifeBatch struct { Events []LifeEvent; NextCursor string; Complete bool; HistoryGap bool }
type LifeEvent struct {
    ID, FileID, ParentID, Path, OldPath, Pickcode string
    Kind string // create, delete, rename, move
    UpdateUnix int64
    IsDir bool
    Verified bool
}
```
Driver owns protocol/root resolution and preserves export deletion ordering. STRM owns scope checks/output operations. Snapshot callback writes only a staging inventory until successful return Complete=true. Tree StableID/Pickcode remain empty when absent, Size=nil; do not manufacture IDs or size=0 then apply minimum-size filtering as if known. Directory tree omits empty directories; explicit capability documentation must say its completeness concerns exportable leaves, not authoritative empty directory inventory. Failed callback/parse/download/cancellation never grants deletion eligibility. Cleanup errors reported separately; a remote export deletion failure interrupts reference before parse and must not be silently treated as success without approved change.

### Serialization and ownership
Use one per-task application lock spanning snapshot start through inventory reconciliation and life-event processing. This simplest choice prevents old exports undoing newer events without assuming export timestamp is a server-consistent checkpoint. Event fetch can occur outside lock but apply/dedup/checkpoint cannot. Persist revision plus event ID dedup window and cursor; empty/error/history-gap batches cannot authorize snapshot deletion. After Cookie outage run complete reconciliation before resuming queued events; monitoring opt-in, starts only after first complete snapshot (R13).
Proposed durable tables: strm_sync_state(task_id,revision,cursor,last_complete_operation,needs_reconcile); strm_event_receipts(task_id,event_id,update_time,status); strm_outputs(task_id,relative_path,kind,remote_id,remote_path,generation,content_fingerprint); strm_output_operations(task_id,operation_id,stage,payload) for crash recovery. Database transaction cannot encompass filesystem atomicity: stage files, persist operation intent, atomically publish, commit inventory/checkpoint, retire obsolete owned files, record pending cleanup for replay. No event Done until required output work durably succeeds. Collision or unknown local file is conflict, not implicit ownership.
Reject root/equal/ancestor/descendant output overlaps and nonempty unowned roots. Lstat components and no-follow operations enforce symlink containment. Delete only inventoried owned files; directory removal only if empty. Existing scanner helpers `removeStaleStrmAndSameStemSidecars` (942), `cleanupMissingRemoteChildDirs` (1221) and writer DeleteTaskOutput (158) cannot implement this contract because they remove unowned sidecars/subtrees. Cloud-primary metadata comparison must intersect deletion candidates with inventory; local-primary/bidirectional remain policy-owned. Rename/move publish destination before retiring source; move outside scope retires owned source only.

### Minimal integration changes
Add generic capabilities in internal/driver/tree.go and events.go; expose through file.Service rather than importing 115 into STRM. Driver 115 tree/events adapters map protocol DTOs. Add internal/strm/tree.go, life.go and ownership.go; extend ScanDeps with optional capability access and inventory repository; scanner selects tree capability before FullList when selected. Reuse writer path layout and path playback references; extend metadata inputs for path refs and ownership filtering. Extend domain strm persistence with monitor settings and sync repositories, store migration/repository, task creation validation, service scheduler/start-stop and coordinator capability-aware pause/resume. Existing enhanced.go remains generic FullList, no fake-ID conversion. Add selected tree mode rather than silently routing every provider to export.

### Approved differences and evidence gates
R11 replaces reference broad remove_no_exist/rmtree and delete-before-create rename with owned-only deletion and destination-first rename. R12 replaces unqualified attachment delete with mode-aware policy, default cloud_primary. R13 adds opt-in startup, durable checkpoints, overlap dedup and serialization; reference marks ignored types and swallowed delete failures Done and keeps a per-day record map. R10 adds explicit partial/cancel result and safe temporary ownership. R3 removes reference client cycling across credential sets. All differences must appear in scenario traces. Do not fix root single-page listing, malformed decode tolerance, recycle time match or queue failure swallowing by intuition: source evidence + explicit difference approval still required where product requirements do not settle behavior.

### Fixture candidates
Root fid/cid ordering and truncated first page; subtree Unicode/slash/percent names; actual tree indentation and invalid encoding; empty export and missing data; polls at2s and timeout boundary; six GET failures; successful transfer then remote-delete failure proving parse not called; parser failure after remote export removed; .srt special API versus .sup priority queue; FIFO priority ties; attachment existing-skip and 1s/0.5s waits; executor completion preceding shared queue; duplicate life IDs after restart; zero-byte file is_dir heuristic; recycle mismatch/negative delta; directory create3s wait; rename destination failure retaining source; unowned sidecars/symlink/manual files; stale snapshot vs new event; crash between publish and checkpoint; Cookie recovery/history gap; all metadata modes.

## Caveats / Not Found
No live account or upstream requests performed. SDK endpoint evidence is supporting local source, not verified bundled SDK parity: pinned client.py:11808 POST https://webapi.115.com/files/export_dir form file_ids,target=U_1_0; :11910 GET same endpoint export_id. Active life helper internals and subtitle helper wire traces still require bundled-version confirmation by protocol owner; do not substitute unused life_list helper or assume generic retries. Restored tree module explicitly describes reconstruction from old source/native evidence; retain restore-note provenance. Exact get_path/iter_files_with_path_skim request-level helper expansion remains a protocol evidence gate, not parity established by this report.
