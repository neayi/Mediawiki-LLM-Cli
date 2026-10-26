package main

import "golang.org/x/sys/windows"

// systemLanguage returns the user's preferred display language, e.g. "fr-FR".
func systemLanguage() string {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil || len(langs) == 0 {
		return ""
	}
	return langs[0]
}
