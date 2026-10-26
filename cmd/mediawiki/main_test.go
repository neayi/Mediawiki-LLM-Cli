package main

import (
	"reflect"
	"testing"
)

func TestParseArgs(t *testing.T) {
	a, err := parseArgs(
		[]string{"–content-model=json", "https://w/wiki/A", "--summary", "s", "f.wiki", "--no-redirect"},
		[]string{"content-model", "summary"}, []string{"no-redirect"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"https://w/wiki/A", "f.wiki"}; !reflect.DeepEqual(a.pos, want) {
		t.Errorf("pos = %q, want %q", a.pos, want)
	}
	if a.vals["content-model"] != "json" || a.vals["summary"] != "s" || !a.bools["no-redirect"] {
		t.Errorf("unexpected options: %v %v", a.vals, a.bools)
	}
	if _, err := parseArgs([]string{"--bogus"}, nil, nil); err == nil {
		t.Error("expected an error for an unknown option")
	}
	if _, err := parseArgs([]string{"--summary"}, []string{"summary"}, nil); err == nil {
		t.Error("expected an error for a missing value")
	}
}

func TestTitle(t *testing.T) {
	w := &Wiki{Server: "https://wiki.example.org", ArticlePath: "/wiki/$1"}
	tests := map[string]string{
		"https://example.org/wiki/Une_page":                     "Une page",
		"https://example.org/wiki/File:Une_image.jpg":           "File:Une image.jpg",
		"https://example.org/wiki/Donn%C3%A9es/Sous%2Fpage":     "Données/Sous/page",
		"https://example.org/index.php?title=Une_page&action=x": "Une_page",
	}
	for u, want := range tests {
		got, err := w.title(u)
		if err != nil || got != want {
			t.Errorf("title(%q) = %q, %v; want %q", u, got, err, want)
		}
	}
	if _, err := w.title("https://example.org/"); err == nil {
		t.Error("expected an error for a URL without a title")
	}
	if got := w.pageURL("Une page"); got != "https://wiki.example.org/wiki/Une_page" {
		t.Errorf("pageURL = %q", got)
	}
}

func TestNormalizeLanguage(t *testing.T) {
	for in, want := range map[string]string{"fr_FR.UTF-8": "fr", "fr-FR": "fr", "en_US": "en", "C.UTF-8": "", "POSIX": "", "": ""} {
		if got := normalizeLanguage(in); got != want {
			t.Errorf("normalizeLanguage(%q) = %q, want %q", in, got, want)
		}
	}
}
