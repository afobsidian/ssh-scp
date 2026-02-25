package ssh

import (
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// shellQuote
// ---------------------------------------------------------------------------

func TestShellQuoteSimple(t *testing.T) {
	got := shellQuote("/home/user/file.txt")
	want := "'/home/user/file.txt'"
	if got != want {
		t.Errorf("shellQuote simple = %q, want %q", got, want)
	}
}

func TestShellQuoteWithSingleQuote(t *testing.T) {
	got := shellQuote("/home/user/it's a file")
	want := "'/home/user/it'\\''s a file'"
	if got != want {
		t.Errorf("shellQuote with quote = %q, want %q", got, want)
	}
}

func TestShellQuoteEmpty(t *testing.T) {
	got := shellQuote("")
	want := "''"
	if got != want {
		t.Errorf("shellQuote empty = %q, want %q", got, want)
	}
}

func TestShellQuoteSpaces(t *testing.T) {
	got := shellQuote("/path/with spaces")
	if !strings.HasPrefix(got, "'") || !strings.HasSuffix(got, "'") {
		t.Errorf("shellQuote should wrap in single quotes, got %q", got)
	}
}

func TestShellQuoteMultipleSingleQuotes(t *testing.T) {
	got := shellQuote("it's a 'test' path")
	if strings.Count(got, `'\''`) != 3 {
		t.Errorf("should escape 3 single quotes, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// splitLines
// ---------------------------------------------------------------------------

func TestSplitLinesMultiple(t *testing.T) {
	got := splitLines("line1\nline2\nline3")
	if len(got) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(got))
	}
	if got[0] != "line1" || got[1] != "line2" || got[2] != "line3" {
		t.Errorf("unexpected lines: %v", got)
	}
}

func TestSplitLinesTrailingNewline(t *testing.T) {
	got := splitLines("line1\nline2\n")
	if len(got) != 2 {
		t.Fatalf("expected 2 lines (trailing newline stripped), got %d: %v", len(got), got)
	}
}

func TestSplitLinesEmpty(t *testing.T) {
	got := splitLines("")
	if len(got) != 0 {
		t.Errorf("expected 0 lines, got %d", len(got))
	}
}

func TestSplitLinesSingle(t *testing.T) {
	got := splitLines("hello")
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("expected [hello], got %v", got)
	}
}

// ---------------------------------------------------------------------------
// splitFields
// ---------------------------------------------------------------------------

func TestSplitFieldsSpaces(t *testing.T) {
	got := splitFields("  a   b   c  ")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("splitFields with spaces = %v", got)
	}
}

func TestSplitFieldsTabs(t *testing.T) {
	got := splitFields("a\tb\tc")
	if len(got) != 3 {
		t.Errorf("splitFields with tabs, expected 3, got %d: %v", len(got), got)
	}
}

func TestSplitFieldsEmpty(t *testing.T) {
	got := splitFields("")
	if len(got) != 0 {
		t.Errorf("splitFields empty, expected 0, got %d", len(got))
	}
}

func TestSplitFieldsSingle(t *testing.T) {
	got := splitFields("hello")
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("splitFields single = %v", got)
	}
}

// ---------------------------------------------------------------------------
// parsePerm
// ---------------------------------------------------------------------------

func TestParsePermFull(t *testing.T) {
	mode := parsePerm("-rwxrwxrwx")
	if mode != 0777 {
		t.Errorf("parsePerm -rwxrwxrwx = %o, want 0777", mode)
	}
}

func TestParsePermReadOnly(t *testing.T) {
	mode := parsePerm("-r--r--r--")
	if mode != 0444 {
		t.Errorf("parsePerm -r--r--r-- = %o, want 0444", mode)
	}
}

func TestParsePermOwnerOnly(t *testing.T) {
	mode := parsePerm("-rwx------")
	if mode != 0700 {
		t.Errorf("parsePerm -rwx------ = %o, want 0700", mode)
	}
}

func TestParsePermDirectory(t *testing.T) {
	mode := parsePerm("drwxr-xr-x")
	if mode != 0755 {
		t.Errorf("parsePerm drwxr-xr-x = %o, want 0755", mode)
	}
}

func TestParsePermShortString(t *testing.T) {
	mode := parsePerm("short")
	if mode != 0 {
		t.Errorf("parsePerm short input = %o, want 0", mode)
	}
}

func TestParsePermEmpty(t *testing.T) {
	mode := parsePerm("")
	if mode != 0 {
		t.Errorf("parsePerm empty = %o, want 0", mode)
	}
}

func TestParsePermNoExec(t *testing.T) {
	mode := parsePerm("-rw-rw-rw-")
	if mode != 0666 {
		t.Errorf("parsePerm -rw-rw-rw- = %o, want 0666", mode)
	}
}

func TestParsePermTypical600(t *testing.T) {
	mode := parsePerm("-rw-------")
	if mode != 0600 {
		t.Errorf("parsePerm -rw------- = %o, want 0600", mode)
	}
}

