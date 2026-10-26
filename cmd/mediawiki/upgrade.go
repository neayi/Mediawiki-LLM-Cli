package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

const repoURL = "https://github.com/neayi/Mediawiki-LLM-Cli"

// cmdUpgrade replaces the running executable with the binary of the latest
// GitHub release, after checking it against the release's checksums.txt.
func cmdUpgrade(a *args) error {
	tag, err := latestTag()
	if err != nil {
		return fmt.Errorf("finding the latest release: %w", err)
	}
	latest := strings.TrimPrefix(tag, "v")
	if latest == version {
		fmt.Printf("mediawiki %s is already the latest version.\n", version)
		return nil
	}

	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return fmt.Errorf("locating the mediawiki executable: %w", err)
	}

	asset := "mediawiki-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		asset += ".exe"
	}
	base := repoURL + "/releases/download/" + tag + "/"
	fmt.Printf("Downloading mediawiki %s...\n", latest)
	data, err := download(base + asset)
	if err != nil {
		return err
	}
	sums, err := download(base + "checksums.txt")
	if err != nil {
		return err
	}
	if err := verifyChecksum(data, asset, sums); err != nil {
		return err
	}

	if err := replaceExecutable(exe, data); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return fmt.Errorf("%w\n%s is not writable: run the upgrade with administrator rights, or reinstall with the installer", err, exe)
		}
		return err
	}
	fmt.Printf("Upgraded mediawiki from %s to %s (%s)\n", version, latest, exe)
	return nil
}

// latestTag returns the tag of the latest release, read from the redirect of
// the releases/latest page (unlike the GitHub API, it is not rate-limited).
func latestTag() (string, error) {
	hc := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	req, err := http.NewRequest(http.MethodGet, repoURL+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	if resp.StatusCode != http.StatusFound || !strings.Contains(loc, "/releases/tag/") {
		return "", fmt.Errorf("unexpected response from %s: %s", req.URL, resp.Status)
	}
	return path.Base(loc), nil
}

func download(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := newHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %s: HTTP %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// verifyChecksum checks data against the line for name in a sha256sum file.
func verifyChecksum(data []byte, name string, sums []byte) error {
	sum := sha256.Sum256(data)
	s := bufio.NewScanner(bytes.NewReader(sums))
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) == 2 && fields[1] == name {
			if fields[0] != hex.EncodeToString(sum[:]) {
				return fmt.Errorf("checksum mismatch for %s: download corrupted, nothing was changed", name)
			}
			return nil
		}
	}
	return fmt.Errorf("no checksum for %s in the release", name)
}

// replaceExecutable atomically replaces exe with data. Windows cannot
// overwrite a running executable but can rename it: it is moved aside to
// exe.old, which removeOldExecutable deletes on a later run.
func replaceExecutable(exe string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(exe), ".mediawiki-upgrade-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(data)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), 0o755)
	}
	if err != nil {
		return err
	}

	if runtime.GOOS != "windows" {
		return os.Rename(tmp.Name(), exe)
	}
	old := exe + ".old"
	os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), exe); err != nil {
		os.Rename(old, exe)
		return err
	}
	return nil
}

// removeOldExecutable deletes the executable left behind by an upgrade on
// Windows.
func removeOldExecutable() {
	if runtime.GOOS != "windows" {
		return
	}
	if exe, err := os.Executable(); err == nil {
		os.Remove(exe + ".old")
	}
}
