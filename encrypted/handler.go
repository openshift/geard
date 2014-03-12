package encrypted

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	jobhttp "github.com/smarterclayton/geard/http"
	"github.com/smarterclayton/geard/jobs"
	"github.com/smarterclayton/geard/utils"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"
)

// Limit of how far in the future a token may expire - 1 day by default
const MaxTokenFutureSeconds = 1 * 60 * 60 * 24

type TokenConfiguration struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

func NewTokenConfiguration(private, public string) (*TokenConfiguration, error) {
	priv, err := loadPrivateKey(private)
	if err != nil {
		return nil, err
	}
	pub, err := loadPublicKey(public)
	if err != nil {
		return nil, err
	}
	return &TokenConfiguration{priv, pub}, nil
}

func (t *TokenConfiguration) Sign(job *jobs.ContentRequest, keyId string, expiration int64) (string, error) {
	source := &TokenData{
		Identifier:     jobs.NewRequestIdentifier().String(),
		Locator:        job.Locator,
		Type:           job.Type,
		ExpirationDate: expiration,
	}

	buf := &bytes.Buffer{}
	encoder := json.NewEncoder(buf)
	if err := encoder.Encode(source); err != nil {
		return "", err
	}

	cipher, err := rsa.EncryptPKCS1v15(rand.Reader, t.publicKey, buf.Bytes())
	if err != nil {
		return "", err
	}

	hash := crypto.SHA256.New()
	if _, err := hash.Write(cipher); err != nil {
		return "", err
	}

	hashed := hash.Sum(nil)
	sig, err := rsa.SignPKCS1v15(rand.Reader, t.privateKey, crypto.SHA256, hashed)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"%s/%s/%s",
		utils.EncodeUrlPath(keyId),
		base64.URLEncoding.EncodeToString(sig),
		base64.URLEncoding.EncodeToString(cipher),
	), nil
}

func (t *TokenConfiguration) Handler(parent http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items := strings.SplitN(r.URL.Path, "/", 4)
		if len(items) != 4 {
			http.Error(w, "Expecting path of /:key/:signed/:ciphertext", http.StatusBadRequest)
			return
		}

		cipher, err := base64.URLEncoding.DecodeString(items[3])
		if err != nil {
			http.Error(w, "Token must be base64 URL encoded", http.StatusBadRequest)
			return
		}
		sig, err := base64.URLEncoding.DecodeString(items[2])
		if err != nil {
			http.Error(w, "Signature must be base64 URL encoded", http.StatusBadRequest)
			return
		}

		hash := crypto.SHA256.New()
		hash.Write(cipher)
		sighash := hash.Sum(nil)

		if err := rsa.VerifyPKCS1v15(t.publicKey, crypto.SHA256, sighash, sig); err != nil {
			http.Error(w, "Signature is not valid", http.StatusBadRequest)
			return
		}

		out, err := rsa.DecryptPKCS1v15(rand.Reader, t.privateKey, cipher)
		if err != nil {
			http.Error(w, "Token is not valid", http.StatusBadRequest)
			return
		}

		token := &TokenData{}
		decoder := json.NewDecoder(bytes.NewReader(out))
		decoder.Decode(token)
		log.Printf("Decoded %+v", *token)

		if token.Locator == "" || token.Type == "" {
			log.Printf("The token has no locator or type")
			http.Error(w, "Token is not valid", http.StatusBadRequest)
			return
		}
		now := time.Now().Unix()
		delta := token.ExpirationDate - now
		if delta < 0 {
			log.Printf("The token expired %i seconds ago", delta)
			http.Error(w, "Token is not valid", http.StatusBadRequest)
			return
		}
		if delta > MaxTokenFutureSeconds {
			log.Printf("The token is too far in the future %i", delta)
			http.Error(w, "Token is not valid", http.StatusBadRequest)
			return
		}

		job := jobhttp.HttpContentRequest{ContentRequest: jobs.ContentRequest{Type: token.Type, Locator: token.Locator}}
		r.Method = job.HttpMethod()
		r.URL.Path = job.HttpPath()
		parent.ServeHTTP(w, r)
	}
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	// Read the private key
	pemData, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("read key file: %s", err))
	}

	// Extract the PEM-encoded data block
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New(fmt.Sprintf("bad key data: %s", "not PEM-encoded"))
	}
	if got, want := block.Type, "RSA PRIVATE KEY"; got != want {
		return nil, errors.New(fmt.Sprintf("unknown key type %q, want %q", got, want))
	}

	// Decode the RSA private key
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("bad private key: %s", err))
	}

	return priv, nil
}

func loadPublicKey(path string) (*rsa.PublicKey, error) {
	// Read the private key
	pemData, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("read key file: %s", err))
	}

	// Extract the PEM-encoded data block
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New(fmt.Sprintf("bad key data: %s", "not PEM-encoded"))
	}
	if got, want := block.Type, "PUBLIC KEY"; got != want {
		return nil, errors.New(fmt.Sprintf("unknown key type %q, want %q", got, want))
	}

	// Decode the RSA private key
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("bad public key: %s", err))
	}

	key, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New(fmt.Sprintf("public key does not implement *rsa.PublicKey: %s", reflect.TypeOf(pub)))
	}

	return key, nil
}
