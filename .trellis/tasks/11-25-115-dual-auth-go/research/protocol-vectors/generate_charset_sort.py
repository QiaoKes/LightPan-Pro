"""Deterministic full-order oracle; run with pinned CPython 3.12.13."""
import sys,pathlib,random,json,gzip
sys.path.insert(0,'/Users/saki/Desktop/code/ebn/115-reference/charset_normalizer/src')
from charset_normalizer.models import CharsetMatch
assert sys.version_info[:3]==(3,12,13)
r=random.Random(35131213); cases=[]
for t in range(3000):
 n=t%101 if t<1010 else 64+t%37
 raw=10000000 if t%4==0 else 100
 rows=[]
 for i in range(n):
  chaos=r.choice([0,.004999999999,.005,.005000000001,.009999999999,.01,.015,.02,.1])
  coherence=r.choice([0,.019999999999,.02,.020000000001,.04,.06,.5])
  length=r.choice([0,1,30,50,50,99,100])
  if t%10==0:chaos=i*.01
  if t%10==1:chaos=(n-i)*.01
  if t%10==2:chaos=(i%32)*.01
  if t%10==3:chaos=0;coherence=0;length=50
  rows.append([chaos,coherence,length])
 payload=b'x'*raw
 matches=[CharsetMatch(payload,str(i),c,False,[('English',h)],'x'*l) for i,(c,h,l) in enumerate(rows)]
 order=[int(m.encoding) for m in sorted(matches)]
 cases.append(dict(raw=raw,rows=rows,order=order))
p=pathlib.Path('drivers/115/client/charset/model_sort_vectors_test.json.gz')
with gzip.GzipFile(filename=str(p),mode='wb',mtime=0) as f:f.write(json.dumps(cases,separators=(',',':')).encode())
print(len(cases))
