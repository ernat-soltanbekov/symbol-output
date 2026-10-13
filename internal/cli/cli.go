// Package cli связывает аргументы командной строки, рисование и работу с файлом.
package cli

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"symbol-output/internal/generator"
	"symbol-output/internal/verifier"
	"symbol-output/internal/writer"
)

const Usage = "Usage: go run . [OPTION] [STRING] [BANNER]\nEX: go run . --output=<fileName.txt> something standard\n"

// Options содержит уже проверенные аргументы; пустая строка ввода допустима.
type Options struct {
	Input       string
	Banner      string
	Output      string
	Verify      bool
	TamperCheck bool
}

// Parse принимает флаги в любом месте, но только в точном формате из ТЗ.
// Разделитель '--' позволяет передать текст, который начинается с дефиса.
func Parse(args []string) (Options, error) {
	options := Options{Banner: "standard"}
	positionals := make([]string, 0, 2)
	parseFlags := true
	for _, arg := range args {
		if parseFlags && arg == "--" {
			parseFlags = false
			continue
		}
		if parseFlags && strings.HasPrefix(arg, "-") {
			switch {
			case strings.HasPrefix(arg, "--output="):
				if options.Output != "" {
					return Options{}, fmt.Errorf("повторный --output")
				}
				options.Output = strings.TrimPrefix(arg, "--output=")
				if options.Output == "" || filepath.Ext(options.Output) != ".txt" || strings.ContainsRune(options.Output, 0) {
					return Options{}, fmt.Errorf("нужен непустой путь с расширением .txt")
				}
			case arg == "--verify":
				if options.Verify {
					return Options{}, fmt.Errorf("повторный --verify")
				}
				options.Verify = true
			case arg == "--tamper-check":
				if options.TamperCheck {
					return Options{}, fmt.Errorf("повторный --tamper-check")
				}
				options.TamperCheck = true
			default:
				return Options{}, fmt.Errorf("неизвестный флаг")
			}
			continue
		}
		positionals = append(positionals, arg)
	}
	if len(positionals) < 1 || len(positionals) > 2 {
		return Options{}, fmt.Errorf("нужна строка и, при желании, название шрифта")
	}
	options.Input = positionals[0]
	if len(positionals) == 2 {
		options.Banner = positionals[1]
	}
	if options.Banner != "standard" && options.Banner != "shadow" && options.Banner != "thinkertoy" {
		return Options{}, fmt.Errorf("неизвестный шрифт")
	}
	if (options.Verify || options.TamperCheck) && options.Output == "" {
		return Options{}, fmt.Errorf("для проверки нужен --output")
	}
	return options, nil
}

// Run возвращает код завершения: 0 — успех, 1 — ошибка или повреждение, 2 — Usage.
func Run(args []string, banners fs.FS, stdout, stderr io.Writer) int {
	if stdout == nil || stderr == nil {
		return 1
	}
	options, err := Parse(args)
	if err != nil {
		if writeErr := writeText(stdout, Usage); writeErr != nil {
			return fail(stderr, writeErr)
		}
		return 2
	}
	if banners == nil {
		return fail(stderr, fmt.Errorf("таблицы шрифтов недоступны"))
	}
	data, err := fs.ReadFile(banners, options.Banner+".txt")
	if err != nil {
		return fail(stderr, fmt.Errorf("не удалось загрузить шрифт: %w", err))
	}
	banner, err := generator.Parse(data)
	if err != nil {
		return fail(stderr, err)
	}
	art, err := banner.Generate(options.Input)
	if err != nil {
		return fail(stderr, err)
	}
	if options.Output == "" {
		if err := writeText(stdout, art.Text); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if options.TamperCheck {
		return checkExisting(options, art, stdout, stderr)
	}
	if protectedOutput(options.Output, banners) {
		return fail(stderr, fmt.Errorf("нельзя перезаписать таблицу шрифта или служебный каталог .git"))
	}
	if err := writer.Write(options.Output, art.Text); err != nil {
		return fail(stderr, err)
	}
	if !options.Verify {
		return 0
	}
	if err := writeText(stdout, fmt.Sprintf("File created: %s\n", options.Output)); err != nil {
		return fail(stderr, err)
	}
	report := verifier.Verify(options.Output, art.Widths)
	if err := verifier.RenderReport(stdout, report); err != nil {
		return fail(stderr, err)
	}
	if report.Score != 5 {
		return 1
	}
	return 0
}

// Проверка изменений никогда не вызывает writer: исходный файл остаётся уликой.
func checkExisting(options Options, art generator.Art, stdout, stderr io.Writer) int {
	tamper, err := verifier.Tamper(options.Output, art.Text)
	if err != nil {
		return fail(stderr, err)
	}
	status := 0
	if options.Verify {
		report := verifier.Verify(options.Output, art.Widths)
		if err := verifier.RenderReport(stdout, report); err != nil {
			return fail(stderr, err)
		}
		if report.Score != 5 {
			status = 1
		}
	}
	if err := verifier.RenderTamper(stdout, tamper); err != nil {
		return fail(stderr, err)
	}
	if len(tamper.Lines) != 0 {
		status = 1
	}
	return status
}

func protectedOutput(path string, banners fs.FS) bool {
	for _, part := range strings.Split(filepath.ToSlash(filepath.Clean(path)), "/") {
		if strings.EqualFold(part, ".git") {
			return true
		}
	}
	name := strings.ToLower(filepath.Base(path))
	if name != "standard.txt" && name != "shadow.txt" && name != "thinkertoy.txt" {
		return false
	}
	// Имя шрифта допустимо для обычного результата в другом каталоге.
	// Защищаем существующую таблицу, узнавая её по имени и исходным байтам.
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > generator.MaxInputBytes {
		return false
	}
	table, err := fs.ReadFile(banners, name)
	if err != nil {
		return false
	}
	current, err := os.ReadFile(path)
	return err == nil && bytes.Equal(current, table)
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "Ошибка: %v\n", err)
	return 1
}

func writeText(out io.Writer, text string) error {
	n, err := io.WriteString(out, text)
	if err == nil && n != len(text) {
		return io.ErrShortWrite
	}
	return err
}
