# W9 architecture and ownership

Package drivers/115/client/charset (package charset) provides Detect([]byte) Detection and Decode([]byte)(string,error); Detection {Encoding, Language string; Confidence float64; Found bool}. The existing tree parser invokes Decode once. No protocol transport changes. Supporting package charset/codecs implements Python codec behavior entirely in Go using pinned generated mapping data as necessary. Offline generators/oracles stay under parent research/protocol-vectors, excluded from runtime.

## Nonoverlapping owners
- charset_engine: charset/detect.go, detector_utils.go, detector_constants.go and detector/tree tests. Owns API/legacy candidate orchestration and final Decode; integrates client/tree.go only after components ready.
- charset_sort: exclusively charset/models.go and sorting tests/generator. Ports model comparison/factoring and CPython natural-run/insertion/gallop/merge ordering for the default candidate bound of at most 99 distinct results; 3,000 full-order oracle cases cover sizes through100.
- charset_mess: charset/mess.go, unicode.go, unicode generated tables/data and mess tests; offline unicode generator. Port exact md.py and Unicode utility semantics.
- charset_coherence: charset/coherence.go, language constants/generated data, coherence tests; offline language generator. Port exact cd.py and language/frequency data.
- charset_codecs: charset/codecs/codec.go, aliases/singlebyte/unicode codec files and generated data/tests; offline codec generator. Owns codec public contract, all aliases/candidate classification and stateless singlebyte/unicode encodings.
- charset_multibyte: charset/codecs/multibyte*.go, multibyte/stateful generated tables/data/tests and offline multibyte generator. Owns all non-Unicode multibyte/stateful codec implementations; no generic substitute missing codecs.
- charset_iso2022: exclusively charset/codecs/iso2022.go, iso2022_* assets/tests and offline generator; seven ISO-2022 decoders from pinned CPython C source. Multibyte owner retains their encoders.
- protocol_baseline: parent charset provenance, independent Python oracle/vector evidence and differential/native-vs-pure validation; no production writes.
- root: task docs/spec/notice integration and final independent checking; no shared production edits while agents own files.

## Shared signatures (freeze before dependent edits)
codecs.Decode(raw []byte, name string, ignoreErrors bool)(string,error); codecs.Encode(text string,name string,ignoreErrors bool)([]byte,error); codecs.Canonical(name string)(string,error); codecs.Supported()[]string; codecs.IsMultiByte(name string)bool. Internal multibyte functions decodeMulti(raw []byte,name string,ignoreErrors bool)(string,error), encodeMulti(text string,name string,ignoreErrors bool)([]byte,error), isMultiByte(name string)bool. These accept canonical Python names. Base codec owner supplies canonicalization and routes UTF codecs itself; multibyte owner handles other multibyte/stateful codecs.

MessRatio(text string, maximumThreshold float64)float64. CoherenceRatio(text string,threshold float64,lgInclusion []string)[]CoherenceMatch; CoherenceMatch {Language string; Ratio float64}; MergeCoherenceRatios([][]CoherenceMatch)[]CoherenceMatch; EncodingLanguages(name string)[]string; MBEncodingLanguages(name string)[]string. Coherence owner owns type.

Unicode shared API/struct: mess owner immediately publishes concrete character-info accessors/field names to coherence owner. Python property semantics, case mapping, NFD/accent/range families must use pinned Unicode version, not silently use a different Go Unicode version. Thread-safe bounded/pure caches; no mutable unsafe shared scratch.

Agents are not alone; do not revert others. No nested implement/check agents. Communicate interface changes before writes. Never add temporary production stubs as completed behavior.
