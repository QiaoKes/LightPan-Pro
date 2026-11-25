"""Execute only offline cipher bytecode with audited local byte-buffer stub."""
import pathlib,marshal,types,sys,json,base64
B=pathlib.Path(__file__).resolve().parent;V=pathlib.Path('/Users/saki/Desktop/code/ebn/restored-python/vendor')
def mod(name,rel):
 m=types.ModuleType(name);m.__package__=name.rsplit('.',1)[0];sys.modules[name]=m
 exec(marshal.loads((V/rel).read_bytes()[16:]),m.__dict__);return m
# filewrap conversion needed here only for bytes/memoryview; no file or HTTP paths.
f=types.ModuleType('filewrap');f.to_bytes_view=lambda x:memoryview(x);sys.modules['filewrap']=f
sys.path.insert(0,str(V))
pkg=types.ModuleType('p115cipher');pkg.__path__=[str(V/'p115cipher')];pkg.__package__='p115cipher';sys.modules['p115cipher']=pkg
sys.modules['Crypto']=None;sys.modules['cryptography']=None;sys.modules['lz4']=None
u=mod('p115cipher.util','p115cipher/util.pyc')
a=mod('p115cipher.aes','p115cipher/aes.pyc')
# Select exact bundled pure AES fallback, avoiding native Linux extension imports.
def enc(data,key,iv):
 encrypt=a.AES(key).encrypt;out=bytearray();last=iv
 for i in range(0,len(u._pad(data)),16):
  last=encrypt([(p^l) for p,l in zip(u._pad(data)[i:i+16],last)]);out.extend(last)
 return bytes(out)
def dec(data,key,iv):
 decrypt=a.AES(key).decrypt;out=bytearray();last=iv
 for i in range(0,len(data),16):
  out.extend(p^l for p,l in zip(decrypt(data[i:i+16]),last));last=data[i:i+16]
 return u._unpad(out)
u.aes_cbc_encrypt=enc;u.aes_cbc_decrypt=dec
exec(marshal.loads((V/'p115cipher/__init__.pyc').read_bytes()[16:]),pkg.__dict__)
vec={'provenance':'actual bundled CPython3.12 cipher bytecode; pure bundled AES fallback; synthetic inputs only','rsa_encrypt':[],'aes_encrypt':[],'ecdh_token':[]}
for s in [b'{"pickcode":"abc","user_id":42}',b'',b'a'*118]:
 vec['rsa_encrypt'].append({'plaintext_hex':s.hex(),'ciphertext_base64':pkg.rsa_encrypt(s).decode()})
for s in [b'hello&',b'a'*16,b'']:
 c=pkg.ecdh_aes_encrypt(s);vec['aes_encrypt'].append({'plaintext_hex':s.hex(),'ciphertext_hex':bytes(c).hex(),'raw_aes_decrypted_hex':bytes(dec(c,pkg.AES_KEY,pkg.AES_IV)).hex() if c else ''})
for t in [0,1700000000]:vec['ecdh_token'].append({'timestamp':t,'token':pkg.ecdh_encode_token(t).decode()})
vec['constants']={k:(getattr(pkg,k).hex() if isinstance(getattr(pkg,k),bytes) else str(getattr(pkg,k))) for k in ['AES_KEY','AES_IV','AES_PUBKEY','RSA_KEY','RSA_RAND_KEY','G_key_l','CRC_SALT']}
vec['rsa_pubkey_n_hex']=hex(u.RSA_PUBKEY_PAIR[0]);vec['rsa_pubkey_e']=u.RSA_PUBKEY_PAIR[1]
# Synthetic response vectors; no captured account/server data.
vec['rsa_decrypt']=[]
for counter in range(2,2048):
 c=counter.to_bytes(128,'big')
 try:
  plain=pkg.rsa_decrypt(base64.b64encode(c))
 except (ValueError,IndexError):continue
 if plain:
  vec['rsa_decrypt'].append({'ciphertext_base64':base64.b64encode(c).decode(),'plaintext_hex':plain.hex(),'kind':'synthetic arbitrary cipher block, not a server signature/JSON'})
  break
response=b'{"state":true,"data":{"file_id":"42"}}'
# Literal-only raw LZ4 block, preceded by SDK little-endian block byte length.
raw=bytes([0xf0,len(response)-15])+response
framed=len(raw).to_bytes(2,'little')+raw+b'\x00\x00'
c=bytes(pkg.ecdh_aes_encrypt(framed))
assert pkg.ecdh_aes_decrypt(c)==response
vec['aes_response']={'decoded_json_utf8':response.decode(),'lz4_framed_hex':framed.hex(),'ciphertext_hex':c.hex()}
from urllib.parse import urlencode
req=b'{"pickcode":"abc","user_id":42}'
form=(urlencode({'data':pkg.rsa_encrypt(req).decode()})+'&').encode()
vec['cookie_download_request']={'url':'https://proapi.115.com/os_windows/2.0/ufile/downurl','k_ec':pkg.ecdh_encode_token(0).decode(),'ua':'','rsa_json_utf8':req.decode(),'form_before_aes_utf8':form.decode(),'wire_body_hex':bytes(pkg.ecdh_aes_encrypt(form)).hex()}
vec['bundled_metadata']={'version':list(pkg.__version__),'license':pkg.__license__}
(B/'cipher-known-answers.json').write_text(json.dumps(vec,indent=2)+'\n');print(json.dumps(vec,indent=2))
