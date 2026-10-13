package cli

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// Образец фиксирует результат из standard.txt независимо от вызовов генератора.
const helloArt = " _              _   _          \n" +
	"| |            | | | |         \n" +
	"| |__     ___  | | | |   ___   \n" +
	"|  _ \\   / _ \\ | | | |  / _ \\  \n" +
	"| | | | |  __/ | | | | | (_) | \n" +
	"|_| |_|  \\___| |_| |_|  \\___/  \n" +
	"                               \n" +
	"                               \n"

const helloReport = "\n--- Verification Report ---\nIntegrity Score: 5/5 (Valid)\n\nChecks:\n" +
	"  [PASS] File is readable\n" +
	"  [PASS] Line count is 8 (8 per row x 1 row)\n" +
	"  [PASS] Width consistent within each row (31)\n" +
	"  [PASS] No padding beyond row width\n" +
	"  [PASS] File size matches (256 bytes)\n"

func TestParseValidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want Options
	}{
		{"default banner", []string{"hello"}, Options{Input: "hello", Banner: "standard"}},
		{"explicit standard", []string{"hello", "standard"}, Options{Input: "hello", Banner: "standard"}},
		{"shadow", []string{"hello", "shadow"}, Options{Input: "hello", Banner: "shadow"}},
		{"thinkertoy", []string{"hello", "thinkertoy"}, Options{Input: "hello", Banner: "thinkertoy"}},
		{"empty input", []string{""}, Options{Banner: "standard"}},
		{"output first", []string{"--output=result.txt", "hello"}, Options{Input: "hello", Banner: "standard", Output: "result.txt"}},
		{"output middle", []string{"hello", "--output=result.txt", "shadow"}, Options{Input: "hello", Banner: "shadow", Output: "result.txt"}},
		{"output last", []string{"hello", "thinkertoy", "--output=result.txt"}, Options{Input: "hello", Banner: "thinkertoy", Output: "result.txt"}},
		{"all flags anywhere", []string{"--verify", "hello", "--output=result.txt", "--tamper-check", "standard"}, Options{Input: "hello", Banner: "standard", Output: "result.txt", Verify: true, TamperCheck: true}},
		{"path with spaces", []string{"hello", "--output=directory with spaces/result.txt"}, Options{Input: "hello", Banner: "standard", Output: "directory with spaces/result.txt"}},
		{"delimiter treats flag as text", []string{"--", "--verify"}, Options{Input: "--verify", Banner: "standard"}},
		{"delimiter after flag", []string{"--output=result.txt", "--", "-hello", "shadow"}, Options{Input: "-hello", Banner: "shadow", Output: "result.txt"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Parse(test.args)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("Parse(%q) = %+v; ожидается %+v", test.args, got, test.want)
			}
		})
	}
}

func TestParseInvalidArgumentsPrintUsage(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"missing input", nil},
		{"missing input with output", []string{"--output=result.txt"}},
		{"missing input with all flags", []string{"--output=result.txt", "--verify", "--tamper-check"}},
		{"too many positionals", []string{"hello", "standard", "extra"}},
		{"unknown banner", []string{"hello", "missing"}},
		{"uppercase banner", []string{"hello", "Standard"}},
		{"empty banner", []string{"hello", ""}},
		{"separated output value", []string{"--output", "result.txt", "hello"}},
		{"single dash output", []string{"-output=result.txt", "hello"}},
		{"missing dash output", []string{"output=result.txt", "hello"}},
		{"empty output value", []string{"--output=", "hello"}},
		{"missing extension", []string{"--output=result", "hello"}},
		{"wrong extension", []string{"--output=result.png", "hello"}},
		{"uppercase extension", []string{"--output=result.TXT", "hello"}},
		{"NUL in output", []string{"--output=bad\x00.txt", "hello"}},
		{"verify without file", []string{"--verify", "hello"}},
		{"tamper without file", []string{"--tamper-check", "hello"}},
		{"verify value", []string{"--verify=true", "--output=result.txt", "hello"}},
		{"tamper value", []string{"--tamper-check=true", "--output=result.txt", "hello"}},
		{"duplicate output", []string{"--output=first.txt", "hello", "--output=second.txt"}},
		{"duplicate verify", []string{"--verify", "--output=result.txt", "hello", "--verify"}},
		{"duplicate tamper", []string{"--tamper-check", "--output=result.txt", "hello", "--tamper-check"}},
		{"unknown flag", []string{"--unknown", "hello"}},
		{"text beginning with dash", []string{"-hello"}},
		{"empty delimiter", []string{"--"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Parse(test.args); err == nil {
				t.Fatalf("Parse(%q) должен отклонить аргументы", test.args)
			}
			status, stdout, stderr := runCLI(test.args, nil)
			if status != 2 || stdout != Usage || stderr != "" {
				t.Fatalf("Usage: status=%d, stdout=%q, stderr=%q", status, stdout, stderr)
			}
		})
	}
}

