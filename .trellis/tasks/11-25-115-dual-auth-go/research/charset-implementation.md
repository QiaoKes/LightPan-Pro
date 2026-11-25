# W9: pure Go directory-tree character detection

User decision D11 requires closing the former UTF8/BOM UTF16/GB18030 approximation. The implementation follows the reference's default `charset_normalizer.legacy.detect` followed by Python text-open decoding with `errors='ignore'` and universal newlines. It does not substitute another detector or add any remote request.

## Pinned sources

- charset-normalizer 3.5.1, commit `e239bdc5cc1eb1f0db08d4046ad531f805dbea71`, cloned under `/Users/saki/Desktop/code/ebn/115-reference/charset_normalizer`.
- CPython 3.12.13, commit `3bb231a6a5dc02b95658877318bf61501a7209e9`, cloned under `/Users/saki/Desktop/code/ebn/115-reference/cpython`.
- The complete compiled code trees for bundled `__init__`, version, legacy, api, utils, models and constant match the pinned source, ignoring location metadata. See `protocol-vectors/charset-source-comparison.json`.
- Official same-tag macOS native scoring and pure Python scoring agree on the independent 335-input corpus. This does not claim direct execution of the bundled Linux native modules on macOS.

## Source-to-Go mapping

| Source | Go owner/files | Contract |
|---|---|---|
| api.py / legacy.py | charset/detect.go | Default sampling, candidate ordering, strict decode, early exits, similar-codec skips, fallback, legacy names and confidence |
| utils.py / constant.py | detector_utils.go / detector_constants.go | Declarations, signatures, chunk coordinates and fixed candidate data |
| models.py / CPython listobject.c | models.go | Equal-text factoring, language/usage ranking, CPython natural runs, insertion, gallop and merge behavior |
| md.py / Unicode 15.0.0 | mess.go / unicode.go / unicode_data.json.gz | All ten mess plugins and pinned Python character properties/case/accent behavior |
| cd.py / language profiles | coherence.go / languages_generated.go | Forty-one ordered profiles, encoding-language associations, ranking and stable merge |
| Python float round | numeric.go | Binary-float decimal ties and operation order, rather than Go rounding assumptions |
| Python encodings / cjkcodecs | codecs/ | Single-byte maps, Unicode codecs, multibyte/stateful decode/encode, strict/ignore recovery |
| _codecs_iso2022.c | codecs/iso2022.go | Seven ISO-2022 decoders, escape/shift states and error boundaries |
| Reference tree reader | client/tree.go | One local charset.Decode call before the existing stack parser |

`Detect` exposes the default business operation, not every configurable API in the Python library. `Decode` preserves declarations and reproduces the final text-reader operation, including UTF8 as the reference Linux locale fallback when no encoding is returned. UTF7 can yield lone surrogates; internal WTF8 representation and Python rune helpers preserve their scoring/count behavior without silently inserting replacement characters.

## Validation inventory

All expected values come from pinned Python execution, not intended input-codec labels. Tiny GB18030 text can, for example, be detected as Big5 by the reference; tests preserve that observed result.

- 335 complete detector cases cover all 99 candidate entries / 98 distinct encodings, final decoded text, multilingual filenames/prose, malformed/stateful input, BOMs, declarations, sampling boundaries, and 9,999,999 / 10,000,000 / 10,000,001-byte thresholds. An additional independent 2,350-input stress corpus covers random bytes, mixed scripts, malformed streams and 768 declaration/prefilter cases; pure/native expected results and actual Go output agree. Existing335 expected values were not changed.
- Sixty-one focused declaration cases verify the exact `_IANA_NAMES` mapping, byte-level prefilter, UTF names and repeated-underscore behavior. Source review found and fixed a codec-lookup versus detector-name mismatch before delivery.
- 99 independent decoded-tree cases compare the resulting leaf paths with the Python stack parser.
- 3,000 full-order model cases cover sizes 0–100, comparator thresholds, nontransitive comparisons, natural runs, merge/gallop paths and large-input ranking. The default inventory has98 distinct names; signatures/ASCII/UTF8 already belong to it, and one declaration adds at most one. Deduplication therefore bounds candidate results at99. CPython minrun at64–99 leaves at most two runs and one merge. This is not a general-purpose Python sorter.
- Component vectors include 74,430 Unicode decode cases, 216 encode cases, 2,598 aliases, all 66 single-byte tables, 12,264 multibyte decode cases, 1,392 encode cases, and 21,084 dedicated ISO-2022 cases.
- Scoring evidence includes 848 mess cases, 40 surrogate cases plus all 2,048 surrogate round trips, coherence/projection boundaries and 14,590 bitwise Python rounding cases.

Five additional meaningful inputs exceeding10MB bring whole-detector coverage to2,690 cases. They include cp1251, cp1252 and GB18030 text and undefined bytes beyond the initial probe. Captured Python traces prove500000-byte prefix validation, suffix validation from50000 and subsequent hard rejection; Go matches the observed fallback encodings. See charset-large summaries and charset-large-go.json.

Encoder tables are immutable compact binaries and loaded separately on demand. The codec worker measured full decoder/encoder test peak RSS at approximately 231 MB, reduced from 717 MB during development. This is a codec test-process measurement, not a production memory guarantee. The five large detector fixtures measured approximately789MiB peak across the Go command/tests, including simultaneous JSON/base64 raw inputs and expected decoded text; the focused race run passed in110.682s with approximately2.48GiB test peak RSS. These are not isolated detector allocation measurements. Large single-byte detection retains lazy candidate payloads, matching the source threshold behavior.

## Integration boundary

Export, polling, export-file download and deletion ordering are unchanged. Character detection executes after the body is acquired and the reference cleanup boundary; it cannot create HTTP probes, retries, health checks or player requests. Runtime and tests use Go and embedded immutable tables; Python generators and the ignored research interpreter are offline tooling only.

Independent source review and the expanded malformed-input differential run passed; see charset-final-check.md. This document records finite implementation evidence, not exhaustive equivalence or account risk-control guarantees. Real115 account/player acceptance remains deferred under D10. Raw slash-name ambiguity already lost by the reference export producer remains inherited.
