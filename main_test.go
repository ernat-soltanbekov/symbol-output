package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"symbol-output/internal/cli"
	"symbol-output/internal/generator"
)

func TestEmbeddedBannersMatchOfficialSources(t *testing.T) {
	for name, want := range map[string]string{
		"standard":   "e194f1033442617ab8a78e1ca63a2061f5cc07a3f05ac226ed32eb9dfd22a6bf",
		"shadow":     "26b94d0b134b77e9fd23e0360bfd81740f80fb7f6541d1d8c5d85e73ee550f73",
		"thinkertoy": "64285e4960d199f4819323c4dc6319ba34f1f0dd9da14d07111345f5d76c3fa3",
	} {
		data, err := bannerFiles.ReadFile(name + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
			t.Errorf("исходные байты %s изменились: %s", name, got)
		}
	}
}

func TestCommittedExamplesAreReproducible(t *testing.T) {
	for _, sample := range []struct{ file, input, banner string }{
		{"test00.txt", `First\nTest`, "shadow"},
		{"test01.txt", "hello", "standard"},
		{"test02.txt", "123 -> #$%", "standard"},
		{"test03.txt", "Hello 01!", "thinkertoy"},
	} {
		data, err := bannerFiles.ReadFile(sample.banner + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		banner, err := generator.Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		art, err := banner.Generate(sample.input)
		if err != nil {
			t.Fatal(err)
		}
		committed, err := os.ReadFile(sample.file)
		if err != nil {
			t.Fatal(err)
		}
		if string(committed) != art.Text {
			t.Errorf("пример %s не соответствует входным данным", sample.file)
		}
	}
}

// Проверяем настоящий процесс: main передаёт правильные коды завершения,
// а встроенные шрифты работают без исходных файлов в рабочем каталоге.
func TestCLIProcessOutsideRepository(t *testing.T) {
	directory := t.TempDir()
	binary := filepath.Join(directory, "symbol-output")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("сборка: %v\n%s", err, output)
	}
	for _, test := range []struct {
		name   string
		args   []string
		status int
		want   string
	}{
		{"stdout", []string{"hello"}, 0, ""},
		{"usage", []string{"--output", "result.txt", "hello", "standard"}, 2, cli.Usage},
		{"verify", []string{"--output=result.txt", "hello", "--verify"}, 0, "Integrity Score: 5/5 (Valid)"},
		{"tamper", []string{"--output=result.txt", "hello", "--tamper-check"}, 0, "unmodified"},
		{"unsupported", []string{"Привет"}, 1, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := exec.Command(binary, test.args...)
			command.Dir = directory
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			err := command.Run()
			status := 0
			if err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					status = exit.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if status != test.status {
				t.Fatalf("код %d вместо %d: %s", status, test.status, stderr.String())
			}
			if test.name == "usage" && stdout.String() != test.want {
				t.Fatalf("Usage не совпадает: %q", stdout.String())
			}
			if test.want != "" && !strings.Contains(stdout.String(), test.want) {
				t.Fatalf("нет %q: %q", test.want, stdout.String())
			}
			if test.name == "stdout" {
				fixture, err := os.ReadFile("test01.txt")
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(stdout.Bytes(), fixture) || stderr.Len() != 0 {
					t.Fatal("вывод процесса не совпал с hello")
				}
			}
		})
	}
}
