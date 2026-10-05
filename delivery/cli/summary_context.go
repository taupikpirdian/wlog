package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const maxAdditionalContextBytes = 64 * 1024

func readAdditionalContext(input *bufio.Reader, out io.Writer) (string, error) {
	if _, err := fmt.Fprint(out, "\nAdditional context for this summary (optional, not saved to the database):\nPaste text across multiple lines, or enter @/path/to/context.md to read a text file.\nFor pasted text, enter a single . to finish. Press Enter to skip, or q to cancel before entering text.\n> "); err != nil {
		return "", err
	}
	var b strings.Builder
	for {
		line, err := input.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if b.Len() == 0 && strings.EqualFold(strings.TrimSpace(line), "q") {
			return "", errors.New("summary canceled")
		}
		if b.Len() == 0 && strings.HasPrefix(strings.TrimSpace(line), "@") {
			return readAdditionalContextFile(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "@")))
		}
		if strings.TrimSpace(line) == "." || (b.Len() == 0 && strings.TrimSpace(line) == "") {
			return strings.TrimSpace(b.String()), nil
		}
		if b.Len()+len(line)+1 > maxAdditionalContextBytes {
			return "", errors.New("additional context exceeds 64 KiB; shorten it and retry")
		}
		b.WriteString(line)
		b.WriteByte('\n')
		if err == io.EOF {
			return strings.TrimSpace(b.String()), nil
		}
	}
}

func readAdditionalContextFile(path string) (string, error) {
	if len(path) >= 2 && ((path[0] == '"' && path[len(path)-1] == '"') || (path[0] == '\'' && path[len(path)-1] == '\'')) {
		path = path[1 : len(path)-1]
	}
	if path == "" {
		return "", errors.New("enter a text file path after @")
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		homeDirectory, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve additional context file: %w", err)
		}
		path = filepath.Join(homeDirectory, path[2:])
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("read additional context file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("additional context file must be a regular text file")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read additional context file: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxAdditionalContextBytes+1))
	if err != nil {
		return "", fmt.Errorf("read additional context file: %w", err)
	}
	if len(data) > maxAdditionalContextBytes {
		return "", errors.New("additional context file exceeds 64 KiB; shorten it and retry")
	}
	if !utf8.Valid(data) || strings.ContainsRune(string(data), '\x00') {
		return "", errors.New("additional context file must contain UTF-8 text; use a .txt or .md export")
	}
	return strings.TrimSpace(strings.TrimPrefix(string(data), "\uFEFF")), nil
}
