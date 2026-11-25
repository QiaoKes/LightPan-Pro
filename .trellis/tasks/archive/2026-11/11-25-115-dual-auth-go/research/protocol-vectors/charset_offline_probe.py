"""Offline official-tag pure/native comparison; imports no account modules."""
import ast,json,pathlib,sys
sys.path.insert(0,sys.argv[1])
import charset_normalizer as cn
from charset_normalizer import md,cd
src=pathlib.Path('/Users/saki/Desktop/code/ebn/115-reference/charset_normalizer')
strings=[]
for f in ['test_mess_detection.py','test_coherence_detection.py']:
 for n in ast.walk(ast.parse((src/'tests'/f).read_text())):
  if isinstance(n,ast.Constant) and isinstance(n.value,str) and 8<=len(n.value)<=2000 and n.value not in strings:
   strings.append(n.value)
strings=strings[:120]
r={'version':cn.__version__,'md_module':md.__file__,'cd_module':cd.__file__,'scores':[],'detect':[]}
for text in strings:
 r['scores'].append({'text':text,'mess_default':md.mess_ratio(text),'mess_threshold1':md.mess_ratio(text,1.0),'coherence':cd.coherence_ratio(text)})
texts=['root\n|--movie.mkv\n|--movie.nfo','根目录\n|--电影\n| |--第1集.mkv\n| |--第1集.nfo','日本語ディレクトリ\n|--映画.mkv\n|--字幕.srt','Русский каталог\n|--фильм.mkv','العقلية التنويم المغناطيسي']
for text in texts:
 for enc in ['utf_8','utf_8_sig','utf_16','utf_16_le','utf_16_be','utf_32','gb18030','gbk','big5','cp932','euc_jp','cp1251','cp1252','cp1256']:
  try:b=text.encode(enc)
  except UnicodeError:continue
  result=cn.detect(b);codec=result['encoding'];decoded=b.decode(codec or 'utf_8',errors='ignore')
  r['detect'].append({'input_codec':enc,'hex':b.hex(),'result':result,'decoded':decoded})
for b in [b'',b'\xff\xfe\x00',b'abc\xff\x80',bytes(range(256)),b'coding: cp1251\n\xff',b'\x00'*80]:
 result=cn.detect(b);r['detect'].append({'input_codec':'adversarial','hex':b.hex(),'result':result,'decoded':b.decode(result['encoding'] or 'utf_8',errors='ignore')})
pathlib.Path(sys.argv[2]).write_text(json.dumps(r,ensure_ascii=False,indent=2)+'\n')
print(md.__file__,cd.__file__,len(strings),len(r['detect']))
