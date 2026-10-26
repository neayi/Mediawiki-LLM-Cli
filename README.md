# mediawiki

A command-line tool to read and edit pages of a MediaWiki wiki (create, update,
upload, rename, delete). It is designed to be driven by a human or by an LLM
agent such as Claude Code. Single executable for Linux, macOS and Windows.

## Installation

**Windows** — open PowerShell and run:

```powershell
irm https://raw.githubusercontent.com/neayi/Mediawiki-LLM-Cli/main/install.ps1 | iex
```

**macOS and Linux** — open a terminal and run:

```sh
curl -fsSL https://raw.githubusercontent.com/neayi/Mediawiki-LLM-Cli/main/install.sh | sh
```

The installer downloads the latest release for your system and processor,
installs it (`%LOCALAPPDATA%\Programs\mediawiki` on Windows, `~/.local/bin` on
macOS and Linux) and adds that folder to your `PATH`. Open a new terminal
afterwards. Run it again to update. Set `MEDIAWIKI_VERSION=0.1.0` to install a
given version, or `MEDIAWIKI_INSTALL_DIR` to install elsewhere.

Manual installation: download the file for your system from the
[latest release](https://github.com/neayi/Mediawiki-LLM-Cli/releases/latest)
(`mediawiki-windows-amd64.exe`, `mediawiki-darwin-arm64` for Apple Silicon
Macs, `mediawiki-darwin-amd64` for Intel Macs…), rename it `mediawiki.exe` or
`mediawiki`, and put it in a folder of your `PATH`. On macOS, a file downloaded
with a browser is blocked by Gatekeeper; unblock it with
`xattr -d com.apple.quarantine mediawiki`.

Developers can also use `go install github.com/neayi/Mediawiki-LLM-Cli/cmd/mediawiki@latest`
(Go 1.26 or later).

To uninstall, delete the `mediawiki` executable and the configuration folder
described below.

### Logging in to a wiki

Log in once per wiki:

```sh
mediawiki login https://tripleperformance.fr
```

This opens the wiki's `Special:BotPasswords` page. Create a bot password there,
granting at least: *Basic rights*, *Edit existing pages*, *Create, edit, and
move pages*, *Upload new files*, *Upload, replace, and move files*, and *Delete
pages* if you need `delete`. Then type the bot username (`YourName@botname`)
and password when prompted. They are checked, then stored on the machine
(`~/.config/mediawiki-cli/config.json` on Linux, `~/Library/Application
Support/mediawiki-cli/config.json` on macOS, `%AppData%\mediawiki-cli\config.json`
on Windows; override with the `MEDIAWIKI_CLI_CONFIG` environment variable).

Non-interactive login (the password is read from standard input):

```sh
echo "$BOT_PASSWORD" | mediawiki login https://tripleperformance.fr --username Me@bot --no-browser
```

## Usage

Every command takes the full URL of a page, as shown in a browser
(`https://host/wiki/Title` or `https://host/index.php?title=Title`). The wiki
is identified from the URL; no other configuration is needed.

```sh
mediawiki read   <page-url>
mediawiki write  <page-url> <file> [--content-model MODEL] [--summary TEXT]
mediawiki upload <file-page-url> <local-file> [--description FILE] [--summary TEXT] [--ignore-warnings]
mediawiki rename <page-url> <new-page-url> [--reason TEXT] [--no-redirect]
mediawiki delete <page-url> [--reason TEXT]
mediawiki login  <wiki-url> [--username NAME] [--no-browser]
mediawiki logout <wiki-url>
mediawiki help
mediawiki version
```

### read — print the wikicode of a page

```sh
mediawiki read https://tripleperformance.fr/wiki/Une_page > unepage.wiki
```

Prints the raw content of the page (wikitext, JSON…) on standard output,
nothing else. Does not require a login on public wikis.

### write — create or replace a page

```sh
mediawiki write https://tripleperformance.fr/wiki/Une_page unepage.wiki --summary "Add section"
mediawiki write --content-model=json https://tripleperformance.fr/wiki/Une_page data.json
cat unepage.wiki | mediawiki write https://tripleperformance.fr/wiki/Une_page -
```

- The **whole** content of the page is replaced by the file. To change part of
  a page, `read` it first, edit the file, then `write` it back.
- The page is created if it does not exist.
- `--content-model` also changes the content model of the page (`wikitext`,
  `json`, `css`, `javascript`, `text`…). For `json`, the file must be valid JSON.
- Output: `Created <url> (revision N)`, `Updated <url> (revision N)` or
  `No change: <url>`.

### upload — upload a file

```sh
mediawiki upload https://tripleperformance.fr/wiki/File:Une_image.jpg image_locale.jpg
mediawiki upload https://tripleperformance.fr/wiki/File:Une_image.jpg image_locale.jpg --description description.wiki
```

- The target name comes from the URL (`File:Une_image.jpg` → `Une image.jpg`);
  the localized namespace (`Fichier:`) also works.
- `--description` gives a file containing the wikicode of the file description
  page (description, categories, license…).
- If the wiki returns warnings (file already exists, duplicate of another
  file…), nothing is uploaded and the warnings are printed. Re-run with
  `--ignore-warnings` to proceed, e.g. to replace an existing file. When
  replacing a file with `--description`, the description page is updated too.
- Output: `Uploaded <url>`.

### rename — move a page

```sh
mediawiki rename https://tripleperformance.fr/wiki/Une_page https://tripleperformance.fr/wiki/Une_autre_page
```

The talk page is moved too. A redirect is left at the old title unless
`--no-redirect` is given. Output: `Renamed "<old title>" to <new url>`.

### delete — delete a page

```sh
mediawiki delete https://tripleperformance.fr/wiki/Une_page --reason "Duplicate"
```

Output: `Deleted "<title>"`.

### General rules

- Options may be placed before or after the URLs, as `--name value` or
  `--name=value`.
- A file argument of `-` means standard input.
- Exit status: `0` on success, `1` on failure (message on standard error,
  starting with `mediawiki <command>:`), `2` on invalid usage.
- `read` on a page that does not exist fails with
  `page "<title>" does not exist` and status 1: use it to check whether a page
  exists.
- Write commands fail with `not logged in to <wiki>` if no credentials are
  stored for that wiki: run `mediawiki login <wiki-url>`.
- Edits are marked as bot edits when the account has the bot right. The default
  edit summary is "Edited with the mediawiki command-line tool".
