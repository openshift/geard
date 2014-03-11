package encrypted

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestDecrypt(t *testing.T) {
	serverPriv, err := loadPrivateKey("fixtures/server")
	if err != nil {
		t.Fatal("Unable to load server private RSA key", err)
	}
	serverPub, err := loadPublicKey("fixtures/server.pub")
	if err != nil {
		t.Fatal("Unable to load server public RSA key", err)
	}

	original := []byte("Do some stuff")
	cipher, err := rsa.EncryptPKCS1v15(rand.Reader, serverPub, original)
	if err != nil {
		t.Fatal("Unable to encrypt text", err)
	}

	text, err := rsa.DecryptPKCS1v15(rand.Reader, serverPriv, cipher)
	if err != nil {
		t.Fatal("Unable to decrypt text", err)
	}
	if string(original) != string(text) {
		t.Fatal("Expected original to match text", original, text)
	}
}

func TestSign(t *testing.T) {
	clientPriv, err := loadPrivateKey("fixtures/client")
	if err != nil {
		t.Fatal("Unable to load client private RSA key", err)
	}
	clientPub, err := loadPublicKey("fixtures/client.pub")
	if err != nil {
		t.Fatal("Unable to load client public RSA key", err)
	}

	original := []byte("Do some stuff")
	hash := crypto.SHA256.New()
	hash.Write(original)
	hashed := hash.Sum(nil)

	sig, err := rsa.SignPKCS1v15(rand.Reader, clientPriv, crypto.SHA256, hashed)
	if err != nil {
		t.Fatal("Unable to sign text", err)
	}
	if err := rsa.VerifyPKCS1v15(clientPub, crypto.SHA256, hashed, sig); err != nil {
		t.Fatal("Unable to verify signature", err)
	}
}

type testWriter struct {
	buf     bytes.Buffer
	test    *testing.T
	headers http.Header
	code    int
}

func (t *testWriter) Header() http.Header {
	return t.headers
}
func (t *testWriter) Write(b []byte) (int, error) {
	return fmt.Fprintf(os.Stderr, string(b))
}
func (t *testWriter) WriteHeader(code int) {
	t.code = code
}

func TestHandle(t *testing.T) {
	config, err := NewTokenConfiguration("fixtures/server", "fixtures/client.pub")
	if err != nil {
		t.Fatal("Found an error while creating config", err)
	}

	serverPub, err := loadPublicKey("fixtures/server.pub")
	if err != nil {
		t.Fatal("Unable to load server public RSA key", err)
	}
	clientPriv, err := loadPrivateKey("fixtures/client")
	if err != nil {
		t.Fatal("Unable to load client private RSA key", err)
	}

	buf := &bytes.Buffer{}
	source := &TokenData{Locator: "foo", Type: "env", ExpirationDate: time.Now().Unix() + 10}
	encoder := json.NewEncoder(buf)
	encoder.Encode(source)
	cipher, _ := rsa.EncryptPKCS1v15(rand.Reader, serverPub, buf.Bytes())
	hash := crypto.SHA256.New()
	hash.Write(cipher)
	hashed := hash.Sum(nil)
	sig, _ := rsa.SignPKCS1v15(rand.Reader, clientPriv, crypto.SHA256, hashed)

	path := fmt.Sprintf("/key/%s/%s", base64.URLEncoding.EncodeToString(sig), base64.URLEncoding.EncodeToString(cipher))

	test := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/environment/foo" {
			t.Fatal("Expected to be called with /environment/foo", r.URL.Path)
		}
	}
	handler := config.Handler(http.HandlerFunc(test))
	r, _ := http.NewRequest("GET", path, nil)
	w := &testWriter{test: t, headers: make(http.Header)}
	handler.ServeHTTP(w, r)

	fmt.Printf("Headers: %+v", w.headers)
	if w.code != 0 {
		t.Fatal("Expected code 0", w.code)
	}
}
