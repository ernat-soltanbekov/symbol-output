// Package writer сохраняет готовый рисунок в файл целиком.
package writer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maxOutputBytes = 64 * 1024 * 1024

// Write атомарно сохраняет content: читатель видит старый файл или полностью
// записанный новый файл. Каталоги автоматически не создаются. Новый файл имеет
// права 0644; у существующего сохраняются обычные биты прав доступа.
// Символические ссылки, другие виды файлов и файлы без прав записи отклоняются.
func Write(path string, content string) (resultErr error) {
	if path == "" {
		return errors.New("путь выходного файла не может быть пустым")
	}
	if len(content) > maxOutputBytes {
		return fmt.Errorf("выходной файл превышает лимит %d МиБ", maxOutputBytes/(1024*1024))
	}

	mode, err := outputMode(path)
	if err != nil {
		return err
	}

	// Временный файл находится рядом с целевым: rename остаётся атомарным
	// и не переносит данные между разными файловыми системами.
	temporary, err := os.CreateTemp(filepath.Dir(path), ".symbol-output-*")
	if err != nil {
		return fmt.Errorf("создать временный выходной файл: %w", err)
	}
	temporaryPath := temporary.Name()
	closed := false
	defer func() {
		if !closed {
			if err := temporary.Close(); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("закрыть временный файл: %w", err))
			}
		}
		if err := os.Remove(temporaryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, fmt.Errorf("удалить временный файл: %w", err))
		}
	}()

	written, err := io.WriteString(temporary, content)
	if err != nil {
		return fmt.Errorf("записать выходной файл: %w", err)
	}
	if written != len(content) {
		return fmt.Errorf("записать выходной файл: %w", io.ErrShortWrite)
	}
	if err := temporary.Chmod(mode); err != nil {
		return fmt.Errorf("установить права выходного файла: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("сохранить выходной файл на диске: %w", err)
	}
	err = temporary.Close()
	closed = true
	if err != nil {
		return fmt.Errorf("закрыть выходной файл: %w", err)
	}

	// Повторная проверка замечает опасную замену цели во время записи.
	if _, err := outputMode(path); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("заменить выходной файл: %w", err)
	}
	return nil
}

// outputMode проверяет сам путь, не переходя по символической ссылке.
func outputMode(path string) (os.FileMode, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0644, nil
	}
	if err != nil {
		return 0, fmt.Errorf("проверить выходной файл: %w", err)
	}
	if !info.Mode().IsRegular() {
		return 0, errors.New("выходной путь должен указывать на обычный файл, а не ссылку или каталог")
	}
	if info.Mode().Perm()&0222 == 0 {
		return 0, fmt.Errorf("выходной файл защищён от записи: %w", os.ErrPermission)
	}
	return info.Mode().Perm(), nil
}
