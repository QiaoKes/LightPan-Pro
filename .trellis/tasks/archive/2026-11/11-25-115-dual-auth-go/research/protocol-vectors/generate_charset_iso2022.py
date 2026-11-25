"""Generate glyph mappings from pinned CPython 3.12.13 (offline only)."""
import json,gzip,pathlib,sys
assert sys.version_info[:3] == (3,12,13), "requires pinned CPython3.12.13"
root=pathlib.Path('drivers/115/client/charset/codecs')
configs={'jp':'BJ@','jp_1':'BDJ@','jp_2':'BDCAJ@AF','jp_2004':'BQP','jp_3':'BOP','jp_ext':'BDJI@','kr':'C'}
allmaps={}
for suffix,marks in configs.items():
 name='iso2022_'+suffix; maps={}
 for mark in set(marks):
  single=mark in 'JI' or (suffix=='jp_2' and mark in 'AF')
  prefix=(b'\x1b.'+mark.encode()) if suffix=='jp_2' and mark in 'AF' else (b'\x1b('+mark.encode() if single else b'\x1b$'+mark.encode())
  if suffix=='kr': prefix=b'\x1b$)C\x0e'
  m={}
  for a in range(256):
   for b in (range(256) if not single else [None]):
    raw=bytes([a]) if b is None else bytes([a,b])
    # Only glyph positions: control/escape processing is handled by the parser.
    if a<32 or a>=128: continue
    try: out=(prefix+(b'\x1bN' if suffix=='jp_2' and mark in 'AF' else b'')+raw).decode(name)
    except UnicodeDecodeError: continue
    if out: m[str(a if b is None else a*256+b)]=out
  maps[mark+('s' if single else 'd')]=m
 allmaps[name]=maps
with open(root/'iso2022_maps.json.gz','wb') as f: f.write(gzip.compress(json.dumps(allmaps,ensure_ascii=True,separators=(',',':')).encode(),mtime=0))
import random
rng=random.Random(2511); vectors=[]
for name in allmaps:
 samples=[b'',b'\x1b',b'\x1b$B$"',b'\x1b&@\x1b$B$"',b'\x1b$)C\x0e0!\n0!',b'\x1b.A\x1bN!']
 for _ in range(1500):
  samples.append(bytes(rng.choices([27,36,40,41,46,38,64,66,67,74,78,14,15,10,32,127,128,255],k=rng.randrange(35))))
 for raw in samples:
  for ignore in [False,True]:
   try: s=raw.decode(name,'ignore' if ignore else 'strict');err=False
   except (UnicodeDecodeError,RuntimeError):s='';err=True
   vectors.append([name,raw.hex(),ignore,s,err])
with open(root/'iso2022_vectors.json.gz','wb') as f:f.write(gzip.compress(json.dumps(vectors,ensure_ascii=True).encode(),mtime=0))
