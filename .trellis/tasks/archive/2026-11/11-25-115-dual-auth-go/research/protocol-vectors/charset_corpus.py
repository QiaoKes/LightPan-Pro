"""Deterministic offline charset-normalizer 3.5.1 oracle, no account/network modules.

Run with --module-root <official source src OR research charset-native>.
Final text is Python TextIOWrapper(errors='ignore',newline=None), matching business open.
"""
from __future__ import annotations
import argparse,base64,codecs,gzip,hashlib,io,json,locale,pathlib,platform,random,sys,unicodedata
P=pathlib.Path(__file__).resolve().parent
SOURCE=pathlib.Path('/Users/saki/Desktop/code/ebn/115-reference/charset_normalizer')
VENDOR=pathlib.Path('/Users/saki/Desktop/code/ebn/restored-python/vendor/charset_normalizer')
TEXTS=[
 'The quiet library contains films, music, photographs and subtitles. Every folder has a useful name. ',
 '目录中的电影、照片与字幕按年份分类，春天的故事在这里开始，文件名称保存完整。',
 '圖書館收藏電影和字幕，每個目錄都有完整名稱。春天的故事在這裡開始。',
 '日本語の映画と字幕を保存する場所です。静かな図書館には新しい物語があります。',
 '한국어 영화와 자막을 보관하는 폴더입니다. 아름다운 이야기를 다시 읽습니다. ',
 'В тихой библиотеке хранятся фильмы, фотографии и субтитры. Каждый каталог имеет название. ',
 'Les fichiers du cinéma présentent une histoire très intéressante avec des caractères accentués. ',
 'Die schönen Bücher und Filme erzählen eine größere Geschichte über das tägliche Leben. ',
 'الكتب والأفلام الموجودة في المكتبة تروي حكايات جميلة عن الحياة والأصدقاء. ',
 'הספרים והסרטים בספרייה מספרים סיפורים יפים על החיים ועל אנשים טובים. ',
 'Η βιβλιοθήκη περιέχει ταινίες και φωτογραφίες με όμορφες ιστορίες για τη ζωή. ',
 'การจัดเก็บภาพยนตร์และคำบรรยายไว้ในห้องสมุดช่วยให้ค้นหาเรื่องราวได้ง่ายขึ้น ',
 'İstanbul şehrindeki güzel filmler ve kitaplar günlük hayat hakkında öyküler anlatır. ',
]
def cases(candidate_order):
 seen=set();coverage=[]
 def emit(cid,raw,**tags):
  assert cid not in seen,cid
  seen.add(cid);return {'id':cid,'raw':raw,**tags}
 for candidate_index,enc in enumerate(candidate_order):
  codec=codecs.lookup(enc).name
  representable=[]
  for text in TEXTS:
   try:
    raw=text.encode(enc);assert isinstance(raw,bytes)
   except (UnicodeError,LookupError,TypeError):continue
   representable.append(text)
  if not representable:
   coverage.append({'candidate_index':candidate_index,'candidate':enc,'actual_codec':codec,'skipped':'no representable text'})
   continue
  # Prefer non-ASCII text, with longest script-rich representative as stable tie breaker.
  text=max(representable,key=lambda t:(sum(ord(c)>127 for c in t),len(t)))
  if enc != 'ascii' and not any(ord(c)>127 for c in text):
   repertoire=[]
   for value in range(128,256):
    try:char=bytes([value]).decode(enc,'strict')
    except UnicodeError:continue
    if len(char)==1 and ord(char)>127 and char.isalpha():repertoire.append(char)
   if repertoire:text=''.join(repertoire)+' '+text
  filename=''.join(c for c in text if not c.isspace())[:22]
  tree='root\r\n|--'+filename+'\r\n| |--01_movie.mkv\r| |--01_movie.nfo\n| |--01_subtitle.srt\r\n'
  prose=text*32
  yield emit('codec/%03d/'%candidate_index+enc+'/tree',tree.encode(enc),group='codec-tree',input_codec=enc)
  yield emit('codec/%03d/'%candidate_index+enc+'/prose',prose.encode(enc),group='codec-prose',input_codec=enc)
  coverage.append({'candidate_index':candidate_index,'candidate':enc,'actual_codec':codec,'cases':2,'nonascii':any(ord(c)>127 for c in text)})
 # Tiny samples and explicit malformed byte sequences are detector inputs, not guessed intended encodings.
 tiny=[b'',b'a',b'\x00',b'\xff',b'\xfe',b'\x80',b'\x81',b'\xff\xfe',b'\xfe\xff',b'\xef\xbb\xbf',b'\xef\xbb',b'\x00\x00\xfe\xff',b'\xff\xfe\x00',b'\xff\xfe\x00\x00',b'\xc0\xaf',b'\xed\xa0\x80',b'\xf4\x90\x80\x80',b'a\r\nb\rc\n',b'\x1b$B',b'\x1b$B$\x1b(B',b'~{',b'+/v8-',bytes(range(32)),bytes(range(128,160))]
 for i,raw in enumerate(tiny):yield emit('tiny/%02d'%i,raw,group='tiny-malformed')
 multibyte=['utf_8','utf_8_sig','utf_16','utf_16_le','utf_16_be','utf_32','utf_32_le','utf_32_be','utf_7','gb18030','gbk','gb2312','big5','big5hkscs','cp932','shift_jis','shift_jis_2004','euc_jp','euc_jis_2004','euc_kr','cp949','johab','hz','iso2022_jp','iso2022_kr']
 for enc in multibyte:
  text=next((t for t in TEXTS[1:]+TEXTS[:1] if _encodable(t,enc)),TEXTS[0])
  raw=('root\n|--'+text*3+'\n').encode(enc)
  yield emit('truncated/'+enc,raw[:-1],group='truncated',input_codec=enc)
  yield emit('corrupt/'+enc,raw[:len(raw)//2]+b'\xff\x00\x81'+raw[len(raw)//2:],group='corrupt',input_codec=enc)
 rng=random.Random(351115)
 for i in range(12):
  n=[31,32,33,511,512,513,2559,2560,2561,4097,8193,16000][i]
  raw=bytes(rng.randrange(256) for _ in range(n))
  yield emit('binary/%02d'%i,raw,group='binary')
 for i,raw in enumerate([bytes(range(256)),b'\x00'*2048,(b'root\x00\x01\x02\x1b\n')*300,bytes(range(32))*100,(b'a\x00')*1500,(b'\x00a')*1500]):
  yield emit('controls/%02d'%i,raw,group='controls')
 for enc in ['cp1251','cp1252','gb18030','big5','cp932','utf_8']:
  text=next((t for t in TEXTS[1:]+TEXTS[:1] if _encodable(t,enc)),TEXTS[0])
  for kind,prefix in [('coding','# coding: '+enc+'\n'),('html','<meta charset="'+enc+'">\n')]:
   yield emit('declaration/'+enc+'/'+kind,(prefix+text*12).encode(enc),group='declaration',input_codec=enc)
 for enc in ['utf_8','gb18030','cp932','utf_16_le']:
  text=next(t for t in TEXTS[1:] if _encodable(t,enc))
  chunk=text.encode(enc)
  for n in [511,512,513,2559,2560,2561,8193]:
   raw=(b'root\n|--'+chunk*((n//len(chunk))+2))[:n]
   yield emit('boundary/'+enc+'/'+str(n),raw,group='chunk-boundary',input_codec=enc)
 # Native/pure branch at TOO_BIG_SEQUENCE=10,000,000; repetitive payload compresses well.
 for n in [9_999_999,10_000_000,10_000_001]:
  unit=b'root\r\n|--movie_01.mkv\r\n|--movie_01.nfo\n'
  raw=(unit*((n//len(unit))+1))[:n]
  yield emit('large/'+str(n),raw,group='lazy-boundary')
 for enc in ['utf_8','gb18030']:
  raw=('root\n|--'+TEXTS[1]*12000).encode(enc)
  yield emit('alignment-large/'+enc,raw,group='large-multibyte',input_codec=enc)
 cases.coverage=coverage

def _encodable(s,enc):
 try:s.encode(enc);return True
 except (UnicodeError,LookupError):return False

def oracle(raw,detect):
 result={'encoding':None,'language':'','confidence':None,'decoded_text':None,'exception':None}
 try:
  r=detect(raw);result.update({k:r[k] for k in ('encoding','language','confidence')})
 except Exception as e:
  result['exception']={'phase':'detect','type':type(e).__name__,'message':str(e)};return result
 try:
  with io.TextIOWrapper(io.BytesIO(raw),encoding=r['encoding'],errors='ignore',newline=None) as stream:
   result['decoded_text']=stream.read()
 except Exception as e:result['exception']={'phase':'text-read','type':type(e).__name__,'message':str(e)}
 return result

def main():
 ap=argparse.ArgumentParser();ap.add_argument('--module-root',required=True);ap.add_argument('--output',required=True);ap.add_argument('--summary',required=True);args=ap.parse_args()
 sys.path.insert(0,args.module_root)
 import charset_normalizer as cn
 from charset_normalizer import md,cd,constant
 assert cn.__version__=='3.5.1'
 metadata={'schema_version':1,'algorithm':'charset_normalizer.detect','version':cn.__version__,'python':platform.python_version(),'unicode_version':unicodedata.unidata_version,'locale_encoding':locale.getencoding(),'decode':'TextIOWrapper(BytesIO(raw), encoding=detect.encoding, errors=ignore, newline=None).read()', 'source_root':str(SOURCE),'md_module':md.__file__,'cd_module':cd.__file__,'candidate_order':constant.IANA_SUPPORTED,'source_sha256':{str(p.relative_to(SOURCE)):hashlib.sha256(p.read_bytes()).hexdigest() for p in (SOURCE/'src/charset_normalizer').glob('*.py')},'bundled_sha256':{p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in VENDOR.glob('*.pyc')}}
 out=pathlib.Path(args.output);out.parent.mkdir(parents=True,exist_ok=True)
 summaries=[];groups={};exception_cases=[]
 with gzip.open(out,'wt',encoding='utf-8',compresslevel=9) as f:
  f.write('{"metadata":');json.dump(metadata,f,ensure_ascii=False);f.write(',"cases":[')
  for i,item in enumerate(cases(constant.IANA_SUPPORTED)):
   raw=item.pop('raw');result=oracle(raw,cn.detect);item.update(result);item['raw_base64']=base64.b64encode(raw).decode('ascii');item['raw_sha256']=hashlib.sha256(raw).hexdigest()
   digest=hashlib.sha256(json.dumps(result,ensure_ascii=False,sort_keys=True,separators=(',',':')).encode()).hexdigest()
   summaries.append({'id':item['id'],'result_sha256':digest,'encoding':result['encoding'],'language':result['language'],'confidence':result['confidence'],'raw_size':len(raw),'raw_sha256':item['raw_sha256']})
   groups[item['group']]=groups.get(item['group'],0)+1
   if result['exception']:exception_cases.append({'id':item['id'],'exception':result['exception']})
   if i:f.write(',')
   json.dump(item,f,ensure_ascii=False,separators=(',',':'))
   if (i+1)%50==0:print('completed',i+1,flush=True)
  f.write(']}')
 coverage=cases.coverage
 summary={'metadata':metadata,'count':len(summaries),'groups':groups,'codec_coverage':coverage,'unique_actual_codecs':len(set(x['actual_codec'] for x in coverage)),'exceptions':exception_cases,'cases':summaries}
 pathlib.Path(args.summary).write_text(json.dumps(summary,ensure_ascii=False,indent=2)+'\n')
 print('complete',len(summaries),out,groups,flush=True)
if __name__=='__main__':main()
