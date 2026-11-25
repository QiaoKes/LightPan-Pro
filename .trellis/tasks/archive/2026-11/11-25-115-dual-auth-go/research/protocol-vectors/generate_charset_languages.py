import sys,json
from pathlib import Path
sys.path.insert(0,'/Users/saki/Desktop/code/ebn/115-reference/charset_normalizer/src')
from charset_normalizer.constant import FREQUENCIES
from charset_normalizer.cd import encoding_languages
from charset_normalizer.constant import IANA_SUPPORTED
out=['// Code generated from charset-normalizer 3.5.1; DO NOT EDIT.','package charset','type languageProfile struct { name, chars string }','var languageProfiles = []languageProfile{']
for n,c in FREQUENCIES.items():out.append('{'+json.dumps(n,ensure_ascii=False)+','+json.dumps(''.join(c),ensure_ascii=False)+'},')
out+=['}','var encodingLanguageProfiles = map[string][]string{']
for n in dict.fromkeys(IANA_SUPPORTED):
 try: v=encoding_languages(n)
 except (OSError,ValueError):continue
 out.append(json.dumps(n)+':{'+','.join(json.dumps(x,ensure_ascii=False) for x in v)+'},')
out+=['}']
Path('drivers/115/client/charset/languages_generated.go').write_text('\n'.join(out)+'\n')
from charset_normalizer.cd import coherence_ratio, merge_coherence_ratios
cases=[]
texts=['This is a long English sentence about the language and the frequency of letters. '*8,'Bonjour ceci est une phrase française avec des caractères accentués et une langue très intéressante. '*6,'这是一个用于测试中文语言识别和字符频率排序的文本。'*10,'日本語の文字と文章を使って言語判定の動作を確認します。'*10,'abcdefghijklmnopqrstuvwxyz'*4,'a'*32,'a'*33,'Привет это русский текст для проверки определения языка и частоты символов. '*8,'Ελληνικά κείμενο για δοκιμή αναγνώρισης γλώσσας. '*8]
for text in texts:
 for inc in [[],['Latin Based'],['English','French'],['English—','English']]:
  cases.append(dict(Text=text,Inclusion=inc,Expected=[dict(Language=l,Ratio=r)for l,r in coherence_ratio(text,.1,','.join(inc) if inc else None)]))
Path('drivers/115/client/charset/coherence_vectors_test.json').write_text(json.dumps(cases,ensure_ascii=False))
