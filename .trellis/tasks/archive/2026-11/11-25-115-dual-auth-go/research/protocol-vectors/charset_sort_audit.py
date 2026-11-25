"""Offline candidate sorting audit against exact CPython 3.12 list.sort."""
import sys,json,gzip,base64,pathlib
P=pathlib.Path(__file__).resolve().parent
sys.path.insert(0,'/Users/saki/Desktop/code/ebn/115-reference/charset_normalizer/src')
import charset_normalizer as cn
from charset_normalizer.models import CharsetMatches
original=CharsetMatches._ensure_sorted
observations=[];current=''
def go_short(items):
 a=list(items);n=len(a)
 if n<2:return a
 run=2
 if a[1]<a[0]:
  while run<n and a[run]<a[run-1]:run+=1
  a[:run]=reversed(a[:run])
 else:
  while run<n and not a[run]<a[run-1]:run+=1
 for start in range(run,n):
  pivot=a[start];lo=0;hi=start
  while lo<hi:
   mid=(lo+hi)//2
   if pivot<a[mid]:hi=mid
   else:lo=mid+1
  a[lo+1:start+1]=a[lo:start];a[lo]=pivot
 return a
def audited(self):
 if not self._is_sorted:
  before=list(self._results);actual=sorted(before);emulated=go_short(before)
  observations.append({'id':current,'count':len(before),'input':[a.encoding for a in before],'python':[a.encoding for a in actual],'go_short':[a.encoding for a in emulated],'equal':all(a is b for a,b in zip(actual,emulated))})
 return original(self)
CharsetMatches._ensure_sorted=audited
fixture_path=sys.argv[1] if len(sys.argv)>1 else 'drivers/115/client/charset/testdata/detection-oracle.json.gz'
fixture=json.load(gzip.open(fixture_path,'rt'))
for case in fixture['cases']:
 current=case['id'];cn.detect(base64.b64decode(case['raw_base64']))
report={'case_count':len(fixture['cases']),'sort_count':len(observations),'max_candidates':max(o['count'] for o in observations),'differences':[o for o in observations if not o['equal']],'observations':observations}
(P/('charset-stress-sort-audit.json' if len(sys.argv)>1 else 'charset-sort-audit.json')).write_text(json.dumps(report,indent=2)+'\n')
print({k:v for k,v in report.items() if k!='observations'})
