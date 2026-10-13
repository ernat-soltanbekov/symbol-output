// Package generator превращает печатные ASCII-символы в рисунок высотой 8 строк.
package generator

import (
	"bytes"
	"fmt"
	"strings"
)

const (
	Height = 8
	// Ограничения проверяются до выделения памяти под рисунок.
	MaxInputBytes  = 1 << 20
	MaxOutputBytes = 64 << 20
	MaxTextRows    = 10_000
	glyphCount     = 95
)

// Banner хранит восемь строк рисунка для каждого символа от пробела до '~'.
type Banner struct {
	glyphs [glyphCount][Height]string
	widths [glyphCount]int
}

// Art содержит рисунок и ожидаемую ширину каждой текстовой строки.
// Widths описывает геометрию, а проверяющий модуль читает сами строки с диска.
type Art struct {
	Text   string
	Widths []int
}

// Parse проверяет таблицу шрифта. Пробелы являются частью рисунка и сохраняются.
func Parse(data []byte) (*Banner, error) {
	if len(data) > MaxInputBytes {
		return nil, fmt.Errorf("таблица шрифта превышает 1 МиБ")
	}
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return nil, fmt.Errorf("таблица шрифта должна заканчиваться переносом строки")
	}
	lines := strings.Split(string(data[:len(data)-1]), "\n")
	if len(lines) != glyphCount*(Height+1) {
		return nil, fmt.Errorf("в таблице шрифта должно быть 855 строк, получено %d", len(lines))
	}
	banner := &Banner{}
	for i := 0; i < glyphCount; i++ {
		start := i * (Height + 1)
		if lines[start] != "" {
			return nil, fmt.Errorf("перед символом %q должен быть пустой разделитель", byte(i+32))
		}
		width := len(lines[start+1])
		if width == 0 {
			return nil, fmt.Errorf("нулевая ширина символа %q", byte(i+32))
		}
		for row := 0; row < Height; row++ {
			line := lines[start+1+row]
			if len(line) != width {
				return nil, fmt.Errorf("неодинаковая ширина строк символа %q", byte(i+32))
			}
			for _, value := range []byte(line) {
				if value < 32 || value > 126 {
					return nil, fmt.Errorf("непечатный ASCII-символ в таблице шрифта")
				}
			}
			banner.glyphs[i][row] = line
		}
		banner.widths[i] = width
	}
	return banner, nil
}

// Generate воспринимает как перенос и настоящий LF, и буквальную пару '\n'.
// Каждая текстовая строка, включая пустую, занимает ровно восемь строк рисунка.
// Полностью пустой ввод создаёт пустой рисунок без текстовых строк.
func (banner *Banner) Generate(input string) (Art, error) {
	if banner == nil {
		return Art{}, fmt.Errorf("шрифт не загружен")
	}
	if len(input) > MaxInputBytes {
		return Art{}, fmt.Errorf("ввод превышает 1 МиБ")
	}
	input = strings.ReplaceAll(input, `\n`, "\n")
	if input == "" {
		return Art{}, nil
	}
	for index, value := range []byte(input) {
		if value != '\n' && (value < 32 || value > 126) {
			return Art{}, fmt.Errorf("неподдерживаемый символ в байте %d: нужны ASCII 32–126 или перенос строки", index+1)
		}
	}
	if strings.Count(input, "\n")+1 > MaxTextRows {
		return Art{}, fmt.Errorf("ввод превышает %d текстовых строк", MaxTextRows)
	}
	rows := strings.Split(input, "\n")
	widths := make([]int, len(rows))
	size := 0
	for i, row := range rows {
		for _, value := range []byte(row) {
			widths[i] += banner.widths[value-32]
		}
		rowSize := Height * (widths[i] + 1)
		if rowSize > MaxOutputBytes-size {
			return Art{}, fmt.Errorf("рисунок превышает 64 МиБ; сократите ввод")
		}
		size += rowSize
	}
	var result strings.Builder
	result.Grow(size)
	for _, row := range rows {
		for artLine := 0; artLine < Height; artLine++ {
			for _, value := range []byte(row) {
				result.WriteString(banner.glyphs[value-32][artLine])
			}
			result.WriteByte('\n')
		}
	}
	return Art{Text: result.String(), Widths: widths}, nil
}
