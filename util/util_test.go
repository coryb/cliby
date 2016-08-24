package util

import (
	"bytes"
	"strings"
	"testing"
)

type Options struct {
	AliasOpts                  map[string]interface{}
}

type CommandData struct {
	Options
	AliasOpts map[string]interface{}
}

// Test to prove templating doesn't natively fetch keys with special characters, i.e. '-'
func TestRunTemplateWithDashOptNameFails(t *testing.T) {
	options := Options {
			map[string]interface {}{"enable-debug": false},
	}
	data := CommandData{
		Options: options,
	}
	buf := bytes.NewBufferString("")
	err := RunTemplate(`{{ .Options.AliasOpts.enable-debug }}`, data, buf)
	if err != nil {
		if !strings.Contains(err.Error(), "bad character U+002D '-'") {
			t.Error(err)
		}
	} else {
		t.Error("expected this to not work")
	}
}

// Workaround to fetch keys with special characters, i.e. '-'
func TestRunTemplateWithDashOptName(t *testing.T) {
	options := Options {
			map[string]interface {}{"enable-debug": false},
	}
	data := CommandData{
		Options: options,
	}
	buf := bytes.NewBufferString("")
	err := RunTemplate(`{{ specialCharMap .Options.AliasOpts "enable-debug" }}`, data, buf)
	if err != nil {
		log.Errorf("Failed to process template")
		t.Fail()
	}

	if buf.String() != "false" {
		t.Error("expected false, but got : " + buf.String())
	}
}

// Test if you ask for a non-existent key you get <no value>
func TestRunTemplateWithNonexistentKey(t *testing.T) {
	options := Options {
			map[string]interface {}{"foo": false},
	}
	data := CommandData{
		Options: options,
	}
	buf := bytes.NewBufferString("")
	err := RunTemplate(`{{ specialCharMap .Options.AliasOpts "enable-debug" }}`, data, buf)
	if err != nil {
		log.Errorf("Failed to process template")
		t.Fail()
	}

	if buf.String() != "<no value>" {
		t.Error("expected nil, but got : " + buf.String())
	}
}
