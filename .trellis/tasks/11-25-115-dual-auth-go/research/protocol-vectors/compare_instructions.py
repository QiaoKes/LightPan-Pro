import inspect_bundled as b
import difflib,types,dis
out=[]
for mod,rel in [('p115client.client','p115client/client.py'),('p115cipher.util','modules/p115cipher/p115cipher/util.py')]:
 old={c.co_qualname:c for c in b.walk(b.load(mod))};new={c.co_qualname:c for c in b.walk(compile((b.PINNED/rel).read_text(),rel,'exec'))}
 for name,c in old.items():
  if name not in new or c.co_name not in {'request','download_url','download_url_app','fs_files','fs_video_subtitle','pad_pkcs1_v1_5','rsa_encrypt_with_pubkey','rsa_decrypt_with_pubkey'}:continue
  def instr(c):
   return [f'{i.opname} '+(('<code '+i.argval.co_qualname+'>') if isinstance(i.argval,types.CodeType) else repr(i.argval)) for i in dis.get_instructions(c) if i.opname not in {'RESUME','EXTENDED_ARG'}]
  delta=list(difflib.unified_diff(instr(c),instr(new[name]),fromfile='bundled '+name,tofile='pinned '+name,n=2))
  out.extend(delta)
(b.BASE/'bundled-pinned-instruction-diff.txt').write_text('\n'.join(out)+'\n')
