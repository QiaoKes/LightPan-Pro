"""Offline only: run with pinned CPython3.12.13; imports pinned3.5.1 source."""
import sys,json,base64, pathlib
sys.path.insert(0,'/Users/saki/Desktop/code/ebn/115-reference/charset_normalizer/src')
from charset_normalizer.constant import _IANA_NAMES,IANA_SUPPORTED,ENCODING_MARKS
from charset_normalizer.utils import any_specified_encoding
from charset_normalizer.legacy import detect
root=pathlib.Path('drivers/115/client/charset')
p=root/'detector_constants.go'
s=p.read_text(); s=s.split('// Python 3.12 exact declaration alias inventory.')[0]
s+='// Python 3.12 exact declaration alias inventory.\nvar declarationNames = map[string]string{\n'
for k,v in sorted(_IANA_NAMES.items()):s+=json.dumps(k)+':'+json.dumps(v)+',\n'
s+='}\n';p.write_text(s)
cases=[]
for prefix in [b'',b'\xff',b'co\xffding: ',b'\xff'*100+b' ']:
 for name in ['utf8','utf-8','utf_8','utf16','utf-16','utf_16','utf_8_sig','utf-8-sig','utf__8','UTF--8','unicode_escape','base64_codec','rot_13','undefined','gb18030']:
  raw=prefix+b'coding: '+name.encode()+b'\nroot\n| |-movie.mkv\r\n'
  try: result=detect(raw);exc=None
  except Exception as e:result=None;exc=type(e).__name__
  cases.append(dict(raw_base64=base64.b64encode(raw).decode(),specified=any_specified_encoding(raw),detection=result,exception=exc))
# ASCII-ignore could create a marker, but reference cheap byte prefilter rejects it.
raw=b'co\xffding: utf8\nhello';cases.append(dict(raw_base64=base64.b64encode(raw).decode(),specified=any_specified_encoding(raw),detection=detect(raw),exception=None))
(root/'testdata/declarations.json').write_text(json.dumps(cases))
print('distinct',len(set(IANA_SUPPORTED)),'signature outside',set(ENCODING_MARKS)-set(IANA_SUPPORTED))
