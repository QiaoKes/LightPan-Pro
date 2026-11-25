# Research: charset detection offline differential oracle

- Query: Validate pure-Go charset-normalizer 3.5.1 against the exact reference detector and final business text-reader behavior.
- Scope: internal / offline official-source differential
- Date: 2026-11-25

## Findings

`research/protocol-vectors/charset_corpus.py` generates 335 deterministic inputs covering all 99 candidate entries (98 canonical codecs, including the original duplicated koi8_r candidate). Every non-ASCII codec receives non-ASCII text. Cases include filename trees, multilingual prose, empty/tiny input, BOMs, malformed bytes, truncated/stateful sequences, controls, declarations, chunk boundaries, and 9,999,999 / 10,000,000 / 10,000,001-byte lazy-decoding thresholds.

The expected fixture is `drivers/115/client/charset/testdata/detection-oracle.json.gz`. Fields preserve actual detection encoding/language/confidence, final Python TextIOWrapper(errors='ignore', newline=None) decoded text, raw bytes and SHA256, and exceptions if any. No intended input-codec label substitutes for the observed detector result. Python3.12.13 locale encoding is UTF-8; null detection encoding therefore follows Python's locale decoder in the final-reader oracle.

The exact official 3.5.1 pure implementation and Mac native md/cd implementation agree on every result for all 335 cases. No codec coverage skips and no oracle exceptions occurred. `charset-corpus-{pure,native}-summary.json` records all results and source/runtime/Unicode hashes; `charset-corpus-comparison.json` records the independent comparison. Bundled Python code-tree equivalence is recorded separately in `charset-source-comparison.json`; Linux bundled native modules cannot execute directly on macOS.

The initial independently captured Go run in `charset-corpus-go-initial.json` has 25 failures while codec implementation is in progress. Most are ISO2022/Johab; koi8 tree samples also expect shift_jis_2004, and truncated shift_jis_2004 fails, so those do not independently establish an engine ranking bug.

`charset_sort_audit.py` intercepts all actual CharsetMatches._ensure_sorted calls during the complete corpus and compares CPython3.12 sorted results with the Go implementation's natural-run plus binary insertion algorithm. `charset-sort-audit.json` records 227 sort calls, at most 22 candidates, and zero order differences. This is stronger than asserting generic stable-sort equivalence: the comparator is nontransitive, and the earlier abstract counterexample proves Go SliceStable can differ from Python. Current short-run algorithm handles those actual corpus sorts.

## Files found

- `/Users/saki/Desktop/code/ebn/115-reference/charset_normalizer/src/charset_normalizer/models.py:265`: CharsetMatches lazy stable sorting, factoring, and best candidate.
- `drivers/115/client/charset/models.go:104`: current short natural-run/binary insertion counterpart, inspected read-only.
- `research/protocol-vectors/charset_corpus.py`: deterministic byte corpus and observed Python reference results.
- `research/protocol-vectors/charset_sort_audit.py`: actual candidate-order audit.

## Related specs

The active task PRD and reference contract require Python-free production execution with reference algorithm fidelity. Python remains isolated offline research/oracle tooling only.

## Caveats / Not Found

The 335 cases do not establish exhaustive charset-normalizer equivalence. Sorting inputs of 64 or more distinct candidates were not observed; full CPython merge behavior remains a separate proof obligation if such candidate lists are reachable. The native parity result concerns official 3.5.1 macOS native modules and matching bundled Python bytecode, not direct execution of the bundled Linux native binaries. Go codec workers were still active during the captured initial run; later harness results supersede its failure count.

## Latest verification

After concurrent codec changes completed, an independent rerun of `go test ./drivers/115/client/charset -run TestDetectionOracle -count=1` passed all 335 cases in 5.464 seconds. This supersedes the initial 25-failure snapshot; no expected fixture values were altered to match Go output.

## Independent randomized/default stress extension

`protocol-vectors/charset_stress.py` adds a separate 2,350-case deterministic corpus using seed 35111520261125; the original 335-case fixture remains unchanged. Groups are 774 random byte samples (six distributions for every length 0 through 128), 768 declarations (48 aliases/forms including repeated underscores, invalid labels, and high-ASCII bytes), 480 encoded chunk boundaries, 168 malformed/stateful sequences, and 160 multilingual mixed-script/BOM cases. The cases retain observed detector results rather than the intended encoding of the generated source text.

The exact Python3.12.13 / charset-normalizer3.5.1 pure and macOS native implementations agree on all 2,350 cases, including encoding, language, confidence and final TextIOWrapper decoded text. No exceptions occurred. Reproducible scripts, source hashes, runtime metadata, complete expected results and summaries are in `charset-stress-{pure,native}.json.gz`, `charset-stress-{pure,native}-summary.json`, and `charset-stress-comparison.json`. Native fixture SHA256 is `7dc2074151a5e31919d8c26e288ec90d1d94dba6b3afb57f98d8cf84fbccc532`.

`charset_go_probe.go` is a research-only public-API comparison driver. Running it against the new native fixture produced `charset-stress-go-initial.json`: 2,350 checked cases and zero mismatches across detection found/encoding/language/confidence and final decoded text. The implementation's declaration fixes were already present at this check; no expected values were modified to satisfy Go.

Additional intercepted candidate sorting recorded in `charset-stress-sort-audit.json` covers 1,239 sort calls with a maximum of 18 candidates and zero CPython short-sort differences. Together with the original corpus, 2,685 independent inputs and 1,466 actual sort operations have passed. This remains differential coverage rather than an exhaustive proof; no observed input exercises a 64-or-more candidate sort.

## Meaningful large legacy validation extension

`charset_large.py` separately generates five cases exceeding 10,000,000 bytes, preserving the previous 2,685 cases. Native and pure reference results and final decoded text agree exactly for all five, and the independent public Go API probe passes all five (`charset-large-go.json`). Cases and observed outputs:

- 10,000,019-byte cp1251 text: windows-1251 / Russian.
- Same payload with undefined cp1251 byte 0x98 at offset 600001: ptcp154 / Russian.
- 10,000,019-byte cp1252 text: windows-1250 / Hungarian (observed detector result, not intended source codec).
- Same payload with undefined cp1252 byte 0x81 at offset 10000012: iso8859_10 / Hungarian.
- 10,000,080-byte complete-unit GB18030 Chinese text: GB18030 / Chinese.

A narrow `sys.settrace` observation of official `api.py` line 344, 482 and 490 directly confirms execution of the 500000-byte initial single-byte validation, strict validation of `sequences[50000:]`, and hard-rejection paths. The invalid cp1251 case hard-rejects cp1251, koi8_t and kz1048 at the suffix-validation step; the invalid cp1252 case hard-rejects cp1250, cp1252, cp1254, cp1257 and cp1258 there. These are not ASCII early-return size tests. All confidence results are 1.0; no exceptions occurred.

`charset-large-{pure,native}-summary.json` records per-case elapsed time and process peak RSS (macOS bytes; cumulative high-water mark, not isolated per-case allocation). Native detections with tracing take about 0.05–0.43 seconds per case and process peak reaches approximately 568 MB including retained full fixture results. Native fixture SHA256 is `6bffe780166eb75b9f19a6eb04c16566123fad33c0b62e00bcc951865284358e`. `charset-large-comparison.json` records reference equality. All 2,690 independently generated input cases now match Go; this remains finite differential coverage.
