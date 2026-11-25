package cipher115

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestRSAReferenceVector(t *testing.T) {
	if got := RSAEncrypt([]byte(`{"pickcode":"abc","user_id":42}`)); got != "ELE5bix6ATPd3RSco79mVv+13oeyB2+jC64FhbHaTmBAaBnITdaK+yc6I8J4KqnrerbpySEDmrH9smQzRZzBu9vDpXKVF4m4m2t2Ws8KODxXtZVinBbtro+iAg4SO+qJ3Rgm3gDPi3gGdyHiU0wdCgHNMbmN7P8dMxeScmdeaxo=" {
		t.Fatal(got)
	}
}
func TestAESLZ4LiteralFrame(t *testing.T) {
	frame := []byte{6, 0, 0x50, 104, 101, 108, 108, 111}
	got, err := AESDecrypt(AESEncrypt(frame))
	if err != nil || string(got) != "hello" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestBundledKnownAnswers(t *testing.T) {
	raw, err := os.ReadFile("test-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		RSAEncrypt []struct {
			Plain  string `json:"plaintext_hex"`
			Cipher string `json:"ciphertext_base64"`
		} `json:"rsa_encrypt"`
		RSADecrypt []struct {
			Plain  string `json:"plaintext_hex"`
			Cipher string `json:"ciphertext_base64"`
		} `json:"rsa_decrypt"`
		AESEncrypt []struct {
			Plain  string `json:"plaintext_hex"`
			Cipher string `json:"ciphertext_hex"`
		} `json:"aes_encrypt"`
		Response struct {
			Plain  string `json:"decoded_json_utf8"`
			Cipher string `json:"ciphertext_hex"`
		} `json:"aes_response"`
	}
	if err = json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	for _, x := range v.RSAEncrypt {
		b, _ := hex.DecodeString(x.Plain)
		if RSAEncrypt(b) != x.Cipher {
			t.Fatal("bundled RSA encrypt mismatch")
		}
	}
	for _, x := range v.RSADecrypt {
		b, err := RSADecrypt(x.Cipher)
		if err != nil || hex.EncodeToString(b) != x.Plain {
			t.Fatalf("bundled RSA decrypt mismatch %x %v", b, err)
		}
	}
	for _, x := range v.AESEncrypt {
		b, _ := hex.DecodeString(x.Plain)
		if hex.EncodeToString(AESEncrypt(b)) != x.Cipher {
			t.Fatal("bundled AES encrypt mismatch")
		}
	}
	b, _ := hex.DecodeString(v.Response.Cipher)
	out, err := AESDecrypt(b)
	if err != nil || string(out) != v.Response.Plain {
		t.Fatalf("bundled AES response %q %v", out, err)
	}
	if Token() != "HQMOgKF43O7OzaN33hKNAHMAAAAAAAAAjtndz1WuYe1G6hIaHPyBAAEAAAAMfvVI" {
		t.Fatal("bundled token mismatch")
	}
}
