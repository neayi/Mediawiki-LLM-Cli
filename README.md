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
afterwards. Set `MEDIAWIKI_VERSION=0.1.0` to install a
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

To update to the latest release (versions after 0.1.0):

```sh
mediawiki upgrade        # or: mediawiki --upgrade
```

It downloads the latest release for your system, checks it against the
release checksums, and replaces the executable in place. It never replaces a
version with an older one. If the executable is
in a folder you cannot write to, run it with administrator rights or re-run the
installer.

To uninstall, delete the `mediawiki` executable and the configuration folder
described below.

### Logging in to a wiki

Log in once per wiki:

```sh
mediawiki login https://tripleperformance.fr
```

This opens the wiki's `Special:BotPasswords` page (after you press Enter) and
explains, in the language of your system (French or English), how to create a bot
password there (the names of the grants to check are shown as the wiki displays them): choose a bot name such as `mediawiki-cli`, then grant
*High-volume (bot) access*, *Edit existing pages*, *Create, edit, and move
pages*, *Upload new files*, *Upload, replace, and move files*, *Patrol changes
to pages* and *Rollback changes to pages* (add *Delete pages, revisions, and
log entries* to use `mediawiki delete`). Then type the bot username
(`YourName@mediawiki-cli`) and password when prompted. They are checked, then stored on the machine
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
mediawiki api    "<api-url>?action=...&param=value"
mediawiki login  <wiki-url> [--username NAME] [--no-browser]
mediawiki logout <wiki-url>
mediawiki help
mediawiki upgrade
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

### api — call any API action

```sh
mediawiki api "https://wiki.tripleperformance.fr/api.php?action=linktitles&page=Une_page"
mediawiki api "https://wiki.tripleperformance.fr/api.php?action=query&list=categorymembers&cmtitle=Catégorie:Élevage"
```

- The argument is an `api.php` URL with the parameters of the call in its
  query string. Quote it: `&` is special in the shell. The wiki is identified
  by the host; the path is ignored and the wiki's API endpoint is used.
- The tool asks the wiki (`action=paraminfo`) how the action must be called:
  actions that must be posted are sent with POST, and actions that take a
  token get one of the right type (`csrf`, `watch`, `patrol`…) automatically,
  with `assert=user`. These require a login; others work anonymously on public
  wikis.
- `format=json&formatversion=2` is always used.
- Output: the JSON response, indented. API errors are printed on standard error
  as `mediawiki api: <info> (<code>)`, with status 1.

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
