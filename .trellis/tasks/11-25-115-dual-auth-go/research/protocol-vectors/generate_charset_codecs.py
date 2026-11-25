import codecs, encodings.aliases, json, pathlib
root=pathlib.Path(__file__).resolve().parents[5]
framework=json.loads((pathlib.Path(__file__).parent/'charset-framework.json').read_text())
names=framework['IANA_SUPPORTED']; aliases={}
for n in list(encodings.aliases.aliases)+names+list(encodings.aliases.aliases.values()):
 try: aliases[n]=codecs.lookup(n).name
 except LookupError: pass
single={}
for n in names:
 c=codecs.lookup(n).name
 if c.startswith(('utf','iso2022')) or n in ('big5','big5hkscs','cp932','cp949','cp950','euc_jis_2004','euc_jisx0213','euc_jp','euc_kr','gb18030','gb2312','gbk','hz','johab','shift_jis','shift_jis_2004','shift_jisx0213'):continue
 table=[]
 for i in range(256):
  try: s=bytes([i]).decode(c); table.append(ord(s) if len(s)==1 else -1)
  except UnicodeError:table.append(-1)
 single[c]=table
out=root/'drivers/115/client/charset/codecs/aliases_generated.go'
with out.open('w') as f:
 f.write('// Code generated from CPython 3.12.13; DO NOT EDIT.\npackage codecs\nvar aliases = map[string]string{\n')
 for k,v in sorted(aliases.items()):f.write(json.dumps(k)+':'+json.dumps(v)+',\n')
 f.write('}\nvar supported = []string{'+','.join(map(json.dumps,names))+'}\nvar singleByte = map[string][]rune{\n')
 for k,v in single.items(): f.write(json.dumps(k)+':{'+','.join(map(str,v))+'},\n')
 f.write('}\nvar singleEncode = map[string]map[rune]byte{\n')
 for k,v in single.items():
  pairs=[]
  for r in sorted(set(v)-{-1}):
   try: b=chr(r).encode(k); pairs.append(str(r)+':'+str(b[0]))
   except UnicodeError: pass
  f.write(json.dumps(k)+':{'+','.join(pairs)+'},\n')
 f.write('}\n')
print(root, list(single))
