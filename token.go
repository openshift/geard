package agent

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

type TokenData struct {
	I string // 32 character hexadecimal string, request identifier
	D int    // expiration time in seconds from the epoch
	U string // user unique identifier in hexadecimal

	T string // resource type
	R string // resource locator
}

func (t *TokenData) RequestId() ([]byte, error) {
	b, err := hex.DecodeString(t.I)
	if err != nil {
		return nil, err
	}
	if len(b) != 16 {
		return nil, errors.New("Request ID must be exactly 16 bytes (32 hexadecimal characters) in length.")
	}
	return b, nil
}

func (t *TokenData) ResourceType() string {
	return t.T
}
func (t *TokenData) ResourceLocator() string {
	return t.R
}

func NewTokenFromPath(path string) (*TokenData, string, error) {
	segment, subpath, has := TakeSegment(path)
	if !has {
		return nil, "", errors.New("No token present in path")
	}

	dec := json.NewDecoder(base64.NewDecoder(base64.URLEncoding, strings.NewReader(segment)))
	token := TokenData{}
	if err := dec.Decode(&token); err != nil {
		return nil, "", err
	}

	return &token, subpath, nil
}
