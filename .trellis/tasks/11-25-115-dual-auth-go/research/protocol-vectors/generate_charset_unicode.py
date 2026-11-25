"""Offline Unicode 15.0/Python 3.12 tables for charset-normalizer 3.5.1."""
import sys,json,gzip,pathlib,unicodedata
sys.path.insert(0,'/Users/saki/Desktop/code/ebn/115-reference/charset_normalizer/src')
assert sys.version_info[:2] == (3, 12), "Run with pinned CPython 3.12"
assert unicodedata.unidata_version == "15.0.0"
from charset_normalizer.md import CharInfo,is_suspiciously_successive_range
from charset_normalizer.constant import _SECONDARY_RANGE_NAMES,_RANGE_FAMILIES
fields='printable alpha upper lower space digit is_ascii case_variable accentuated latin is_cjk is_katakana is_halfwidth_katakana is_arabic is_ligature is_superscript is_sentence_open_punctuation is_glyph punct sym sep emoticon safe common_cjk'.split()
rows=[];lower={};unaccent={};cased=[];ignorable=[];remove={}
for n in range(0x110000):
 c=chr(n)
 if c.lower()!=c.upper(): cased.append(n)
 if ('AΣ'+c+'A').lower()[1]=='σ' and ('AΣ'+c+'.').lower()[1]=='ς': ignorable.append(n)
 decomp=unicodedata.decomposition(c)
 if decomp and not decomp.startswith('<'): remove[n]=chr(int(decomp.split()[0],16))
 i=CharInfo(c); bits=sum(int(getattr(i,f))<<k for k,f in enumerate(fields)); val=[bits,i.flags,i.range or '']
 if rows and rows[-1][1]==n and rows[-1][2:]==val: rows[-1][1]=n+1
 else: rows.append([n,n+1,*val])
 if c.lower()!=c:lower[n]=c.lower()
 if i.unaccented!=c:unaccent[n]=i.unaccented
names=list(_RANGE_FAMILIES)
susp=[[a,b] for a in names for b in names if is_suspiciously_successive_range(a,b)]
data=dict(cased=cased,ignorable=ignorable,remove=remove,rows=rows,lower=lower,unaccent=unaccent,secondary=list(_SECONDARY_RANGE_NAMES),susp=susp)
p=pathlib.Path('drivers/115/client/charset/unicode_data.json.gz');p.write_bytes(gzip.compress(json.dumps(data,ensure_ascii=True,separators=(',',':')).encode(),mtime=0))
import sys,json,random
sys.path.insert(0,'/Users/saki/Desktop/code/ebn/115-reference/charset_normalizer/src')
from charset_normalizer.md import mess_ratio
random.seed(51)
texts=['','plain text','\x1bhello','éÉéÉéÉéÉ','中文測試漢字龘龘','ｱｲｳｴ龘龘龘','hello Ωworld','a'*600,'\x00'*40,'aBéDéFgH '*20,'السلام عليكم','こんにちは世界']
chars='abcXYZ éÉàΩωДд中文龘ｱｲｳﷺ\x00\x1b\n !?+€²æ¿'
for _ in range(200): texts.append(''.join(random.choices(chars,k=random.randrange(1,1500))))
rows=[{'Text':s,'Threshold':t,'Want':mess_ratio(s,t)} for s in texts for t in [.0,.2,1.,10.]]
pathlib.Path('drivers/115/client/charset/mess_vectors_test.json.gz').write_bytes(gzip.compress(json.dumps(rows,ensure_ascii=True).encode(),mtime=0))

# JSON codepoint lists avoid JSON implementations replacing lone surrogates.
surrogate_texts = ['\ud800', '\udfff', 'a\ud800b', '\ud800\udfff',
                   'AΣ\ud800A', 'AΣ\u0301\udfff', '\ud800' * 40,
                   'abc' * 170 + '\ud800' + 'ΩΣ' * 100,
                   '漢字\ud800ｱｲｳｴ龘龘龘', 'éÉ\udfff' * 350]
surrogate_rows = [dict(Runes=list(map(ord, s)), Lower=list(map(ord, s.lower())),
                       Threshold=t, Want=mess_ratio(s, t))
                  for s in surrogate_texts for t in [0.0, 0.2, 1.0, 10.0]]
pathlib.Path('drivers/115/client/charset/surrogate_vectors_test.json').write_text(
    json.dumps(surrogate_rows, separators=(',', ':')))
