package main

import (
	"os"
	"strings"
)

// userLanguage returns the lowercase language code of the user, such as "fr"
// or "en", from the locale environment variables or else from the system
// settings. It returns "" when unknown.
func userLanguage() string {
	for _, v := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if l := normalizeLanguage(os.Getenv(v)); l != "" {
			return l
		}
	}
	return normalizeLanguage(systemLanguage())
}

// normalizeLanguage turns a locale such as "fr_FR.UTF-8" or "fr-FR" into "fr".
// The C and POSIX locales carry no language.
func normalizeLanguage(locale string) string {
	l, _, _ := strings.Cut(locale, ".")
	l, _, _ = strings.Cut(l, "_")
	l, _, _ = strings.Cut(l, "-")
	l = strings.ToLower(l)
	if l == "c" || l == "posix" {
		return ""
	}
	return l
}
