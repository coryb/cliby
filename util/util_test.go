package util

import (
	"bytes"
	"os"
	"testing"
)

func TestRunTemplateFindLatestArtifact(t *testing.T) {
	// setup
	if err := os.MkdirAll("foo/bar", os.ModePerm); err != nil {
		t.Error(err)
	}
	fa, err := os.Create("foo/bar/old.txt")
	if err != nil {
		os.RemoveAll("foo")
		t.Error(err)
	}
	fa.Close()
	fb, err := os.Create("foo/bar/baz.txt")
	if err != nil {
		os.RemoveAll("foo")
		t.Error(err)
	}
	fb.Close()

	var blank map[string]string
	var buf, expected bytes.Buffer
	if err := RunTemplate(`Find glob: {{findLatestArtifact "**/*.txt"}}`, blank, &buf); err != nil {
		t.Error(err)
	}

	expected.WriteString("Find glob: foo/bar/baz.txt")
	if buf.String() != expected.String() {
		t.Errorf("Expected %v but recieved %v", expected.String(), buf.String())
	}

	// Cleanup
	os.RemoveAll("foo")
}

func TestRunTemplateCwd(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Error(err)
	}
	var blank map[string]string
	var buf bytes.Buffer
	if err := RunTemplate("{{cwd}}", blank, &buf); err != nil {
		t.Error(err)
	}
	if buf.String() != wd {
		t.Errorf("Expected %v but recieved %v", wd, buf.String())
	}
}
