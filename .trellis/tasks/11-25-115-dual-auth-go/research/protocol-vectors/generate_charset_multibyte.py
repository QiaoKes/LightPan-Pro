"""Offline Python 3.12 incremental codec transducer generator."""
import codecs, collections, gzip, json, pathlib, random, struct
NAMES='big5 big5hkscs cp932 cp949 cp950 euc_jis_2004 euc_jisx0213 euc_jp euc_kr gb18030 gb2312 gbk hz iso2022_jp iso2022_jp_1 iso2022_jp_2 iso2022_jp_2004 iso2022_jp_3 iso2022_jp_ext iso2022_kr johab shift_jis shift_jis_2004 shift_jisx0213'.split()
ROOT=pathlib.Path('drivers/115/client/charset/codecs')
def generate(name):
 d=codecs.getincrementaldecoder(name)('ignore'); states=[d.getstate()]; ids={states[0]:0}; rows=[]; finals=[]
 for state in ([] if name.startswith("iso2022") else states):
  row=[]
  for b in range(256):
   d.setstate(state)
   if (name=='gb18030' and len(state[0])==1 and state[0][0]>=0x80 and 0x30<=b<=0x39) or (name=='euc_kr' and state[0]==b'\xa4' and b==0xd4):
    row.append([0,'',False]); continue
   out=d.decode(bytes([b]),False); ns=d.getstate()
   if ns not in ids:
    if len(states)>5000: raise RuntimeError((name,len(states),states[-10:]))
    ids[ns]=len(states); states.append(ns)
   # strict validity is independently observable, including buffered prefixes
   strict=codecs.getincrementaldecoder(name)('strict'); strict.setstate(state)
   try: strict.decode(bytes([b]),False); valid=True
   except UnicodeError: valid=False
   row.append([ids[ns],out,valid])
  rows.append(row); d.setstate(state)
  strict=codecs.getincrementaldecoder(name)('strict'); strict.setstate(state)
  try: strict.decode(b'',True); finals.append(True)
  except UnicodeError: finals.append(False)
 print(name,'decoder',len(states),flush=True)
 e=codecs.getincrementalencoder(name)('strict'); es=[e.getstate()]; ei={es[0]:0}; er=[]; ef=[]; ignored=[]
 for state in es:
  row={}
  for cp in range(0x110000):
   if 0xd800<=cp<=0xdfff: continue
   e.setstate(state)
   try: out=e.encode(chr(cp),False)
   except UnicodeError: continue
   if name=='gb18030' and len(out)==4: continue
   ns=e.getstate()
   if ns not in ei: ei[ns]=len(es); es.append(ns)
   row[str(cp)]=[ei[ns],out.hex()]
  er.append(row); e.setstate(state); ef.append(e.encode('',True).hex())
  ie=codecs.getincrementalencoder(name)('ignore');ie.setstate(state);out=ie.encode(chr(0x10ffff),False);ns=ie.getstate()
  if ns not in ei: ei[ns]=len(es);es.append(ns)
  ignored.append([ei[ns],out.hex()])
 enc=bytearray(struct.pack('<I',len(er)))
 for row,final,ig in zip(er,ef,ignored):
  enc.extend(struct.pack('<I',len(row)))
  for cp,(nxt,h) in sorted(row.items(),key=lambda x:int(x[0])):
   v=bytes.fromhex(h);enc.extend(struct.pack('<IIH',int(cp),nxt,len(v)));enc.extend(v)
  for nxt,h in [(0,final),ig]:
   v=bytes.fromhex(h);enc.extend(struct.pack('<IH',nxt,len(v)));enc.extend(v)
 (ROOT/('multibyte_'+name+'.bin.gz')).write_bytes(gzip.compress(enc,mtime=0))
 data={'p':[s[0].hex() for s in states],'d':rows,'f':finals}
 if name=='gb18030':
  ranges=[]
  for ix in range(126*10*126*10):
   v=ix; b4=v%10+48; v//=10; b3=v%126+129; v//=126; b2=v%10+48; b1=v//10+129
   try: cp=ord(bytes([b1,b2,b3,b4]).decode(name))
   except UnicodeError: continue
   if ranges and ix==ranges[-1][1]+1 and cp==ranges[-1][2]+ix-ranges[-1][0]: ranges[-1][1]=ix
   else: ranges.append([ix,ix,cp])
  data['g']=ranges
 if name=='euc_kr':
  special={}
  for a in range(0xa1,0xff):
   for b in range(0xa1,0xff):
    for c in range(0xa1,0xff):
     seq=bytes([0xa4,0xd4,0xa4,a,0xa4,b,0xa4,c])
     try: out=seq.decode(name)
     except UnicodeError: continue
     special[str((a<<16)|(b<<8)|c)]=out
  data['k']=special
 (ROOT/('multibyte_'+name+'.json.gz')).write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
 print(name,'done',flush=True)
def vectors():
 r=random.Random(32012);out=[]
 for n in NAMES:
  samples=[bytes([b]) for b in range(256)]+[bytes(r.randrange(256) for _ in range(r.randrange(2,30))) for i in range(250)]
  samples += [s.encode(n,'ignore') for s in ['中文日本語한국어','αβγ café €😀','か゚カ゚æ̀','가각힣','ASCII~\\']]
  for b in samples:
   try: b.decode(n);valid=True
   except UnicodeError:valid=False
   out.append([n,b.hex(),b.decode(n,'ignore'),valid])
 pathlib.Path('drivers/115/client/charset/codecs/multibyte_vectors.json.gz').write_bytes(gzip.compress(json.dumps(out,ensure_ascii=True).encode(),mtime=0))
 out=[]
 for n in NAMES:
  samples=['中文日本語한국어','αβγ café €😀','か゚カ゚æ̀','か😀゚','가각힣','ASCII~\\','か𐀀','æ😀̀']
  samples += [''.join(chr(r.randrange(0xd800)) for _ in range(12)) for i in range(50)]
  for text in samples:
   try: text.encode(n);valid=True
   except UnicodeError:valid=False
   out.append([n,text,text.encode(n,'ignore').hex(),valid])
 pathlib.Path('drivers/115/client/charset/codecs/multibyte_encode_vectors.json.gz').write_bytes(gzip.compress(json.dumps(out,ensure_ascii=True).encode(),mtime=0))

if __name__=='__main__':
 import sys
 if sys.argv[1:]==['--vectors']: vectors()
 else:
  for name in (sys.argv[1:] or NAMES): generate(name)
