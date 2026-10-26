# Development notes

`mediawiki` is a Go command-line client for the MediaWiki action API. User
documentation is in README.md; keep it in sync with the `usage` string in
`cmd/mediawiki/main.go` when commands or options change.

## Layout

All code is in `cmd/mediawiki` (package `main`), so that `go install
.../cmd/mediawiki` produces an executable named `mediawiki`.

- `main.go` — usage text, version, command table (positional argument count,
  value options, boolean options, handler) and dispatch.
- `args.go` — option parser. Options may appear anywhere among positional
  arguments; a leading en/em dash is accepted as `--` (commands copied from word
  processors).
- `commands.go` — one `cmdXxx` function per command, plus `session()` (resolve
  the wiki and log in), prompts and browser opening.
- `client.go` — API client: GET/POST/multipart calls (always `format=json`,
  `formatversion=2`), API error handling, login with a bot password, CSRF token,
  `pageInfo`.
- `site.go` — wiki discovery and URL ↔ title conversion.
- `config.go` — persisted configuration.

## Key behaviors

- **API discovery**: the page URL given by the user is fetched (following
  redirects, e.g. `tripleperformance.fr` → `wiki.tripleperformance.fr`) and the
  API endpoint is read from the `<link rel="EditURI">` tag; `/w/api.php` and
  `/api.php` are probed as a fallback. `siteinfo` then gives `server` and
  `articlepath`, used to extract titles from URLs and build page URLs.
- **Config** (`config.json` in `os.UserConfigDir()/mediawiki-cli`, or
  `$MEDIAWIKI_CLI_CONFIG`): `wikis` is keyed by API URL and holds server,
  article path and credentials; `hosts` maps each host typed by the user to an
  API URL. Written with mode 0600; passwords are stored in clear (bot passwords,
  revocable on the wiki).
- Every command logs in again (bot password login is cheap); write actions send
  `assert=user` so they never run anonymously.
- `upload` resolves the title through the API to check it is in namespace 6 and
  to strip the localized prefix. Since MediaWiki only uses `text` on the first
  upload, replacing a file with `--description` also edits the description page.
- Output conventions: results on stdout, errors on stderr prefixed with
  `mediawiki <command>:`, exit 1 on failure and 2 on usage errors. `read`
  prints the content exactly, adding a final newline only on a terminal. LLM
  agents rely on these messages and exit codes: change them deliberately and
  update README.md.

## Dependencies

Standard library only, plus `golang.org/x/term` (hidden password input,
terminal detection). Go 1.26+ (required by `x/term`). Avoid adding
dependencies: the goal is a small static executable.

## Build and test

```sh
make build   # ./mediawiki
make test    # go vet + unit tests (argument parsing, title extraction)
make dist    # CGO_ENABLED=0 cross-compilation to dist/ for linux, darwin, windows × amd64, arm64
```

Run `gofmt -w cmd` before committing. The `version` variable in `main.go`
(also sent in the User-Agent) is injected with `-ldflags -X main.version=...`;
the Makefile derives it from `git describe`, or takes `VERSION=...`.

## Releases and installers

- `.github/workflows/ci.yml` runs `make test` and `make dist` on every push to
  `main` and on pull requests.
- `.github/workflows/release.yml` runs on a `v*` tag: it builds with
  `make dist VERSION=<tag without v>` and publishes a GitHub release with the
  binaries and `checksums.txt`. To release: `git tag v0.2.0 && git push origin v0.2.0`.
- Release asset names carry no version (`mediawiki-<os>-<arch>[.exe]`) so that
  `releases/latest/download/<name>` always points to the latest version. The
  install scripts rely on these names: keep them in sync with the Makefile.
- `install.sh` (macOS, Linux; POSIX sh) and `install.ps1` (Windows PowerShell
  5.1+) download the binary into a per-user folder (no admin rights needed)
  and add it to the user's PATH (shell rc file / user `Path` variable). Files
  downloaded by curl or PowerShell are not quarantined, so there is no
  Gatekeeper or SmartScreen prompt, without code signing.
- `MEDIAWIKI_DOWNLOAD_URL` overrides the download base URL, to test the
  scripts against a local server:
  `make dist && python3 -m http.server 8765 -d dist`, then
  `HOME=/tmp/fakehome MEDIAWIKI_DOWNLOAD_URL=http://localhost:8765 sh install.sh`.

## Testing against a real wiki

Never test write commands on production wikis. Use a throwaway MediaWiki in
Docker:

```sh
docker run -d --name mwcli-test -p 8089:80 mediawiki:1.43
docker exec mwcli-test php maintenance/run.php install --dbtype sqlite --dbpath /var/www/data \
  --server http://localhost:8089 --scriptpath "" --pass 'AdminPass12345!' TestWiki Admin
docker exec mwcli-test bash -c 'cat >> LocalSettings.php <<EOF
\$wgEnableUploads = true;
\$wgArticlePath = "/wiki/\$1";
EOF
chown -R www-data /var/www/data images; a2enmod rewrite
printf "RewriteEngine On\nRewriteRule ^/?wiki(/.*)?\$ %%{DOCUMENT_ROOT}/index.php [L]\n" > .htaccess
sed -i "s/AllowOverride None/AllowOverride All/" /etc/apache2/apache2.conf; apache2ctl graceful'
docker exec mwcli-test php maintenance/run.php createBotPassword \
  --grants basic,editpage,createeditmovepage,uploadfile,uploadeditmovefile,delete,highvolume Admin --appid cli
```

Then use a separate config file so real credentials are untouched:

```sh
export MEDIAWIKI_CLI_CONFIG=/tmp/mwcli-test.json
echo '<password>' | ./mediawiki login http://localhost:8089 --username Admin@cli --no-browser
./mediawiki write http://localhost:8089/wiki/Test page.wiki
docker rm -f mwcli-test
```
