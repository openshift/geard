package api

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"github.com/smarterclayton/geard/jobs"	
)

type TokenData struct {
	I string // 32 character hexadecimal string, request identifier
	D int    // expiration time in seconds from the epoch
	U string // user unique identifier in hexadecimal

	T string // resource type
	R string // resource locator
}

func (t *TokenData) RequestId() (jobs.RequestIdentifier, error) {
	b, err := hex.DecodeString(t.I)
	if err != nil {
		return nil, err
	}
	if len(b) != 16 {
		return nil, errors.New("Request ID must be exactly 16 bytes (32 hexadecimal characters) in length.")
	}
	return jobs.RequestIdentifier(b), nil
}

func (t *TokenData) ResourceType() string {
	return t.T
}
func (t *TokenData) ResourceLocator() string {
	return t.R
}

func NewTokenFromString(s string) (*TokenData, error) {
	dec := json.NewDecoder(base64.NewDecoder(base64.URLEncoding, strings.NewReader(s)))
	token := TokenData{}
	if err := dec.Decode(&token); err != nil {
		return nil, err
	}
	return &token, nil
}

func NewTokenFromMap(m map[string][]string) (*TokenData, error) {
	token := TokenData{}
	token.I = firstParam(m, "i")
	if len(token.I) > 0 && len(token.I) < 32 {
		token.I = strings.Repeat("0", 32-len(token.I)) + token.I
	}
	date := firstParam(m, "d")
	if date != "" {
		d, err := strconv.Atoi(date)
		if err != nil {
			return nil, err
		}
		token.D = d
	}
	token.U = firstParam(m, "u")
	token.T = firstParam(m, "t")
	token.R = firstParam(m, "r")
	return &token, nil
}

func firstParam(m map[string][]string, k string) string {
	if a, found := m[k]; found {
		if len(a) > 0 {
			return a[0]
		}
	}
	return ""
}