// ---------------------------------------------------------------------------
// parseLSLine
// ---------------------------------------------------------------------------

func TestParseLSLineRegularFile(t *testing.T) {
	line := "-rw-r--r-- 1 user group 12345 2024-01-15 10:30:00 myfile.txt"
	f := parseLSLine(line)
	if f == nil {
		t.Fatal("parseLSLine returned nil for valid line")
	}
	if f.Name != "myfile.txt" {
		t.Errorf("Name = %q, want %q", f.Name, "myfile.txt")
	}
	if f.Size != 12345 {
		t.Errorf("Size = %d, want 12345", f.Size)
	}
	if f.IsDir {
		t.Error("should not be dir")
	}
	if f.Mode != 0644 {
		t.Errorf("Mode = %o, want 0644", f.Mode)
	}
}

func TestParseLSLineDirectory(t *testing.T) {
	line := "drwxr-xr-x 2 user group 4096 2024-01-15 10:30:00 mydir"
	f := parseLSLine(line)
	if f == nil {
		t.Fatal("parseLSLine returned nil for dir line")
	}
	if f.Name != "mydir" {
		t.Errorf("Name = %q, want %q", f.Name, "mydir")
	}
	if !f.IsDir {
		t.Error("should be dir")
	}
}

func TestParseLSLineShort(t *testing.T) {
	f := parseLSLine("abc")
	if f != nil {
		t.Errorf("expected nil for short line, got %+v", f)
	}
}

func TestParseLSLineEmpty(t *testing.T) {
	f := parseLSLine("")
	if f != nil {
		t.Errorf("expected nil for empty line, got %+v", f)
	}
}

func TestParseLSLineFiveFields(t *testing.T) {
	// Some ls formats have fewer fields
	line := "-rw-r--r-- 1 user group readme.txt"
	f := parseLSLine(line)
	if f == nil {
		t.Fatal("should parse 5-field line")
	}
	if f.Name != "readme.txt" {
		t.Errorf("Name = %q, want %q", f.Name, "readme.txt")
	}
}

// ---------------------------------------------------------------------------
// parseLS
// ---------------------------------------------------------------------------

func TestParseLSBasic(t *testing.T) {
	output := `total 16
drwxr-xr-x 2 user user 4096 2024-01-15 10:00:00 .
drwxr-xr-x 3 user user 4096 2024-01-15 10:00:00 ..
-rw-r--r-- 1 user user 1024 2024-01-15 10:00:00 file1.txt
-rw-r--r-- 1 user user 2048 2024-01-15 10:00:00 file2.txt
drwxr-xr-x 2 user user 4096 2024-01-15 10:00:00 subdir
`
	files := parseLS(output)
	if len(files) != 3 {
		t.Fatalf("expected 3 files (excluding . and ..), got %d", len(files))
	}

	names := make(map[string]bool)
	for _, f := range files {
		names[f.Name] = true
	}
	for _, n := range []string{"file1.txt", "file2.txt", "subdir"} {
		if !names[n] {
			t.Errorf("missing file %q", n)
		}
	}
}

func TestParseLSEmpty(t *testing.T) {
	files := parseLS("")
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestParseLSTotalOnly(t *testing.T) {
	files := parseLS("total 0\n")
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestParseLSSkipsDotEntries(t *testing.T) {
	output := "drwxr-xr-x 2 u g 4096 2024-01-15 10:00:00 .\ndrwxr-xr-x 2 u g 4096 2024-01-15 10:00:00 ..\n"
	files := parseLS(output)
	if len(files) != 0 {
		t.Errorf("expected 0 after filtering . and .., got %d", len(files))
	}
}

// ---------------------------------------------------------------------------
// PasswordAuth
// ---------------------------------------------------------------------------

func TestPasswordAuth(t *testing.T) {
	auth := PasswordAuth("secret")
	if auth == nil {
		t.Fatal("PasswordAuth returned nil")
	}
}

// ---------------------------------------------------------------------------
// PubKeyAuth
// ---------------------------------------------------------------------------

func TestPubKeyAuthNonexistentFile(t *testing.T) {
	_, err := PubKeyAuth("/nonexistent/path/key")
	if err == nil {
		t.Error("expected error for nonexistent key file")
	}
}

func TestPubKeyAuthInvalidKeyContent(t *testing.T) {
	tmp := t.TempDir()
	keyFile := tmp + "/badkey"
	if err := os.WriteFile(keyFile, []byte("not a real key"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := PubKeyAuth(keyFile)
	if err == nil {
		t.Error("expected error for invalid key content")
	}
}

// ---------------------------------------------------------------------------
// RemoteFile struct
// ---------------------------------------------------------------------------

func TestRemoteFileFields(t *testing.T) {
	f := RemoteFile{
		Name:  "test.txt",
		Size:  1024,
		IsDir: false,
	}
	if f.Name != "test.txt" || f.Size != 1024 || f.IsDir {
		t.Errorf("RemoteFile fields not set correctly: %+v", f)
	}
}
