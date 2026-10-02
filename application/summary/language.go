package summary

import "fmt"

type OutputLanguage string

const (
	LanguageIndonesian OutputLanguage = "id"
	LanguageEnglish    OutputLanguage = "en"
)

// ResolveOutputLanguage preserves Indonesian as the default for existing callers.
func ResolveOutputLanguage(language OutputLanguage) (OutputLanguage, error) {
	switch language {
	case "", LanguageIndonesian:
		return LanguageIndonesian, nil
	case LanguageEnglish:
		return LanguageEnglish, nil
	default:
		return "", fmt.Errorf("unsupported output language %q; choose id or en", language)
	}
}
