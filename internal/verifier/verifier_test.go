package verifier

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestVerifyIndependentChecks(t *testing.T) {
	clean := strings.Repeat("ab \n", 8)
	uneven := "ab\n" + strings.Repeat("ab \n", 7)
	tests := []struct {
		name    string
		content string
		widths  []int
		checks  []bool
		score   int
		verdict string
	}{
		{"clean", clean, []int{3}, []bool{true, true, true, true, true}, 5, "Valid"},
		{"missing final LF", strings.TrimSuffix(clean, "\n"), []int{3}, []bool{true, true, true, true, false}, 4, "Acceptable"},
		{"one shortened line", uneven, []int{3}, []bool{true, true, false, false, true}, 3, "Corrupted"},
		{"shortened line and final LF", strings.TrimSuffix(uneven, "\n"), []int{3}, []bool{true, true, false, false, false}, 2, "Corrupted"},
		{"uniform extra columns", strings.Repeat("ab  \n", 8), []int{3}, []bool{true, true, true, false, true}, 4, "Acceptable"},
		{"uniform removed columns", strings.Repeat("ab\n", 8), []int{3}, []bool{true, true, true, false, true}, 4, "Acceptable"},
		{"extra blank line", clean + "\n", []int{3}, []bool{true, false, false, false, true}, 2, "Corrupted"},
		{"missing line", strings.Repeat("ab \n", 7), []int{3}, []bool{true, false, false, false, true}, 2, "Corrupted"},
		{"extra whole block", clean + clean, []int{3}, []bool{true, false, false, false, true}, 2, "Corrupted"},
		{"empty with no rows", "", nil, []bool{true, true, true, true, true}, 5, "Valid"},
		{"empty with one row", "", []int{0}, []bool{true, false, false, false, true}, 2, "Corrupted"},
		{"empty text row", strings.Repeat("\n", 8), []int{0}, []bool{true, true, true, true, true}, 5, "Valid"},
		{"different widths by row", clean + strings.Repeat("abcd \n", 8), []int{3, 5}, []bool{true, true, true, true, true}, 5, "Valid"},
		{"negative width", clean, []int{-1}, []bool{true, true, true, false, true}, 4, "Acceptable"},
		{"tab", strings.Repeat("ab\t\n", 8), []int{3}, []bool{true, true, true, false, true}, 4, "Acceptable"},
		{"CRLF", strings.Repeat("ab\r\n", 8), []int{3}, []bool{true, true, true, false, true}, 4, "Acceptable"},
		{"NUL", strings.Repeat("ab\x00\n", 8), []int{3}, []bool{true, true, true, false, true}, 4, "Acceptable"},
		{"UTF8 counted in bytes", strings.Repeat("é \n", 8), []int{3}, []bool{true, true, true, false, true}, 4, "Acceptable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := createTestFile(t, tt.content)
			report := Verify(path, tt.widths)
			if len(report.Checks) != 5 {
				t.Fatalf("got %d checks, want 5", len(report.Checks))
			}
			for i, want := range tt.checks {
				if got := report.Checks[i].Passed; got != want {
					t.Errorf("check %d: got %t, want %t; %s", i+1, got, want, report.Checks[i].Detail)
				}
			}
			if report.Score != tt.score || report.Verdict != tt.verdict {
				t.Errorf("got %d/5 %s, want %d/5 %s", report.Score, report.Verdict, tt.score, tt.verdict)
			}
		})
	}
}

func TestUnreadableAndUnsafeFiles(t *testing.T) {
	dir := t.TempDir()
	oversized := filepath.Join(dir, "oversized.txt")
	file, err := os.Create(oversized)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(MaxFileSize + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(dir, "symlink.txt")
	if err := os.Symlink(createTestFile(t, strings.Repeat("ab \n", 8)), symlink); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(dir, "missing.txt"), dir, oversized, symlink} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			report := Verify(path, []int{3})
			if report.Score != 0 || report.Verdict != "Corrupted" {
				t.Errorf("unreadable file got %+v", report)
			}
			for _, check := range report.Checks {
				if check.Passed {
					t.Errorf("unreadable file passed %s", check.Name)
				}
			}
			if _, err := Tamper(path, ""); err == nil {
				t.Error("tamper check should return the read failure")
			}
		})
	}
}

