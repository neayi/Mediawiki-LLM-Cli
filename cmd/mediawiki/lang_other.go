//go:build !windows && !darwin

package main

// systemLanguage has no fallback on Linux and other Unix systems, where the
// locale environment variables are authoritative.
func systemLanguage() string { return "" }
