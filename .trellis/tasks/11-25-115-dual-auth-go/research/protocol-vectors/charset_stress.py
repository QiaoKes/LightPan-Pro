"""Independent deterministic stress oracle. No network or account modules."""
from __future__ import annotations
import argparse,base64,codecs,gzip,hashlib,json,pathlib,random,sys,platform,unicodedata,locale
from charset_corpus import oracle,TEXTS
P=pathlib.Path(__file__).resolve().parent
SEED=35111520261125

def inputs():
 rng=random.Random(SEED)
 for n in range(129):
  for kind in range(6):
   if kind==0:raw=bytes(rng.randrange(256) for _ in range(n))
   elif kind==1:raw=bytes(rng.randrange(128) for _ in range(n))
   elif kind==2:raw=bytes(rng.randrange(128,256) for _ in range(n))
   elif kind==3:raw=bytes(rng.choice([0,9,10,13,27,32,65,97,127,128,129,255]) for _ in range(n))
   elif kind==4:raw=(b'root\r\n|--'+bytes(rng.randrange(256) for _ in range(n)))[:n]
   else:raw=bytes(rng.choice([0,0,0,32,32,65,97,255]) for _ in range(n))
   yield f'random/{n:03}/{kind}',raw,'short-random'
 aliases=['utf-8','utf_8','UTF8','utf--8','utf__8','utf___8','UTF-16','utf_16_le','utf16le','utf_8_sig','utf-8-sig','cp1252','windows-1252','windows_1252','latin1','latin-1','iso-8859-1','iso8859_1','ISO__8859__1','shift-jis','shift_jis','sjis','ms-kanji','ms_kanji','windows-31j','cp932','gb18030','GBK','gb-2312','gb2312','big5','big5-hkscs','euc-jp','euc_jis_2004','iso-2022-jp','iso2022_kr','koi8-r','macintosh','us-ascii','ascii','unknown','not_a_codec','x-utf-8','UTF\u00c9-8','\u00ffutf8','utf\u0080_8','utf-8\u00ff','utf\u0080-8']
 for i,alias in enumerate(aliases):
  for form in range(8):
   header=[f'# coding: {alias}\n',f'# coding={alias}\n',f'<meta charset="{alias}">\n',f'encoding = "{alias}"\n',f'charset={alias};\n',f'coding: {alias}\n',f'# CoDiNg : {alias}\n',f'<?xml version="1.0" encoding="{alias}"?>\n'][form]
   for payload,enc in [(TEXTS[6],'cp1252'),(TEXTS[1],'gb18030')]:
    raw=header.encode('latin1')+(payload*3).encode(enc)
    yield f'declaration/{i:02}/{form}/{enc}',raw,'declaration-alias'
 encs=['utf_8','utf_8_sig','utf_16','utf_16_le','utf_16_be','utf_32','utf_32_le','utf_32_be','utf_7','gb18030','gbk','big5','big5hkscs','cp932','shift_jis','shift_jis_2004','euc_jp','euc_jis_2004','euc_kr','cp949','johab','hz','iso2022_jp','iso2022_kr']
 for enc in encs:
  texts=[]
  for text in TEXTS:
   try:text.encode(enc)
   except UnicodeError:continue
   texts.append(text)
  text=max(texts,key=lambda s:sum(ord(c)>127 for c in s))
  unit=('root\r\n|--'+text+'\r|--movie.mkv\n').encode(enc)
  for n in [1,2,3,4,7,15,31,63,127,255,511,512,513,1023,2559,2560,2561,4095,4096,8193]:
   raw=(unit*((n//len(unit))+1))[:n]
   yield f'boundary/{enc}/{n}',raw,'encoded-boundary'
  for i in range(7):
   raw=bytearray(unit*2);at=rng.randrange(len(raw))
   if i<3:raw[at:at]=[b'\xff',b'\x00',b'\x1b$'][i]
   elif i<5:del raw[at:at+1+i%2]
   else:raw[at]=rng.randrange(256)
   yield f'malformed/{enc}/{i}',bytes(raw),'encoded-malformed'
 for i in range(160):
  text='root\r\n|--'+rng.choice(TEXTS)+rng.choice(TEXTS)+'\n|--subtitle.srt\r'
  enc=rng.choice(['utf_8','utf_16','utf_16_le','utf_32','gb18030'])
  raw=text.encode(enc)
  if i%4==0:raw=b'\xef\xbb\xbf'+raw
  elif i%4==1:raw=b'\xff\xfe'+raw
  elif i%4==2:raw=raw[:-rng.randrange(1,5)]
  yield f'mixed/{i:03}',raw,'mixed-script-bom'

def main():
 ap=argparse.ArgumentParser();ap.add_argument('--module-root',required=True);ap.add_argument('--kind',required=True);a=ap.parse_args();sys.path.insert(0,a.module_root)
 import charset_normalizer as cn
 from charset_normalizer import md,cd
 assert cn.__version__=='3.5.1'
 source=pathlib.Path('/Users/saki/Desktop/code/ebn/115-reference/charset_normalizer/src/charset_normalizer')
 metadata={'schema_version':1,'version':cn.__version__,'seed':SEED,'python':platform.python_version(),'unicode_version':unicodedata.unidata_version,'locale_encoding':locale.getencoding(),'md_module':md.__file__,'cd_module':cd.__file__,'generator_sha256':hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest(),'source_sha256':{p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in source.glob('*.py')}}
 cases=[];groups={};exceptions=[]
 for cid,raw,group in inputs():
  result=oracle(raw,cn.detect);row={'id':cid,'group':group,'raw_base64':base64.b64encode(raw).decode(),'raw_sha256':hashlib.sha256(raw).hexdigest(),**result};cases.append(row);groups[group]=groups.get(group,0)+1
  if result['exception']:exceptions.append({'id':cid,'exception':result['exception']})
 out=P/f'charset-stress-{a.kind}.json.gz'
 with gzip.GzipFile(filename=str(out),mode='wb',mtime=0) as z:z.write(json.dumps({'metadata':metadata,'cases':cases},ensure_ascii=False,separators=(',',':')).encode())
 summary={'count':len(cases),'groups':groups,'exceptions':exceptions,'metadata':metadata,'fixture_sha256':hashlib.sha256(out.read_bytes()).hexdigest()}
 (P/f'charset-stress-{a.kind}-summary.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary,indent=2))
if __name__=='__main__':main()
