"""Offline-only inspection: never imports client/account/network modules."""
import marshal,types,dis,pathlib,json,io,hashlib,struct,zlib
BASE=pathlib.Path(__file__).resolve().parent
VENDOR=pathlib.Path('/Users/saki/Desktop/code/ebn/restored-python/vendor')
PINNED=pathlib.Path('/Users/saki/Desktop/code/ebn/115-reference/p115client')
def load(name):return marshal.loads((VENDOR/(name.replace('.','/')+'.pyc')).read_bytes()[16:])
def walk(c):
 yield c
 for v in c.co_consts:
  if isinstance(v,types.CodeType):yield from walk(v)
mods={'p115client.client':{'P115Client','get_request','request','download_url','download_url_app','fs_files','fs_dir_getid','fs_video_subtitle'},'p115client.util':{'complete_url'},'p115cipher.__init__':None,'p115cipher.util':None}
for name,names in mods.items():
 c=load(name);buf=io.StringIO()
 for x in walk(c):
  if (names is None or x.co_name in names):
   print('QUALNAME',x.co_qualname,file=buf);dis.dis(x,file=buf,depth=0)
 (BASE/('actual-'+name+'.dis.txt')).write_text(buf.getvalue())
archive=pathlib.Path('/Users/saki/Desktop/code/ebn/image-analysis/unpacked/PYZ.pyz').read_bytes()
toc=dict(marshal.loads(archive[struct.unpack('!i',archive[8:12])[0]:]))
match={}
for name in ['p115client','p115client.client','p115client.util','p115cipher','p115cipher.util','p115cipher.aes']:
 _,pos,size=toc[name];raw=zlib.decompress(archive[pos:pos+size]);rel=name.replace('.','/')+('/__init__.pyc' if name in ['p115client','p115cipher'] else '.pyc');vendor=(VENDOR/rel).read_bytes()[16:]
 match[name]={'pyz_payload_sha256':hashlib.sha256(raw).hexdigest(),'vendor_payload_sha256':hashlib.sha256(vendor).hexdigest(),'equal':raw==vendor}
(BASE/'pyz-vendor-match.json').write_text(json.dumps(match,indent=2)+'\n')
# Compare marshalled function trees against pinned source compiled with exact runtime.
def sig(c):
 return (c.co_code,c.co_names,c.co_varnames,c.co_argcount,c.co_posonlyargcount,c.co_kwonlyargcount,tuple(sig(x) if isinstance(x,types.CodeType) else x for x in c.co_consts))
comparison={}
for mod,rel in [('p115cipher.__init__','modules/p115cipher/p115cipher/__init__.py'),('p115cipher.util','modules/p115cipher/p115cipher/util.py'),('p115client.client','p115client/client.py'),('p115client.util','p115client/util.py')]:
 old={x.co_qualname:x for x in walk(load(mod))};new={x.co_qualname:x for x in walk(compile((PINNED/rel).read_text(),rel,'exec'))};comparison[mod]={name:sig(x)==sig(new[name]) for name,x in old.items() if name in new and (mod.startswith('p115cipher') or x.co_name in {'get_request','complete_url','download_url','download_url_app','request','fs_files','fs_dir_getid','fs_video_subtitle'})}
(BASE/'bundled-pinned-function-comparison.json').write_text(json.dumps(comparison,indent=2)+'\n')
