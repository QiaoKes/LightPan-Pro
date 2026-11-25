# W9 execution

- [x] D11 confirmed; bundled3.5.1 source identity established.
- [x] Freeze shared component signatures and codec inventory.
- [x] Implement engine, mess, coherence, codecs/multibyte independently.
- [x] Validate pinned oracle components and detector outputs, fix differences.
- [x] Integrate tree decode; preserve HTTP call traces.
- [x] Independent trellis-check, targeted/race/full Go checks and Linux build.
- [x] Update spec/notices/acceptance and existing commit manifest.

Python only generates offline evidence and immutable mapping tables. Builds/tests of runtime packages must not launch Python. Live115 testing remains deferred by D10.

## Final offline evidence

All2690 detector/text-reader oracle cases,61 declaration cases,99 tree cases and component vectors pass. The coordinator broad affected-package race run passed after final production fixes; the added five meaningful >10MB fixtures separately pass normal7.583s and race110.682s. Full fixture test peak RSS was~789MiB normal/~2.48GiB race, including corpus JSON/base64/expected text and instrumentation; no isolated detector memory guarantee follows. Goall/vet and pureGo Linux cross-build pass. Frontend sources have not changed since prior type/build verification. Independent source review is parent research/charset-final-check.md; source/data map is charset-implementation.md.

No115 or CDN calls, real credentials, commits, push or deployment. D10 real-account tests remain deferred. Workflow commit confirmation remains pending, so tasks stay unarchived.