func TestRunHelloToStdout(t *testing.T) {
	status, stdout, stderr := runCLI([]string{"hello"}, os.DirFS("../.."))
	if status != 0 || stdout != helloArt || stderr != "" {
		t.Fatalf("рисунок: status=%d, stdout=%q, stderr=%q", status, stdout, stderr)
	}
	assertHelloGeometry(t, []byte(stdout))
}

func TestRunOutputCreatesOnlyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.txt")
	status, stdout, stderr := runCLI([]string{"--output=" + path, "hello", "standard"}, os.DirFS("../.."))
	if status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("запись: status=%d, stdout=%q, stderr=%q", status, stdout, stderr)
	}
	assertFileBytes(t, path, []byte(helloArt))
	assertHelloGeometry(t, readFile(t, path))
}

func TestRunVerifyMatchesSubjectReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.txt")
	status, stdout, stderr := runCLI([]string{"hello", "--verify", "--output=" + path}, os.DirFS("../.."))
	want := "File created: " + path + "\n" + helloReport
	if status != 0 || stdout != want || stderr != "" {
		t.Fatalf("проверка: status=%d, stdout=%q, stderr=%q; ожидается %q", status, stdout, stderr, want)
	}
	assertFileBytes(t, path, []byte(helloArt))
}

func TestRunTamperKeepsEvidenceAndDetectsFourthLine(t *testing.T) {
	for _, verify := range []bool{false, true} {
		name := "tamper only"
		if verify {
			name = "verify and tamper"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "result.txt")
			// Меняем знак, сохраняя количество строк, столбцов и байтов.
			changed := []byte(helloArt)
			changed[3*32] = '#'
			if err := os.WriteFile(path, changed, 0644); err != nil {
				t.Fatal(err)
			}
			args := []string{"--output=" + path, "hello", "--tamper-check"}
			if verify {
				args = append(args, "--verify")
			}
			status, stdout, stderr := runCLI(args, os.DirFS("../.."))
			want := "\n--- Tamper Check ---\nResult: 1 line differs from a fresh regeneration\n  Line 4 differs\n"
			if verify {
				want = helloReport + want
			}
			if status != 1 || stdout != want || stderr != "" {
				t.Fatalf("изменение: status=%d, stdout=%q, stderr=%q; ожидается %q", status, stdout, stderr, want)
			}
			assertFileBytes(t, path, changed)
		})
	}
}

func TestRunTamperUnmodifiedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.txt")
	if err := os.WriteFile(path, []byte(helloArt), 0644); err != nil {
		t.Fatal(err)
	}
	args := []string{"hello", "--output=" + path, "--verify", "--tamper-check"}
	status, stdout, stderr := runCLI(args, os.DirFS("../.."))
	want := helloReport + "\n--- Tamper Check ---\nResult: file matches a fresh regeneration (unmodified)\n"
	if status != 0 || stdout != want || stderr != "" {
		t.Fatalf("исходный файл: status=%d, stdout=%q, stderr=%q", status, stdout, stderr)
	}
	assertFileBytes(t, path, []byte(helloArt))
}

func TestRunTamperMissingFileDoesNotCreateIt(t *testing.T) {
	for _, verify := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "missing.txt")
		args := []string{"hello", "--output=" + path, "--tamper-check"}
		if verify {
			args = append(args, "--verify")
		}
		status, stdout, stderr := runCLI(args, os.DirFS("../.."))
		if status != 1 || stdout != "" || stderr == "" {
			t.Fatalf("отсутствующий файл: status=%d, stdout=%q, stderr=%q", status, stdout, stderr)
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("проверка создала файл: %v", err)
		}
	}
}

func TestRunUnsupportedInputDoesNotOverwriteFile(t *testing.T) {
	for _, input := range []string{"Привет", "hello\tworld", "hello\rworld", "hello\x00world", "hello\xffworld"} {
		t.Run(input, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "result.txt")
			original := []byte("сохранить исходный файл\n")
			if err := os.WriteFile(path, original, 0644); err != nil {
				t.Fatal(err)
			}
			status, stdout, stderr := runCLI([]string{input, "--output=" + path}, os.DirFS("../.."))
			if status != 1 || stdout != "" || stderr == "" {
				t.Fatalf("неподдерживаемый ввод: status=%d, stdout=%q, stderr=%q", status, stdout, stderr)
			}
			assertFileBytes(t, path, original)
		})
	}
}

func TestRunProtectsFontsAndGitPaths(t *testing.T) {
	for _, name := range []string{"standard.txt", "shadow.txt", "thinkertoy.txt", "STANDARD.txt", filepath.Join(".git", "result.txt"), filepath.Join(".GIT", "result.txt")} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			original := []byte("защищённые данные\n")
			if !strings.Contains(strings.ToLower(name), ".git") {
				original = readFile(t, filepath.Join("../..", strings.ToLower(name)))
			}
			if err := os.WriteFile(path, original, 0644); err != nil {
				t.Fatal(err)
			}
			status, stdout, stderr := runCLI([]string{"hello", "--output=" + path}, os.DirFS("../.."))
			if status != 1 || stdout != "" || stderr == "" {
				t.Fatalf("защищённый путь: status=%d, stdout=%q, stderr=%q", status, stdout, stderr)
			}
			assertFileBytes(t, path, original)
		})
	}
}

