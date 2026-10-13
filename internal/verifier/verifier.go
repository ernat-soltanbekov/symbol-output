// Package verifier повторно читает результат с диска и проверяет его устройство.
package verifier

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// MaxFileSize ограничивает чтение: повреждённый файл не должен занять всю память.
const MaxFileSize int64 = 64 << 20

// MaxFileLines ограничивает память для списка строк, даже если каждая пуста.
const MaxFileLines = 1 << 20

const linesPerRow = 8

// Check хранит результат одной независимой проверки и понятное пояснение.
type Check struct {
	Name   string
	Passed bool
	Detail string
}

// Report содержит пять проверок, количество успехов и итог из таблицы задания.
type Report struct {
	Checks  []Check
	Score   int
	Verdict string
}

// Verify проверяет файл на диске. widths содержит ширину каждой строки текста,
// рассчитанную генератором по символам баннера, а не по проверяемому файлу.
func Verify(path string, widths []int) Report {
	report := Report{Checks: []Check{
		{Name: "File is readable"},
		{Name: "Line count"},
		{Name: "Width consistent within each row"},
		{Name: "No padding beyond row width"},
		{Name: "File size matches"},
	}}

	data, err := readRegularFile(path)
	if err != nil {
		report.Checks[0].Detail = "(" + err.Error() + ")"
		for i := 1; i < len(report.Checks); i++ {
			report.Checks[i].Detail = "(file could not be read)"
		}
		return scoreReport(report)
	}
	report.Checks[0].Passed = true

	lines := splitLines(data)
	expectedLines := linesPerRow * len(widths)
	report.Checks[1].Passed = len(lines) == expectedLines
	rowWord := "rows"
	if len(widths) == 1 {
		rowWord = "row"
	}
	report.Checks[1].Detail = fmt.Sprintf("is %d (8 per row x %d %s)", len(lines), len(widths), rowWord)
	if !report.Checks[1].Passed {
		report.Checks[1].Detail += fmt.Sprintf("; expected %d", expectedLines)
	}

	// Равенство ширин — самостоятельная проверка. Все восемь строк могут
	// ошибочно стать длиннее на один пробел и всё ещё быть одинаковыми.
	consistent := len(lines) == expectedLines
	actualWidths := make([]string, 0, len(widths))
	if consistent {
		for row := range widths {
			start := row * linesPerRow
			width := len(lines[start])
			actualWidths = append(actualWidths, strconv.Itoa(width))
			for line := start + 1; line < start+linesPerRow; line++ {
				if len(lines[line]) != width {
					consistent = false
				}
			}
		}
	}
	report.Checks[2].Passed = consistent
	switch {
	case consistent && len(widths) == 0:
		report.Checks[2].Detail = "(empty output)"
	case consistent:
		report.Checks[2].Detail = "(" + strings.Join(actualWidths, ", ") + ")"
	case len(lines) != expectedLines:
		report.Checks[2].Detail = "(incomplete or extra row blocks)"
	default:
		report.Checks[2].Detail = "(lines in a row have different widths)"
	}

	// Конечные пробелы рисунка законны: проверяем ровно ширину из баннера,
	// не обрезая их. Это ловит и лишние столбцы, и удалённое заполнение.
	padding := len(lines) == expectedLines
	for i, line := range lines {
		if i/linesPerRow >= len(widths) || len(line) != widths[i/linesPerRow] {
			padding = false
		}
		for _, char := range line {
			if char < ' ' || char > '~' {
				padding = false
			}
		}
	}
	report.Checks[3].Passed = padding
	if !padding {
		report.Checks[3].Detail = "(unexpected columns or non-printable ASCII)"
	}

	// Размер считается по повторно прочитанным байтам. Ровно один '\n'
	// требуется после каждой строки, включая последнюю; пустой файл имеет 0 байт.
	expectedSize := 0
	for _, line := range lines {
		expectedSize += len(line) + 1
	}
	report.Checks[4].Passed = len(data) == expectedSize
	if report.Checks[4].Passed {
		report.Checks[4].Detail = fmt.Sprintf("(%d bytes)", len(data))
	} else {
		report.Checks[4].Detail = fmt.Sprintf("(actual: %d bytes; expected: %d bytes)", len(data), expectedSize)
	}
	return scoreReport(report)
}

func scoreReport(report Report) Report {
	report.Score = 0
	for _, check := range report.Checks {
		if check.Passed {
			report.Score++
		}
	}
	switch report.Score {
	case 5:
		report.Verdict = "Valid"
	case 4:
		report.Verdict = "Acceptable"
	default:
		report.Verdict = "Corrupted"
	}
	return report
}

