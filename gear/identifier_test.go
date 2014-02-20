package gear

import (
	"testing"
)

func TestNewIdentifier(t *testing.T) {
	if out, _ := NewIdentifier(""); out != InvalidIdentifier {
		t.Error("Empty identifier should return InvalidIdentifier")
	}
	if _, err := NewIdentifier(""); err == nil {
		t.Error("Empty identifier should return a valid error")
	}
	if _, err := NewIdentifier(""); "Gear identifier may not be empty" != err.Error() {
		t.Error("Empty identifier should return appropriate message")
	}
	if _, err := NewIdentifier("^^^^"); err == nil {
		t.Error("Identifier should disallow special characters")
	}
}
