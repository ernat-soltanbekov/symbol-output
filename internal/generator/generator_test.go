package generator

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// Образец hello взят из ТЗ, а не получен проверяемой функцией.
const hello = " _              _   _          \n" +
	"| |            | | | |         \n" +
	"| |__     ___  | | | |   ___   \n" +
	"|  _ \\   / _ \\ | | | |  / _ \\  \n" +
	"| | | | |  __/ | | | | | (_) | \n" +
	"|_| |_|  \\___| |_| |_|  \\___/  \n" +
	"                               \n" +
	"                               \n"

func loadBanner(t testing.TB, name string) *Banner {
	t.Helper()
	data, err := os.ReadFile("../../" + name + ".txt")
	if err != nil {
		t.Fatal(err)
	}
	banner, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return banner
}

func TestHelloMatchesSubject(t *testing.T) {
	art, err := loadBanner(t, "standard").Generate("hello")
	if err != nil {
		t.Fatal(err)
	}
	if art.Text != hello || len(art.Text) != 256 || len(art.Widths) != 1 || art.Widths[0] != 31 {
		t.Fatalf("hello должен совпадать с ТЗ и занимать 256 байт: %q, widths=%v", art.Text, art.Widths)
	}
}

func TestAllPrintableCharactersAndBanners(t *testing.T) {
	for _, name := range []string{"standard", "shadow", "thinkertoy"} {
		t.Run(name, func(t *testing.T) {
			banner := loadBanner(t, name)
			var input strings.Builder
			for value := byte(32); value <= 126; value++ {
				input.WriteByte(value)
			}
			art, err := banner.Generate(input.String())
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSuffix(art.Text, "\n"), "\n")
			if len(lines) != Height || len(art.Widths) != 1 {
				t.Fatalf("неверная геометрия: %d lines %v", len(lines), art.Widths)
			}
			for _, line := range lines {
				if len(line) != art.Widths[0] {
					t.Fatal("неодинаковая ширина")
				}
			}
		})
	}
}

func TestNewlines(t *testing.T) {
	banner := loadBanner(t, "standard")
	for _, test := range []struct {
		input string
		rows  int
	}{
		{"", 0}, {"hello", 1}, {`First\nTest`, 2}, {"First\nTest", 2},
		{`\n`, 2}, {"\n", 2}, {"A\n", 2}, {"\nA", 2}, {"A\n\nB", 3}, {" ", 1},
	} {
		art, err := banner.Generate(test.input)
		if err != nil {
			t.Fatalf("%q: %v", test.input, err)
		}
		if len(art.Widths) != test.rows || strings.Count(art.Text, "\n") != Height*test.rows {
			t.Errorf("%q: widths=%v, lines=%d", test.input, art.Widths, strings.Count(art.Text, "\n"))
		}
	}
	literal, _ := banner.Generate(`First\nTest`)
	actual, _ := banner.Generate("First\nTest")
	if literal.Text != actual.Text {
		t.Fatal("буквальный и настоящий перенос должны совпадать")
	}
}

func TestRejectInvalidAndOversizedInput(t *testing.T) {
	banner := loadBanner(t, "standard")
	for _, input := range []string{"Привет", "🙂", "a\t", "a\r\n", "\x00", "\x7f", string([]byte{0xff}), strings.Repeat("a", MaxInputBytes+1), strings.Repeat("W", MaxInputBytes), strings.Repeat("\n", MaxTextRows)} {
		if _, err := banner.Generate(input); err == nil {
			t.Errorf("не отклонён недопустимый ввод длины %d", len(input))
		}
	}
	if _, err := (*Banner)(nil).Generate("hello"); err == nil {
		t.Fatal("nil-шрифт должен возвращать ошибку")
	}
}

func TestParseRejectsCorruption(t *testing.T) {
	data, err := os.ReadFile("../../standard.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, broken := range [][]byte{
		nil, data[:len(data)-1], append(append([]byte(nil), data...), '\n'),
		bytes.Replace(data, []byte("\n"), []byte("X\n"), 1),
		bytes.Replace(data, []byte("   "), []byte("\t  "), 1),
		bytes.Replace(data, []byte("   \n"), []byte("  \n"), 1),
	} {
		if _, err := Parse(broken); err == nil {
			t.Fatal("повреждённый шрифт принят")
		}
	}
	if _, err := Parse(bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))); err != nil {
		t.Fatalf("CRLF-шрифт должен поддерживаться: %v", err)
	}
}

func FuzzGenerate(f *testing.F) {
	banner := loadBanner(f, "standard")
	for _, seed := range []string{"hello", `A\nB`, "", "\n", "#$% -> 123", "\xff"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		art, err := banner.Generate(input)
		if err != nil {
			return
		}
		if len(art.Text) > MaxOutputBytes || strings.Count(art.Text, "\n") != Height*len(art.Widths) {
			t.Fatal("нарушены ограничения результата")
		}
		if art.Text == "" {
			return
		}
		lines := strings.Split(strings.TrimSuffix(art.Text, "\n"), "\n")
		for i, line := range lines {
			if len(line) != art.Widths[i/Height] {
				t.Fatal("нарушена ширина строки")
			}
		}
	})
}
