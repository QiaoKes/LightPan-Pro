import gzip,json,random,codecs,pathlib,encodings.aliases,itertools
root=pathlib.Path(__file__).resolve().parents[5];out=root/'drivers/115/client/charset/codecs/unicode_vectors.json.gz';vectors=[]
r=random.Random(115)
names=['utf-7','utf-8','utf-8-sig','utf-16','utf-16-le','utf-16-be','utf-32','utf-32-le','utf-32-be']
for name in names:
 cases=[bytes([i]) for i in range(256)]+[bytes(r.randrange(256) for _ in range(r.randrange(2,20))) for _ in range(2000)]
 cases += [bytes.fromhex(x) for x in ['fffe00d86100','fffe00dc','fffe00d800dc','feffd8000061','0000feff0000d800','fffe000000001100','efbbbf61','efbb','00d800d8']]
 if name=='utf-7':
  cases += [('+'+__import__('base64').b64encode(b''.join(u.to_bytes(2,'big') for u in seq)).decode().rstrip('=')+tail).encode() for size in range(1,4) for seq in itertools.product([0,65,0xd800,0xdbff,0xdc00,0xdfff,0xffff],repeat=size) for tail in ['', '-', '!']]
  cases += [b'+2AA-',b'+3AA-',b'+2ADYAA-',b'+2ADcAA-',b'+3ADYAA-',b'+2AA!',b'+2AB-',b'+2AA',b'+2AAA-']
  cases += [('+'+s+t).encode() for size in range(6) for seq in itertools.product('AZz/+',repeat=size) for s in [''.join(seq)] for t in ['','-','!','=']]
 for raw in cases:
  for ignore in [False,True]:
   try:s=raw.decode(name,'ignore' if ignore else 'strict');err=False
   except UnicodeError:s='';err=True
   vectors.append(dict(name=name,raw=raw.hex(),ignore=ignore,text=s,texthex=s.encode('utf8','surrogatepass').hex(),error=err))
enc=[]
for name in names:
 for s in [''.join(chr(i) for i in range(128)), '日本 😀 +!a-/\\~','日!','日+','日\0','é日本abc','\ufffd','\ufeffx','\ud800','\udc00','x\ud800y','\ud800\udc00']:
  for ignore in [False,True]:
   try: data=s.encode(name,'ignore' if ignore else 'strict');err=False
   except UnicodeError:data=b'';err=True
   enc.append(dict(name=name,text=s,texthex=s.encode('utf8','surrogatepass').hex(),raw=data.hex(),ignore=ignore,error=err))
alias=[]
for n in sorted(set(list(encodings.aliases.aliases)+list(encodings.aliases.aliases.values())+names)):
 for v in [n,n.upper(),n.replace('_','-'),' '+n+' ',n.replace('_','..'),n.replace('_',' / ')]:
  try:c=codecs.lookup(v).name
  except LookupError:c=''
  alias.append(dict(name=v,canonical=c))
out.write_bytes(gzip.compress(json.dumps(dict(decode=vectors,encode=enc,aliases=alias),ensure_ascii=True).encode(),mtime=0))
print(len(vectors),len(alias))