func TestRunAllowsFontNameForOrdinaryOutput(t *testing.T) {
	for _, name := range []string{"standard.txt", "shadow.txt", "thinkertoy.txt"} {
		path := filepath.Join(t.TempDir(), name)
		status, _, stderr := runCLI([]string{"hello", "--output=" + path}, os.DirFS("../.."))
		if status != 0 || stderr != "" {
			t.Fatalf("обычное имя файла %s: status=%d stderr=%q", name, status, stderr)
		}
		assertFileBytes(t, path, []byte(helloArt))
	}
}

func TestRunRejectsSymlinkAndDirectoryOutput(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		directory := t.TempDir()
		target := filepath.Join(directory, "original.txt")
		path := filepath.Join(directory, "result.txt")
		original := []byte("сохранить\n")
		if err := os.WriteFile(target, original, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Skipf("ссылки недоступны: %v", err)
		}
		status, stdout, stderr := runCLI([]string{"hello", "--output=" + path}, os.DirFS("../.."))
		if status != 1 || stdout != "" || stderr == "" {
			t.Fatalf("ссылка: status=%d, stdout=%q, stderr=%q", status, stdout, stderr)
		}
		assertFileBytes(t, target, original)
		if link, err := os.Readlink(path); err != nil || link != target {
			t.Fatalf("ссылка изменена: %q, %v", link, err)
		}
	})
	t.Run("directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "result.txt")
		if err := os.Mkdir(path, 0755); err != nil {
			t.Fatal(err)
		}
		status, stdout, stderr := runCLI([]string{"hello", "--output=" + path}, os.DirFS("../.."))
		if status != 1 || stdout != "" || stderr == "" {
			t.Fatalf("каталог: status=%d, stdout=%q, stderr=%q", status, stdout, stderr)
		}
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			t.Fatalf("каталог изменён: info=%v, err=%v", info, err)
		}
	})
}

func TestRunRejectsUnavailableOrMalformedBanners(t *testing.T) {
	tests := []struct {
		name    string
		banners fs.FS
	}{
		{"nil filesystem", nil},
		{"nil map", fstest.MapFS(nil)},
		{"missing banner", fstest.MapFS{}},
		{"empty banner", fstest.MapFS{"standard.txt": &fstest.MapFile{}}},
		{"invalid banner", fstest.MapFS{"standard.txt": &fstest.MapFile{Data: []byte("broken\n")}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "result.txt")
			original := []byte("сохранить\n")
			if err := os.WriteFile(path, original, 0644); err != nil {
				t.Fatal(err)
			}
			status, stdout, stderr := runCLI([]string{"hello", "--output=" + path}, test.banners)
			if status != 1 || stdout != "" || stderr == "" {
				t.Fatalf("повреждённый шрифт: status=%d, stdout=%q, stderr=%q", status, stdout, stderr)
			}
			assertFileBytes(t, path, original)
		})
	}
}

func TestRunReturnsFailureWhenStdoutCannotBeWritten(t *testing.T) {
	for _, test := range []struct {
		name string
		out  io.Writer
	}{
		{"write error", brokenOutput{err: errors.New("вывод недоступен")}},
		{"short write", brokenOutput{}},
		{"nil output", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, args := range [][]string{{"hello"}, nil} {
				var stderr bytes.Buffer
				status := Run(args, os.DirFS("../.."), test.out, &stderr)
				if status != 1 || (test.out != nil && stderr.Len() == 0) {
					t.Fatalf("ошибка вывода: args=%q, status=%d, stderr=%q", args, status, stderr.String())
				}
			}
		})
	}
}

func TestRunReturnsFailureForNilStderr(t *testing.T) {
	if status := Run([]string{"hello"}, os.DirFS("../.."), io.Discard, nil); status != 1 {
		t.Fatalf("пустой stderr: status=%d; ожидается 1", status)
	}
}

type brokenOutput struct{ err error }

func (out brokenOutput) Write(_ []byte) (int, error) { return 0, out.err }

func runCLI(args []string, banners fs.FS) (int, string, string) {
	var stdout, stderr bytes.Buffer
	status := Run(args, banners, &stdout, &stderr)
	return status, stdout.String(), stderr.String()
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func assertFileBytes(t *testing.T, path string, expected []byte) {
	t.Helper()
	if actual := readFile(t, path); !bytes.Equal(actual, expected) {
		t.Fatalf("файл %s изменён: %q; ожидается %q", path, actual, expected)
	}
}

func assertHelloGeometry(t *testing.T, content []byte) {
	t.Helper()
	if len(content) != 256 {
		t.Fatalf("размер hello: %d байтов; ожидается 256", len(content))
	}
	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	if len(lines) != 8 {
		t.Fatalf("количество строк hello: %d; ожидается 8", len(lines))
	}
	for index, line := range lines {
		if len(line) != 31 {
			t.Fatalf("ширина строки %d: %d; ожидается 31", index+1, len(line))
		}
	}
}
