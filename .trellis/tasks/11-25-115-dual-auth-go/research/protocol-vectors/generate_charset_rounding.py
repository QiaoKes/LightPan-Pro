"""Independent CPython binary64 rounding oracle; run offline with pinned 3.12."""
import math,struct,json
from pathlib import Path
bits=lambda x: struct.pack('>d',x).hex()
rows=[]
for digits in (3,4):
 values=[0.,-0.,float('inf'),-float('inf'),float('nan'),1.9125,2.675]
 for i in range(-200,201):
  tie=(i+.5)/(10**digits)
  values.extend((tie,math.nextafter(tie,-math.inf),math.nextafter(tie,math.inf)))
 for denominator in range(1,101):
  for numerator in range(denominator+1):
   value=numerator/denominator
   values.extend((value,(value+0.12345)/2))
 seen=set()
 for value in values:
  key=bits(value)
  if key in seen:continue
  seen.add(key)
  rows.append(dict(Input=key,Digits=digits,Expected=bits(round(value,digits))))
Path('drivers/115/client/charset/rounding_vectors_test.json').write_text(json.dumps(rows,separators=(',',':'))+'\n')
print(len(rows))
