package encrypted

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

type TokenData struct {
	Identifier     string `json:"i,omitempty"` // request identifier
	ExpirationDate int64  `json:"d,omitempty"` // expiration time in seconds from the epoch
	Content        string `json:"c,omitempty"` // opaque content
}

func (t *TokenData) ToValues(values *url.Values) {
	if t.Identifier != "" {
		values.Set("i", t.Identifier)
	}
	if t.Content != "" {
		values.Set("c", t.Content)
	}
	if t.ExpirationDate != 0 {
		values.Set("d", strconv.FormatInt(t.ExpirationDate, 10))
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
	token.Identifier = firstParam(m, "i")
	if len(token.Identifier) > 0 && len(token.Identifier) < 32 {
		token.Identifier = strings.Repeat("0", 32-len(token.Identifier)) + token.Identifier
	}
	date := firstParam(m, "d")
	if date != "" {
		d, err := strconv.ParseInt(date, 10, 64)
		if err != nil {
			return nil, err
		}
		token.ExpirationDate = d
	}
	token.Content = firstParam(m, "c")
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
