package setupwizard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDotEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := strings.Join([]string{
		"# comment",
		"export FOO=bar",
		`BAZ="hello world"`,
		"QUX='single'",
		"SPACED=  hi  ",
		"EMPTY=",
		"NOEQUALS",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	env, err := parseDotEnvFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if env["FOO"] != "bar" {
		t.Fatalf("FOO=%q, want %q", env["FOO"], "bar")
	}
	if env["BAZ"] != "hello world" {
		t.Fatalf("BAZ=%q, want %q", env["BAZ"], "hello world")
	}
	if env["QUX"] != "single" {
		t.Fatalf("QUX=%q, want %q", env["QUX"], "single")
	}
	if env["SPACED"] != "hi" {
		t.Fatalf("SPACED=%q, want %q", env["SPACED"], "hi")
	}
	if _, ok := env["EMPTY"]; !ok {
		t.Fatalf("expected EMPTY key to exist")
	}
}

func TestWriteDotEnvFile_SortsAndEscapes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	env := map[string]string{
		"B": "a b",
		"A": "x",
	}

	if err := writeDotEnvFile(path, env, dotenvWriteOptions{
		headerComment: "Line1\n#Line2",
		mode:          0600,
	}); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)

	if !strings.Contains(text, "# Line1\n#Line2\n\n") {
		t.Fatalf("unexpected header:\n%s", text)
	}

	wantOrder := []string{"A=x", `B="a b"`}
	idxA := strings.Index(text, wantOrder[0])
	idxB := strings.Index(text, wantOrder[1])
	if idxA == -1 || idxB == -1 || idxA > idxB {
		t.Fatalf("expected sorted keys %q then %q, got:\n%s", wantOrder[0], wantOrder[1], text)
	}
}

