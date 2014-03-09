package http

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/smarterclayton/geard/jobs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type TokenData struct {
	I string // 32 character hexadecimal string, request identifier
	D int    // expiration time in seconds from the epoch
	U string // user unique identifier in hexadecimal

	T string // resource type
	R string // resource locator
}

func (t *TokenData) RequestId() (jobs.RequestIdentifier, error) {
	return jobs.NewRequestIdentifierFromString(t.I)
}
func (t *TokenData) SetRequestIdentifier(id jobs.RequestIdentifier) {
	t.I = hex.EncodeToString(id)
}

func (t *TokenData) ResourceType() string {
	return t.T
}
func (t *TokenData) ResourceLocator() string {
	return t.R
}
func (t *TokenData) ToValues(values *url.Values) {
	if t.I != "" {
		values.Set("i", t.I)
	}
	if t.T != "" {
		values.Set("t", t.T)
	}
	if t.R != "" {
		values.Set("r", t.R)
	}
	if t.U != "" {
		values.Set("u", t.U)
	}
	if t.D != 0 {
		values.Set("d", strconv.Itoa(t.D))
	}
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

func extractToken(segment string, r *http.Request) (token *TokenData, id jobs.RequestIdentifier, rerr *apiRequestError) {
	if segment == "__test__" {
		t, err := NewTokenFromMap(r.URL.Query())
		if err != nil {
			rerr = &apiRequestError{err, "Invalid test query: " + err.Error(), http.StatusForbidden}
			return
		}
		token = t
	} else {
		t, err := NewTokenFromString(segment)
		if err != nil {
			rerr = &apiRequestError{err, "Invalid authorization token", http.StatusForbidden}
			return
		}
		token = t
	}

	if token.I == "" {
		id = jobs.NewRequestIdentifier()
	} else {
		i, errr := token.RequestId()
		if errr != nil {
			rerr = &apiRequestError{errr, "Unable to parse token for this request: " + errr.Error(), http.StatusBadRequest}
			return
		}
		id = i
	}

	return
}
