package main

import (
	"os/exec"
	"strings"
)

// systemLanguage returns the macOS locale, e.g. "fr_FR", for terminals that
// do not set LANG.
func systemLanguage() string {
	out, err := exec.Command("defaults", "read", "-g", "AppleLocale").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
