package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"strings"
)

var (
	editURILinkRe = regexp.MustCompile(`(?i)<link[^>]+rel="EditURI"[^>]*>`)
	hrefRe        = regexp.MustCompile(`(?i)href="([^"]+)"`)
)

// resolveWiki finds the wiki a page URL belongs to. The API endpoint of each
// host is discovered once, then remembered in the configuration.
func resolveWiki(cfg *Config, pageURL string, refresh bool) (*Wiki, error) {
	u, err := url.Parse(pageURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("%q is not a wiki URL (expected https://host/wiki/Page)", pageURL)
	}
	host := strings.ToLower(u.Host)
	if api, ok := cfg.Hosts[host]; ok && !refresh {
		if w, ok := cfg.Wikis[api]; ok {
			return w, nil
		}
	}

	api, err := discoverAPI(pageURL)
	if err != nil {
		return nil, err
	}
	info, err := newClient(api).siteInfo()
	if err != nil {
		return nil, err
	}
	w, ok := cfg.Wikis[api]
	if !ok {
		w = &Wiki{API: api}
		cfg.Wikis[api] = w
	}
	w.Server, w.ArticlePath = info.Server, info.ArticlePath
	// A protocol-relative server takes the scheme of the API, which was
	// reached after following redirects (e.g. http to https).
	if strings.HasPrefix(w.Server, "//") {
		scheme, _, _ := strings.Cut(api, ":")
		w.Server = scheme + ":" + w.Server
	}
	cfg.Hosts[host] = api
	if err := cfg.save(); err != nil {
		return nil, err
	}
	return w, nil
}

// discoverAPI fetches the page and reads the API endpoint from its EditURI
// link, which every MediaWiki page advertises. Redirects (to another host, for
// instance) are followed. Common install paths are probed as a fallback.
func discoverAPI(pageURL string) (string, error) {
	hc := newHTTPClient()
	resp, err := hc.Get(pageURL)
	if err != nil {
		return "", err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	resp.Body.Close()
	if err != nil {
		return "", err
	}
	final := resp.Request.URL

	if link := editURILinkRe.Find(body); link != nil {
		if m := hrefRe.FindSubmatch(link); m != nil {
			if ref, err := url.Parse(html.UnescapeString(string(m[1]))); err == nil {
				api := final.ResolveReference(ref)
				api.RawQuery, api.Fragment = "", ""
				return api.String(), nil
			}
		}
	}

	for _, path := range []string{"/w/api.php", "/api.php"} {
		api := final.Scheme + "://" + final.Host + path
		if _, err := newClient(api).siteInfo(); err == nil {
			return api, nil
		}
	}
	return "", fmt.Errorf("could not find the MediaWiki API of %s", final.Host)
}

type siteInfo struct {
	Server      string `json:"server"`
	ArticlePath string `json:"articlepath"`
}

func (c *Client) siteInfo() (*siteInfo, error) {
	body, err := c.get(url.Values{"action": {"query"}, "meta": {"siteinfo"}})
	if err != nil {
		return nil, err
	}
	var r struct {
		Query struct {
			General siteInfo `json:"general"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	if r.Query.General.ArticlePath == "" {
		return nil, fmt.Errorf("%s is not a MediaWiki API", c.api)
	}
	return &r.Query.General, nil
}

// title extracts the page title from a page URL, either /wiki/Title or
// index.php?title=Title.
func (w *Wiki) title(pageURL string) (string, error) {
	u, err := url.Parse(pageURL)
	if err != nil {
		return "", err
	}
	if t := u.Query().Get("title"); t != "" {
		return t, nil
	}
	prefix, suffix, _ := strings.Cut(w.ArticlePath, "$1")
	path := u.EscapedPath()
	if len(path) > len(prefix)+len(suffix) && strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix) {
		t, err := url.PathUnescape(path[len(prefix) : len(path)-len(suffix)])
		if err != nil {
			return "", err
		}
		return strings.ReplaceAll(t, "_", " "), nil
	}
	return "", fmt.Errorf("no page title found in %q (expected %s%s)", pageURL, w.Server, w.ArticlePath)
}

// pageURL returns the canonical URL of a page.
func (w *Wiki) pageURL(title string) string {
	t := url.PathEscape(strings.ReplaceAll(title, " ", "_"))
	return w.Server + strings.Replace(w.ArticlePath, "$1", t, 1)
}
