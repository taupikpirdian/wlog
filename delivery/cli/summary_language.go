package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	application "github.com/taupikpirdian/wlog/application/summary"
)

func selectOutputLanguage(input *bufio.Reader, out io.Writer) (application.OutputLanguage, error) {
	if _, err := fmt.Fprintln(out, "\nSelect output language:\n  1. Bahasa Indonesia\n  2. English"); err != nil {
		return "", err
	}
	for {
		if _, err := fmt.Fprint(out, "Choose language [1-2] (default 1), or q to cancel: "); err != nil {
			return "", err
		}
		value, err := readInputLine(input)
		if err != nil {
			return "", err
		}
		switch strings.ToLower(value) {
		case "", "1", "id":
			return application.LanguageIndonesian, nil
		case "2", "en":
			return application.LanguageEnglish, nil
		case "q":
			return "", errors.New("language selection canceled")
		}
		if _, err := fmt.Fprintln(out, "Enter 1 for Bahasa Indonesia or 2 for English."); err != nil {
			return "", err
		}
	}
}
