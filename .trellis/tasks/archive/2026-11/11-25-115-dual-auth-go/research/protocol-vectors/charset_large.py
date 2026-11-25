"""Large legacy-byte decoding oracle with direct API branch coverage tracing."""
import argparse,base64,gzip,hashlib,json,pathlib,sys,time,resource,platform,locale
from charset_corpus import TEXTS,oracle
P=pathlib.Path(__file__).resolve().parent
N=10_000_019

def cases():
 for enc,text in [('cp1251',TEXTS[5]),('cp1252',TEXTS[6])]:
  unit=('root\r\n|--'+text+'movie.mkv\n').encode(enc)
  raw=(unit*((N//len(unit))+1))[:N]
  yield enc+'/valid',raw
  bad=bytearray(raw);at=600_001 if enc=='cp1251' else N-7;bad[at]=0x98 if enc=='cp1251' else 0x81
  yield enc+'/invalid-'+str(at),bytes(bad)
 unit=('root\r\n|--'+TEXTS[1]+'movie.mkv\n').encode('gb18030')
 # Preserve complete final multibyte units while exceeding the large threshold.
 raw=unit*((N//len(unit))+1)
 yield 'gb18030/valid',raw

def main():
 ap=argparse.ArgumentParser();ap.add_argument('--module-root',required=True);ap.add_argument('--kind',required=True);a=ap.parse_args();sys.path.insert(0,a.module_root)
 import charset_normalizer as cn
 from charset_normalizer import api,md,cd
 assert cn.__version__=='3.5.1'
 rows=[];records=[];current={};api_file=api.__file__
 def trace(frame,event,arg):
  if frame.f_code.co_filename!=api_file:return None
  if event=='line' and frame.f_lineno in (344,482,490):
   enc=frame.f_locals.get('encoding_iana');key={344:'prefix500000',482:'suffix_from50000',490:'suffix_hard_reject'}[frame.f_lineno]
   if enc is not None:current.setdefault(key,[]).append(enc)
  return trace
 for cid,raw in cases():
  current={};start=time.perf_counter()
  # Trace only native: no additional expected result calculation or altered API.
  if a.kind=='native':sys.settrace(trace)
  result=oracle(raw,cn.detect)
  sys.settrace(None)
  seconds=time.perf_counter()-start
  rows.append({'id':'large-legacy/'+cid,'group':'large-legacy','raw_base64':base64.b64encode(raw).decode(),'raw_sha256':hashlib.sha256(raw).hexdigest(),**result})
  records.append({'id':cid,'size':len(raw),'encoding':result['encoding'],'language':result['language'],'confidence':result['confidence'],'exception':result['exception'],'seconds':seconds,'peak_rss':resource.getrusage(resource.RUSAGE_SELF).ru_maxrss,'api_branch_candidates':current})
  print(json.dumps(records[-1]),flush=True)
 metadata={'schema_version':1,'version':cn.__version__,'python':platform.python_version(),'locale_encoding':locale.getencoding(),'md_module':md.__file__,'cd_module':cd.__file__,'generator_sha256':hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest()}
 out=P/f'charset-large-{a.kind}.json.gz'
 with gzip.GzipFile(filename=str(out),mode='wb',mtime=0) as z:z.write(json.dumps({'metadata':metadata,'cases':rows},ensure_ascii=False,separators=(',',':')).encode())
 (P/f'charset-large-{a.kind}-summary.json').write_text(json.dumps({'metadata':metadata,'cases':records,'fixture_sha256':hashlib.sha256(out.read_bytes()).hexdigest()},indent=2)+'\n')
if __name__=='__main__':main()