func TestVerifyRereadsDisk(t *testing.T) {
	path := createTestFile(t, strings.Repeat("ab \n", 8))
	if report := Verify(path, []int{3}); report.Score != 5 {
		t.Fatalf("initial report: %+v", report)
	}
	if err := os.WriteFile(path, []byte("broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if report := Verify(path, []int{3}); report.Score >= 4 {
		t.Errorf("changed disk data was not detected: %+v", report)
	}
}

func TestExcessiveLineCount(t *testing.T) {
	for _, content := range []string{
		strings.Repeat("\n", MaxFileLines+1),
		strings.Repeat("\n", MaxFileLines) + "x",
	} {
		path := createTestFile(t, content)
		report := Verify(path, nil)
		if report.Score != 0 || report.Verdict != "Corrupted" {
			t.Errorf("excessive lines got %+v", report)
		}
		if !strings.Contains(report.Checks[0].Detail, "line limit") {
			t.Errorf("missing line-limit explanation: %s", report.Checks[0].Detail)
		}
		if _, err := Tamper(path, ""); err == nil || !strings.Contains(err.Error(), "line limit") {
			t.Errorf("tamper check: got %v, want line-limit error", err)
		}
	}

	// Предельное число разделителей законно: последняя завершающая '\n'
	// не превращается в лишнюю пустую строку при подсчёте лимита.
	boundary := strings.Repeat("\n", MaxFileLines)
	path := createTestFile(t, boundary)
	data, err := readRegularFile(path)
	if err != nil {
		t.Fatalf("boundary file should be readable: %v", err)
	}
	if string(data) != boundary {
		t.Error("boundary file content changed during reading")
	}
}

func TestTamperExactLineDifferences(t *testing.T) {
	tests := []struct {
		name     string
		disk     string
		fresh    string
		differed []int
	}{
		{"unchanged", "one\ntwo\n", "one\ntwo\n", nil},
		{"empty unchanged", "", "", nil},
		{"changed fourth line", "a\nb\nc\nchanged\ne\n", "a\nb\nc\nd\ne\n", []int{4}},
		{"multiple changes", "x\nb\nz\n", "a\nb\nc\n", []int{1, 3}},
		{"missing lines", "a\n", "a\nb\nc\n", []int{2, 3}},
		{"extra lines", "a\nb\nc\n", "a\n", []int{2, 3}},
		{"missing final LF", "a\nb", "a\nb\n", []int{2}},
		{"extra final LF", "a\nb\n", "a\nb", []int{2}},
		{"extra blank line", "a\n\n", "a\n", []int{2}},
		{"empty versus newline", "", "\n", []int{1}},
		{"CR difference", "a\r\n", "a\n", []int{1}},
		{"space difference", "a \n", "a\n", []int{1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := createTestFile(t, tt.disk)
			result, err := Tamper(path, tt.fresh)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Lines, tt.differed) {
				t.Errorf("changed lines: got %v, want %v", result.Lines, tt.differed)
			}
			if result.Unmodified != (len(tt.differed) == 0) {
				t.Errorf("unmodified is %t for %v", result.Unmodified, result.Lines)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != tt.disk {
				t.Error("tamper check modified the file")
			}
		})
	}
}

func TestReportMatchesSubject(t *testing.T) {
	path := createTestFile(t, strings.Repeat(strings.Repeat(" ", 31)+"\n", 8))
	var out bytes.Buffer
	if err := RenderReport(&out, Verify(path, []int{31})); err != nil {
		t.Fatal(err)
	}
	want := "\n--- Verification Report ---\nIntegrity Score: 5/5 (Valid)\n\nChecks:\n" +
		"  [PASS] File is readable\n" +
		"  [PASS] Line count is 8 (8 per row x 1 row)\n" +
		"  [PASS] Width consistent within each row (31)\n" +
		"  [PASS] No padding beyond row width\n" +
		"  [PASS] File size matches (256 bytes)\n"
	if out.String() != want {
		t.Errorf("report:\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestRenderTamper(t *testing.T) {
	tests := []struct {
		result TamperResult
		want   string
	}{
		{TamperResult{Unmodified: true}, "Result: file matches a fresh regeneration (unmodified)\n"},
		{TamperResult{Lines: []int{4}}, "Result: 1 line differs from a fresh regeneration\n  Line 4 differs\n"},
		{TamperResult{Lines: []int{2, 5}}, "Result: 2 lines differ from a fresh regeneration\n  Line 2 differs\n  Line 5 differs\n"},
	}
	for _, tt := range tests {
		var out bytes.Buffer
		if err := RenderTamper(&out, tt.result); err != nil {
			t.Fatal(err)
		}
		if want := "\n--- Tamper Check ---\n" + tt.want; out.String() != want {
			t.Errorf("got %q, want %q", out.String(), want)
		}
	}
}

func TestRenderPropagatesWriteErrors(t *testing.T) {
	problem := errors.New("write failed")
	for _, render := range []func(io.Writer) error{
		func(out io.Writer) error { return RenderReport(out, Report{}) },
		func(out io.Writer) error { return RenderTamper(out, TamperResult{}) },
	} {
		if err := render(failingWriter{err: problem}); !errors.Is(err, problem) {
			t.Errorf("got %v, want %v", err, problem)
		}
		if err := render(failingWriter{}); !errors.Is(err, io.ErrShortWrite) {
			t.Errorf("got %v, want short write", err)
		}
		if err := render(nil); err == nil {
			t.Error("nil writer should return an error")
		}
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write(_ []byte) (int, error) { return 0, w.err }

func createTestFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "output.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func FuzzVerify(f *testing.F) {
	clean := []byte(strings.Repeat("ab \n", 8))
	f.Add(clean, uint16(3))
	f.Add(bytes.TrimSuffix(clean, []byte{'\n'}), uint16(3))
	f.Add([]byte("\x00\t\xff\r\n"), uint16(4))
	f.Add([]byte{}, uint16(0))
	f.Fuzz(func(t *testing.T, content []byte, width uint16) {
		// Небольшие входы дают больше различных проверок за время fuzz-теста.
		if len(content) > 8192 {
			t.Skip()
		}
		path := createTestFile(t, string(content))
		report := Verify(path, []int{int(width)})
		if len(report.Checks) != 5 {
			t.Fatalf("got %d checks, want 5", len(report.Checks))
		}
		passed := 0
		for _, check := range report.Checks {
			if check.Passed {
				passed++
			}
		}
		if report.Score != passed {
			t.Fatalf("score %d differs from passed-check count %d", report.Score, passed)
		}
		wantVerdict := "Corrupted"
		if passed == 5 {
			wantVerdict = "Valid"
		} else if passed == 4 {
			wantVerdict = "Acceptable"
		}
		if report.Verdict != wantVerdict {
			t.Fatalf("got verdict %s, want %s", report.Verdict, wantVerdict)
		}
		if !report.Checks[0].Passed {
			t.Fatalf("readable temporary file was rejected: %s", report.Checks[0].Detail)
		}
		result, err := Tamper(path, string(content))
		if err != nil || !result.Unmodified || len(result.Lines) != 0 {
			t.Fatalf("identical disk and memory differ: %+v, %v", result, err)
		}
	})
}