// RenderReport печатает отчёт в формате задания и возвращает ошибку вывода.
func RenderReport(out io.Writer, report Report) error {
	var text strings.Builder
	fmt.Fprintf(&text, "\n--- Verification Report ---\nIntegrity Score: %d/5 (%s)\n\nChecks:\n", report.Score, report.Verdict)
	for _, check := range report.Checks {
		status := "FAIL"
		if check.Passed {
			status = "PASS"
		}
		fmt.Fprintf(&text, "  [%s] %s", status, check.Name)
		if check.Detail != "" {
			fmt.Fprintf(&text, " %s", check.Detail)
		}
		text.WriteByte('\n')
	}
	return writeText(out, text.String())
}

// TamperResult содержит номера изменённых строк, начиная с единицы.
type TamperResult struct {
	Lines      []int
	Unmodified bool
}

// Tamper сравнивает существующий файл с заново созданным рисунком в памяти.
// Она ничего не записывает: перезапись до сравнения скрыла бы повреждение.
func Tamper(path string, fresh string) (TamperResult, error) {
	data, err := readRegularFile(path)
	if err != nil {
		return TamperResult{}, err
	}
	actual := splitComparisonLines(data)
	expected := splitComparisonLines([]byte(fresh))
	count := len(actual)
	if len(expected) > count {
		count = len(expected)
	}
	result := TamperResult{}
	for i := 0; i < count; i++ {
		if i >= len(actual) || i >= len(expected) || !bytes.Equal(actual[i], expected[i]) {
			result.Lines = append(result.Lines, i+1)
		}
	}
	result.Unmodified = len(result.Lines) == 0
	return result, nil
}

// RenderTamper печатает номера изменённых строк в формате бонусного задания.
func RenderTamper(out io.Writer, result TamperResult) error {
	var text strings.Builder
	text.WriteString("\n--- Tamper Check ---\n")
	if len(result.Lines) == 0 {
		text.WriteString("Result: file matches a fresh regeneration (unmodified)\n")
	} else {
		if len(result.Lines) == 1 {
			text.WriteString("Result: 1 line differs from a fresh regeneration\n")
		} else {
			fmt.Fprintf(&text, "Result: %d lines differ from a fresh regeneration\n", len(result.Lines))
		}
		for _, line := range result.Lines {
			fmt.Fprintf(&text, "  Line %d differs\n", line)
		}
	}
	return writeText(out, text.String())
}

func readRegularFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect file: %w", err)
	}
	// Ссылка, каталог, устройство и FIFO не являются рисунком. Читаем только
	// обычный файл, который writer также согласился бы записать. Чтение
	// FIFO способно ждать бесконечно, поэтому отказываем до os.ReadFile.
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("file is not a regular file")
	}
	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("file exceeds %d-byte limit", MaxFileSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	// Файл мог вырасти между Lstat и ReadFile: результат тоже проверяется.
	if int64(len(data)) > MaxFileSize {
		return nil, fmt.Errorf("file exceeds %d-byte limit", MaxFileSize)
	}
	// Миллионы '\n' занимают мало байт, но bytes.Split создаёт отдельный
	// элемент списка для каждой строки. Считаем строки до этого выделения.
	lineCount := bytes.Count(data, []byte{'\n'})
	if len(data) > 0 && data[len(data)-1] != '\n' {
		lineCount++
	}
	if lineCount > MaxFileLines {
		return nil, fmt.Errorf("file exceeds %d-line limit", MaxFileLines)
	}
	return data, nil
}

// Удаляем только один завершающий разделитель. Второй '\n' — настоящая
// пустая строка, которую проверка количества строк обязана заметить.
func splitLines(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}
	lines := bytes.Split(data, []byte{'\n'})
	if data[len(data)-1] == '\n' {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// В сравнении сохраняем '\n' внутри строки: его удаление меняет последнюю
// строку, даже когда все видимые символы остались прежними.
func splitComparisonLines(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}
	lines := bytes.SplitAfter(data, []byte{'\n'})
	if data[len(data)-1] == '\n' {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func writeText(out io.Writer, text string) error {
	if out == nil {
		return fmt.Errorf("output writer is nil")
	}
	n, err := io.WriteString(out, text)
	if err == nil && n != len(text) {
		return io.ErrShortWrite
	}
	return err
}
