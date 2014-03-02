package utils

import (
	"strings"
)

func TakePrefix(s string, prefix string) (string, bool) {
	if strings.HasPrefix(s, prefix) {
		return s[len(prefix):], true
	}
	return s, false
}
