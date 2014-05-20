package config

import (
	"testing"
)

type Config_Test struct{}

var configConfigTest = &Config_Test{}

func Test_Config_SetContainerRunPath(t *testing.T) {
	var err error
	expected := "/unit/test"

	err = SetContainerRunPath(expected)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	actual := ContainerRunPath()

	if actual != expected {
		t.Errorf("Expected: %s, actual %s", expected, actual)
	}

	err = SetContainerRunPath("")

	if err == nil {
		t.Errorf("Expected error for empty string")
	}
}

func Test_Config_SetContainerBasePath(t *testing.T) {
	var err error
	expected := "/unit/test"

	err = SetContainerBasePath(expected)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	actual := ContainerBasePath()

	if actual != expected {
		t.Errorf("Expected: %s, actual %s", expected, actual)
	}

	err = SetContainerBasePath("")

	if err == nil {
		t.Errorf("Expected error for empty string")
	}
}
