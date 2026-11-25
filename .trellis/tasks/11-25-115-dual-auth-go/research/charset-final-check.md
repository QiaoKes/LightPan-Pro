# Independent charset final review

2026-11-25. Scoped review of W9, with one production embed fix.

## Findings
No concrete defect established in inspected orchestration/model/chunk paths. Detect/Decode retains legacy confidence adjustment, UTF8 signature alias, UTF8 no-result fallback, ignore decoding and universal newlines. Tree invokes charset.Decode locally; charset production contains no HTTP or exec.Command calls.

Independent bound check: detector_constants.go has 99 entries, 98 distinct encodings (koi8_r repeats). tested deduplicates names before admission. A declaration may add at most one canonical encoding; signature encodings are already in that set. Thus result count <=99, including before text-equivalence folding. For n=64..99 CPython minrun is ceil(n/2), so extending the first natural run to minrun leaves <=49 entries, and exactly one merge suffices. The specialized two-run implementation is bounded for the actual default API. It is not a general list.sort replacement for arbitrary caller-supplied candidates.

Source inspection compared candidate skip/defer/prefix-validation and UTF7 BOM handling to pinned api.py, and inspected model comparison/gallop/merge and sequenceChunks. >=10MB disables text deduplication and multibyte-usage tie ranking; retained decoding pointers are iteration-local, while accepted nonpriority large candidates clear text. Prefix validation uses 500000 bytes and tail validation starts at50000 as reference. These unusual differing constants are intentional.

## Verification
- go test ./drivers/115/client/charset/... ./drivers/115/client: PASS (cached).
- go vet ./drivers/115/client/charset/... ./drivers/115/client: PASS (completed before race stage).
- Race run: PASS, charset34.477s/codecs12.812s/client9.454s (before embed-only fix).

## Limits
This is a scoped source review, not an exhaustive proof of every generated codec table, every Unicode property, or all possible Python malformed-stream semantics. Existing differential fixtures provide finite evidence only. No live115 calls, credential access or production Python execution performed. Root retains aggregate build/type/frontend gates and provenance/license acceptance. No new production interfaces or behavior changes made.

Fixed: multibyte.go now enumerates JSON production tables explicitly, excluding304KiB of test oracle fixtures previously matched by the wildcard. No generator/test path changes needed.

Unresolved finding sent to root: specifiedEncoding uses codecs.Canonical, which returns hyphenated UTF names while orchestration uses underscores; Python iana_name also differs in repeated-underscore normalization. Requires focused differential coverage/fix by engine owner.

## Follow-up review and completed post-fix checks
- Production embed exclusion initially exposed tests depending on the production embedded FS. Updated multibyte_test.go to read its local oracle files with os.Open. No oracle/generator paths changed.
- Post-fix go test ./drivers/115/client/charset/... ./drivers/115/client PASS (charset3.913s, codecs2.055s, client2.169s); subsequent scoped go vet PASS.
- Read remaining default api.py branches through definitive modes, signature selection and fallback ordering: matches Go flow. Source's redundant UTF fallback predicate simplifies to fallbackUTF!=nil, as implemented.
- Compared full cut_sequence_chunks: iso2022 decoded-coordinate integer scaling, deferred base slicing, raw chunk-end guard, negative-index adjustment and BOM reconstruction match. Omitted nearby search is a performance-only shortcut: source still tests global substring if nearby search fails, so global containment yields the identical adjustment condition. No encoding/HTTP fallback added.
- Reviewed coherence language-profile selection, stable frequency order, projection/rank arithmetic and Unicode-backed alpha layering; no additional concrete mismatch established. This does not independently rederive all generated Unicode tables or exhaustively validate every codec transition.
- Read prior integration-audit.md as historical evidence. It explicitly describes earlier unresolved playback issues; this review does not treat that historical snapshot as proof they are currently resolved. Current W9 cross-layer delta is confined to local tree decoding and leaves cloud export/request ownership unchanged; coordinator owns broad final acceptance.
- Coordinator assigned the declaration-normalization defect to the engine owner. Do not interpret this report as a final PASS until that correction and expanded default differential fixtures have passed.

## Declaration follow-up
Engine corrected declaration lookup to pinned _IANA_NAMES and byte prefilter. Reviewed current declarations.json:61 rows, zero non-null exceptions. Ran offline Python source probes for unicode_escape/raw_unicode_escape/base64_codec/rot13/hex_codec declarations with ordinary ASCII, malformed escape and128 high bytes: no escaped exception. There is no demonstrated reason to change Detect's public error contract or reject all such declarations. Blanket rejection would introduce unsupported behavior. Any future exception oracle must assert Decode failure rather than skip acceptance.

Default candidate bound remains98 unique frozen candidates plus at most one declared canonical name =99; priority ASCII/UTF8 and all signature names are members of the98. No caller-controlled candidate list is exposed by this business API. The specialized sorter is justified within that default bound only.

Final scoped rerun after engine declaration integration and detection-stress-oracle.json.gz registration: go test ./drivers/115/client/charset/... ./drivers/115/client PASS (charset6.273s; codecs/client cached). Stress fixture test is registered in detector_oracle_test.go and included in that run. Coordinator reports2350 additional pure/native differential cases; this reviewer inspected registration/execution, not independently regenerated all oracle outputs. No remaining established W9 defect in this review. Scope is business default detection/tree decoding, not general comparative-library compatibility.

## Final large-input gate
Independently ran go test ./drivers/115/client/charset -run TestDetectionLargeOracle -count=1: PASS7.950s. Harness asserts exactly5 cases and rejects unexpected oracle exceptions. Cases cover meaningful10MB single-byte legacy text, high-byte invalidity beyond initial500000-byte validation, and GB18030 multibyte content, rather than ASCII-only early returns. Coordinator supplied pure/native pinned comparison evidence in charset-large-{pure,native}.json.gz and charset-large-go.json; source prefix/tail offsets already checked above. Declaration mismatch and production fixture embedding are resolved. Scoped review PASS with finite fixture/source-inspection limits stated above; broad aggregate/race acceptance belongs coordinator.


## Coordinator aggregate closure

After final production fixes, go test ./..., go vet ./..., broad affected-package go test -race and CGO_ENABLED=0 GOOS=linux GOARCH=amd64 build passed. The five subsequently integrated large oracle fixtures separately pass normal7.583s and race110.682s. No production changes followed those aggregate commands. Test fixture process peaks are not isolated production memory measurements. D10 live tests remain deferred; no zero-risk or exhaustive-equivalence claim is made.
