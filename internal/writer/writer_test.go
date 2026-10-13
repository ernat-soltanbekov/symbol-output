package writer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestWriteCreatesAndReplacesWholeFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "result.txt")
	for _, content := range []string{"первая строка\nвторая строка\n", "короче\n", ""} {
		if err := Write(path, content); err != nil {
			t.Fatalf("Write: %v", err)
		}
		assertContent(t, path, content)
		assertDirectoryEntries(t, directory, "result.txt")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0644 {
		t.Fatalf("права нового файла: %04o; нужны 0644", info.Mode().Perm())
	}
}

func TestWritePreservesExistingPermissions(t *testing.T) {
	for _, mode := range []os.FileMode{0600, 0640, 0750} {
		t.Run(mode.String(), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "result.txt")
			if err := os.WriteFile(path, []byte("старое содержимое"), mode); err != nil {
				t.Fatal(err)
			}
			// Chmod задаёт точные биты независимо от umask процесса.
			if err := os.Chmod(path, mode); err != nil {
				t.Fatal(err)
			}
			if err := Write(path, "новое содержимое"); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != mode {
				t.Fatalf("права изменились: %04o вместо %04o", info.Mode().Perm(), mode)
			}
			assertContent(t, path, "новое содержимое")
		})
	}
}

func TestWriteRejectsUnsafeTargetsWithoutChangingThem(t *testing.T) {
	t.Run("каталог", func(t *testing.T) {
		directory := t.TempDir()
		path := filepath.Join(directory, "result.txt")
		if err := os.Mkdir(path, 0755); err != nil {
			t.Fatal(err)
		}
		if err := Write(path, "данные"); err == nil {
			t.Fatal("запись поверх каталога должна завершаться ошибкой")
		}
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			t.Fatalf("каталог изменён: info=%v, err=%v", info, err)
		}
		assertDirectoryEntries(t, directory, "result.txt")
	})
	t.Run("символическая ссылка", func(t *testing.T) {
		directory := t.TempDir()
		target := filepath.Join(directory, "original.txt")
		path := filepath.Join(directory, "result.txt")
		if err := os.WriteFile(target, []byte("сохранить"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Skipf("символические ссылки недоступны: %v", err)
		}
		if err := Write(path, "данные"); err == nil {
			t.Fatal("символическая ссылка должна отклоняться")
		}
		assertContent(t, target, "сохранить")
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("ссылка изменена: info=%v, err=%v", info, err)
		}
		assertDirectoryEntries(t, directory, "original.txt", "result.txt")
	})
	t.Run("оборванная символическая ссылка", func(t *testing.T) {
		directory := t.TempDir()
		path := filepath.Join(directory, "result.txt")
		if err := os.Symlink(filepath.Join(directory, "missing.txt"), path); err != nil {
			t.Skipf("символические ссылки недоступны: %v", err)
		}
		if err := Write(path, "данные"); err == nil {
			t.Fatal("оборванная ссылка должна отклоняться")
		}
		assertDirectoryEntries(t, directory, "result.txt")
	})
	t.Run("файл без прав записи", func(t *testing.T) {
		directory := t.TempDir()
		path := filepath.Join(directory, "result.txt")
		if err := os.WriteFile(path, []byte("сохранить"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0444); err != nil {
			t.Fatal(err)
		}
		if err := Write(path, "данные"); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("нужна ошибка доступа; получено %v", err)
		}
		assertContent(t, path, "сохранить")
		assertDirectoryEntries(t, directory, "result.txt")
	})
}

func TestWriteRejectsInvalidPaths(t *testing.T) {
	if err := Write("", "данные"); err == nil {
		t.Fatal("пустой путь должен отклоняться")
	}
	directory := t.TempDir()
	path := filepath.Join(directory, "missing", "result.txt")
	if err := Write(path, "данные"); err == nil {
		t.Fatal("отсутствующий родительский каталог должен приводить к ошибке")
	}
	assertDirectoryEntries(t, directory)
}

func TestWriteRejectsOversizedContentWithoutChangingFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "result.txt")
	if err := os.WriteFile(path, []byte("сохранить"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, strings.Repeat("x", maxOutputBytes+1)); err == nil {
		t.Fatal("файл больше лимита должен отклоняться")
	}
	assertContent(t, path, "сохранить")
	assertDirectoryEntries(t, directory, "result.txt")
}

func TestConcurrentWritesProduceOneCompletePayload(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "result.txt")
	const count = 12
	payloads := make(map[string]bool, count)
	errorsFound := make(chan error, count)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for index := 0; index < count; index++ {
		content := strings.Repeat(string(rune('A'+index)), 256*1024) + "\n"
		payloads[content] = true
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			errorsFound <- Write(path, content)
		}()
	}
	close(start)
	workers.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatalf("параллельная запись: %v", err)
		}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !payloads[string(content)] {
		t.Fatal("итоговый файл содержит смесь данных разных записей")
	}
	assertDirectoryEntries(t, directory, "result.txt")
}

func TestWriteFailureInReadOnlyDirectoryPreservesOriginal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root может обходить права каталога")
	}
	directory := t.TempDir()
	path := filepath.Join(directory, "result.txt")
	if err := os.WriteFile(path, []byte("сохранить"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(directory, 0755); err != nil {
			t.Errorf("восстановить права каталога: %v", err)
		}
	})
	if err := Write(path, "данные"); err == nil {
		t.Fatal("запись в защищённый каталог должна завершаться ошибкой")
	}
	assertContent(t, path, "сохранить")
	assertDirectoryEntries(t, directory, "result.txt")
}

func assertContent(t *testing.T, path string, expected string) {
	t.Helper()
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != expected {
		t.Fatalf("содержимое %s: %q; ожидается %q", path, actual, expected)
	}
}

func assertDirectoryEntries(t *testing.T, directory string, expected ...string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(expected) {
		t.Fatalf("в каталоге осталось %d элементов вместо %d: %v", len(entries), len(expected), entries)
	}
	for index, entry := range entries {
		if entry.Name() != expected[index] {
			t.Fatalf("неожиданный элемент каталога: %s", entry.Name())
		}
	}
}
