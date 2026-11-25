# Parallel requirement matrix and roadmap

Status: in_progress. Execution authorized on 2026-11-25; nine child tasks created, including the D11 character-detection follow-up. Implementation and independent audits run in parallel with gpt-6.1-sol agents.

| ID | Deliverable | Dependencies | Exclusive ownership | Output | Acceptance |
|---|---|---|---|---|---|
|W0|协议基线与共享契约|无|research/*、contracts.md；协调者独占共享接口决策|SDK版本差异、完整请求清单、DTO/错误/事件/QR契约|G1–G4完成；契约评审|
|W1|Go 协议客户端与密码学|W0|drivers/115/client/transport*、errors*、cipher/|请求执行器、错误分类与固定向量；业务调度由W4/W5接线|序列化/解密向量、429、取消、并发和优先级|
|W2|Cookie/Open 登录与凭据|W0；集成依赖W1|drivers/115/client/*auth*、drivers/115/auth.go；指定共享auth/store文件|双扫码、UID核对、刷新与原子持久化|状态机、交叉账号拒绝、并发刷新、凭据不互相覆盖|
|W3|挂载与扫码前端|W0；集成依赖W2|web/src/components/admin/AddAccountDialog.vue、QrLoginModal.vue、web/src/api/qr.ts、accounts.ts|双授权表单、设备选择、失效提示|缺一不可、取消/过期/成功、其他驱动扫码回归|
|W4|115 驱动与文件接口|W0；集成依赖W1/W2|drivers/115/driver.go、config.go、ops.go、client/*files*；注册接线由协调者|浏览/路径/属性/变更及原有上传离线能力适配|分页完整、文件操作/跨盘/上传接口回归|
|W5|直链与播放|W0；集成依赖W1/W2/W4|drivers/115/client/download.go、internal/playback/；驱动下载适配由W4接线|Open优先取链、UA/缓存/coalescing、302|多UA、多账号、过期/错误、GET/HEAD/Range调用次数|
|W6|目录树与 STRM|W0；集成依赖W1/W4|drivers/115/client/tree.go、drivers/115/tree.go、internal/strm扫描与writer指定文件|树导出解析与完整快照、路径引用|根/子目录、Unicode、超时/取消、不完整不清理、临时文件|
|W7|附件与事件增量|W0；集成依赖W1/W4/W6|client/events.go、subtitles.go、drivers/115/events.go、STRM新增附件/事件文件|字幕/NFO/图片、事件去重/断点/重命名|重放幂等、checkpoint故障恢复、附件限速与播放优先|
|W8|集成与质量门|W1–W7|协调者：registry/router/shared contracts；检查代理：tests/docs/notices|注册替换、删除废弃入口、端到端验证、部署说明|Go/race、前端类型/构建、离线请求轨迹、无Python运行依赖；真实播放按D10后续验收|

## Waves
1. M0: W0 research and contracts. Hard gate for protocol-dependent implementation.
2. M1: W1 and W2/W3/W4 contract-driven scaffolding can proceed in parallel; only W1 owns shared protocol transport. Business scheduling is assigned explicitly to W4/W5. Stubs are not accepted as completed features.
3. M2: W2/W4 integrated; W5 and W6 can develop concurrently against frozen FileRef. W3 integrates dual login.
4. M3: W7 integrates snapshots/events; W5/W6 behavior tests and cross-provider regressions run in parallel on non-overlapping files.
5. M4: W8 integrates all features, removes obsolete 115 entrypoints, performs offline verification; real-account verification is deferred under D10.

Critical path: W0 -> W1 -> W2/W4 -> W6 -> W7 -> W8. Playback path W5 can finish before event synchronization. No calendar estimates until protocol evidence gates close.

## Agent handoff rules
Every dispatch begins `Active task: .trellis/tasks/11-25-115-dual-auth-go` and states work-package ID, dependency revisions, exact writable files, read-only references, acceptance commands and unresolved gates. Agents are not alone; never revert another agent's edits. Shared files (domain/account.go, driver contracts, api/router.go, registry and migrations) have one named integrator at a time. Request contract changes from coordinator; do not independently invent DTO variants. W4 owns ops.go, so W5 proposes its adapter changes instead of editing concurrently.

Each delivery includes changed files, acceptance evidence, request differences from the reference, outstanding unknowns and integration notes. Parallelism is allowed only after contracts freeze; interface changes re-open affected gates. Child Trellis tasks can be materialized from these rows after planning review, with explicit dependencies copied into each child PRD/design rather than relying on tree order.

## Architecture refinement
Apply design.md consolidated proposal: W2 includes capability-aware auth gates; W5 includes Emby session/temporary-copy integration; W6 includes non-overlapping output ownership; W7 respects metadata sync modes and orders event/snapshot reconciliation. W0 must freeze these contracts before workers touch shared interfaces.

## PRD alignment — 2026-11-25
The user approved consolidated behavior for PRD inclusion. `prd.md` R1–R15 and A1–A13 are authoritative; D1–D11 in decisions.md are settled product behavior. Earlier proposed/pending product wording is superseded. The user subsequently authorized execution. Offline protocol evidence is verified; offline delivery and deferred live-account acceptance remain separate under D10.

## Reference fidelity gate
Read `reference-contract.md` before dispatch, implementation or review. Every package must inventory required reference modules and deliver source-to-Go mapping plus ordered request traces. Do not optimize waiting/retries/cache/cleanup or omit helpers without an approved behavior difference. Copy this constraint into every child task.

## Execution checkpoint: 2026-11-25

| Package | State | Evidence / remaining gate |
|---|---|---|
| W0 | Offline verified | Same-artifact SDK/cipher execution, RSA/AES/LZ4 vectors, provenance recorded |
| W1 | Offline verified | Bundled cipher vectors, raw QR/SDK query distinction, encrypted wire fixtures |
| W2 | Offline verified; L1 deferred (D10) | Independent health, UID, generation CAS, field-scoped atomic credential/runtime persistence; race fixtures |
| W3 | Type/build passed; L1 deferred (D10) | Dual credentials and QR, independent reconnect/health, monitoring and metadata policy |
| W4 | Offline verified; D9 complete | Driver replacement, retained file/upload/offline/profile capabilities, native path mapping; trash-only schema/runtime, exact-ID single call |
| W5 | Offline traces passed | 20 GET/HEAD seeks reuse URL; actual driver copy/downurl/trash order; durable known-ID cleanup; live players deferred (D10) |
| W6 | Offline verified; L3 deferred, L4 closed by W9 | Complete snapshots, absolute path references, confined owned outputs, crash recovery and task policies |
| W7 | Offline verified; L3 deferred (D10) | Bundled traversal/paging/queues, known-pickcode streaming, per-account event polling and per-poll expansion sharing |
| W8 | Offline checks passed; live follow-up deferred (D10) | Independent audits remediated; Go/race/vet/frontend/Linux build passed; evidence in acceptance.md |

| W9 | Offline differential verification passed (D11) | Pinned default detector/codecs/scoring/sorting;2690 detector inputs,99 tree cases,61 declaration cases and component oracles; no remote calls |

Offline fixture results must not be represented as live-service stability or a guarantee against account risk controls.

## Deferred acceptance and documented limits

D10 explicitly defers L1–L3 to a later round; each can be executed independently with the prerequisites below. They remain untested, not current offline-delivery blockers. D11/W9 closes the former L4 character-detection approximation; the export producer name ambiguity remains documented. D9 closes L5 by adopting trash-only behavior.

| ID | Work | Prerequisite | Ownership boundary | Acceptance evidence |
|---|---|---|---|---|
| L1 | Real Cookie/Open QR, same UID, restart and refresh | User configures a test mount through product UI | Auth/API/UI; never put secrets in task documents | Successful and cancelled/expired flows, reconnect isolation, one real refresh |
| L2 | Actual players,20seeks,Emby devices | L1 usable Open credentials and player access | Playback/Emby; passive request accounting | Separate control API and CDN Range traces, downurl/refresh/copy counts |
| L3 | Real tree,metadata,events,restart | L1 usable Cookie and small dedicated cloud/output test roots | Tree/events/attachments | Complete/incomplete scan, subtitles, rename/move/delete, owned/manual files, checkpoint restart |
| L4 / W9 | Closed under D11: default charset-normalizer3.5.1 pure Go port | Bundled source identity, pinned Python oracle | Detector, mess/Unicode, language coherence, base codecs, multibyte/ISO and independent checking in separate files | All-candidate detector and component differential fixtures, tree leaves, race, large-input behavior, pure Go Linux build |
| L5 | Closed by D9: trash-only product scope | User decision recorded | Driver schema and exact-ID trash operation | Permanent choice/branch removed; one call, exact IDs, no recycle probe; offline tests and independent review passed |

L1–L3 are deferred live acceptance under D10. D11/W9 completes the L4 default character-detection port and differential validation. Raw export slash-name ambiguity is inherited; required monitor escaping and its Cookie path inverse are implemented from bundled vectors. Generic unused SDK path options are not part of the required business port. L5 is resolved by D9; permanent purge is outside the approved scope. No extra recurring health/CDN probes should be added for any row.

## W9 D11 parallel follow-up

User explicitly rejected leaving three-encoding approximation as final. Child task 11-25-115-w9-charset freezes six independent ownership boundaries: detector/models; mess/Unicode; coherence/numeric; base codecs; multibyte encoders/decoders; ISO-2022 decoder. Protocol-baseline independently owns Python source matching, native/pure oracle and detector corpus. Model sort was further delegated exclusively to charset_sort while engine completes tree wiring/large-input handling. Shared signatures and generated-file ownership are in child design.md. No worker adds upstream requests. W9 passed scoped independent source review; final acceptance records the coordinator aggregate checks and finite differential coverage.

## Final offline handoff checkpoint — 2026-11-25

W0–W9 implementation and final independent scope audits are integrated. `research/completion-audit.md` is the requirement-level handoff; acceptance.md records aggregate checks and current-source isolated runtime smoke. L1–L3 retain the independent live-acceptance ownership/dependencies above under D10; they must not add recurring control-plane or CDN probes. W9 closes D11 with pure Go reference detection. Workflow commit confirmation is still pending; this checkpoint is not a commit, deployment or live-account certification.
