package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/term"
)

const defaultSummary = "Edited with the mediawiki command-line tool"

// session resolves the wiki of pageURL and returns a client for it, logged in
// with the stored bot password if there is one. With requireLogin, a missing
// login is an error.
func session(pageURL string, requireLogin bool) (*Wiki, *Client, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, nil, err
	}
	w, err := resolveWiki(cfg, pageURL, false)
	if err != nil {
		return nil, nil, err
	}
	c := newClient(w.API)
	if w.Username == "" {
		if requireLogin {
			return nil, nil, fmt.Errorf("not logged in to %s; run: mediawiki login %s", w.Server, w.Server)
		}
		return w, c, nil
	}
	if err := c.login(w.Username, w.Password); err != nil {
		return nil, nil, fmt.Errorf("%w (run: mediawiki login %s)", err, w.Server)
	}
	return w, c, nil
}

func cmdLogin(a *args) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	w, err := resolveWiki(cfg, a.pos[0], true)
	if err != nil {
		return err
	}

	link := w.pageURL("Special:BotPasswords")
	fmt.Printf(`To use this tool, create a bot password on %s:

  %s

Give the bot a name (e.g. "cli") and grant it at least:
  - Basic rights
  - Edit existing pages
  - Create, edit, and move pages
  - Upload new files / Upload, replace, and move files
  - Delete pages, revisions, and log entries (for "mediawiki delete")

The wiki then shows a username like "YourName@cli" and a password.

`, w.Server, link)
	if !a.bools["no-browser"] {
		openBrowser(link)
	}

	in := bufio.NewReader(os.Stdin)
	username := a.vals["username"]
	if username == "" {
		if username, err = prompt(in, "Bot username: "); err != nil {
			return err
		}
	}
	password, err := promptPassword(in, "Bot password: ")
	if err != nil {
		return err
	}
	if username == "" || password == "" {
		return fmt.Errorf("username and password are required")
	}

	if err := newClient(w.API).login(username, password); err != nil {
		return err
	}
	w.Username, w.Password = username, password
	if err := cfg.save(); err != nil {
		return err
	}
	fmt.Printf("Logged in to %s as %s. Credentials saved in %s\n", w.Server, username, cfg.path)
	return nil
}

func cmdLogout(a *args) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	w, err := resolveWiki(cfg, a.pos[0], false)
	if err != nil {
		return err
	}
	w.Username, w.Password = "", ""
	if err := cfg.save(); err != nil {
		return err
	}
	fmt.Printf("Credentials for %s removed.\n", w.Server)
	return nil
}

func cmdRead(a *args) error {
	w, c, err := session(a.pos[0], false)
	if err != nil {
		return err
	}
	title, err := w.title(a.pos[0])
	if err != nil {
		return err
	}
	body, err := c.get(url.Values{
		"action":  {"query"},
		"prop":    {"revisions"},
		"rvprop":  {"content"},
		"rvslots": {"main"},
		"titles":  {title},
	})
	if err != nil {
		return err
	}
	var r struct {
		Query struct {
			Pages []struct {
				pageInfo
				Revisions []struct {
					Slots struct {
						Main struct {
							Content string `json:"content"`
						} `json:"main"`
					} `json:"slots"`
				} `json:"revisions"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return err
	}
	if len(r.Query.Pages) == 0 {
		return fmt.Errorf("no page returned for %q", title)
	}
	p := r.Query.Pages[0]
	switch {
	case p.Invalid:
		return fmt.Errorf("invalid title %q: %s", title, p.Reason)
	case p.Missing || len(p.Revisions) == 0:
		return fmt.Errorf("page %q does not exist", p.Title)
	}
	content := p.Revisions[0].Slots.Main.Content
	if term.IsTerminal(int(os.Stdout.Fd())) && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	_, err = io.WriteString(os.Stdout, content)
	return err
}

func cmdWrite(a *args) error {
	text, err := readInput(a.pos[1])
	if err != nil {
		return err
	}
	w, c, err := session(a.pos[0], true)
	if err != nil {
		return err
	}
	title, err := w.title(a.pos[0])
	if err != nil {
		return err
	}
	r, err := edit(c, title, text, a.vals["content-model"], summary(a.vals["summary"]))
	if err != nil {
		return err
	}
	switch {
	case r.NoChange:
		fmt.Printf("No change: %s\n", w.pageURL(r.Title))
	case r.New:
		fmt.Printf("Created %s (revision %d)\n", w.pageURL(r.Title), r.NewRevID)
	default:
		fmt.Printf("Updated %s (revision %d)\n", w.pageURL(r.Title), r.NewRevID)
	}
	return nil
}

type editResult struct {
	Result   string `json:"result"`
	Title    string `json:"title"`
	New      bool   `json:"new"`
	NoChange bool   `json:"nochange"`
	NewRevID int    `json:"newrevid"`
}

// edit replaces the content of a page, creating it if needed.
func edit(c *Client, title, text, contentModel, summary string) (*editResult, error) {
	params := url.Values{
		"action":  {"edit"},
		"title":   {title},
		"text":    {text},
		"summary": {summary},
		"bot":     {"1"},
	}
	if contentModel != "" {
		params.Set("contentmodel", contentModel)
	}
	body, err := c.postWithToken(params)
	if err != nil {
		return nil, err
	}
	var r struct {
		Edit json.RawMessage `json:"edit"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	var e editResult
	if err := json.Unmarshal(r.Edit, &e); err != nil {
		return nil, err
	}
	if e.Result != "Success" {
		return nil, fmt.Errorf("edit refused: %s", r.Edit)
	}
	return &e, nil
}

func cmdUpload(a *args) error {
	localFile := a.pos[1]
	if _, err := os.Stat(localFile); err != nil {
		return err
	}
	var description string
	if path := a.vals["description"]; path != "" {
		var err error
		if description, err = readInput(path); err != nil {
			return err
		}
	}
	w, c, err := session(a.pos[0], true)
	if err != nil {
		return err
	}
	title, err := w.title(a.pos[0])
	if err != nil {
		return err
	}
	info, err := c.pageInfo(title)
	if err != nil {
		return err
	}
	if info.NS != 6 {
		return fmt.Errorf("%q is not a file page (expected File:Name.ext)", info.Title)
	}
	_, filename, _ := strings.Cut(info.Title, ":")

	token, err := c.csrfToken()
	if err != nil {
		return err
	}
	params := url.Values{
		"action":   {"upload"},
		"filename": {filename},
		"comment":  {summary(a.vals["summary"])},
		"token":    {token},
		"assert":   {"user"},
	}
	if description != "" {
		params.Set("text", description)
	}
	if a.bools["ignore-warnings"] {
		params.Set("ignorewarnings", "1")
	}
	body, err := c.postFile(params, "file", localFile)
	if err != nil {
		return err
	}
	var r struct {
		Upload struct {
			Result   string                     `json:"result"`
			Warnings map[string]json.RawMessage `json:"warnings"`
		} `json:"upload"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return err
	}
	if r.Upload.Result != "Success" {
		var warnings []string
		for k, v := range r.Upload.Warnings {
			warnings = append(warnings, fmt.Sprintf("%s: %s", k, v))
		}
		sort.Strings(warnings)
		return fmt.Errorf("upload not done (%s): %s\nUse --ignore-warnings to upload anyway.", r.Upload.Result, strings.Join(warnings, "; "))
	}

	// The description is only used when a file is first uploaded: update the
	// page explicitly when replacing an existing file.
	if !info.Missing && description != "" {
		if _, err := edit(c, info.Title, description, "", summary(a.vals["summary"])); err != nil {
			return fmt.Errorf("file uploaded, but updating its description failed: %w", err)
		}
	}
	fmt.Printf("Uploaded %s\n", w.pageURL(info.Title))
	return nil
}

func cmdRename(a *args) error {
	w, c, err := session(a.pos[0], true)
	if err != nil {
		return err
	}
	from, err := w.title(a.pos[0])
	if err != nil {
		return err
	}
	to, err := w.title(a.pos[1])
	if err != nil {
		return err
	}
	params := url.Values{
		"action":   {"move"},
		"from":     {from},
		"to":       {to},
		"reason":   {a.vals["reason"]},
		"movetalk": {"1"},
	}
	if a.bools["no-redirect"] {
		params.Set("noredirect", "1")
	}
	body, err := c.postWithToken(params)
	if err != nil {
		return err
	}
	var r struct {
		Move struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"move"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return err
	}
	fmt.Printf("Renamed %q to %s\n", r.Move.From, w.pageURL(r.Move.To))
	return nil
}

func cmdDelete(a *args) error {
	w, c, err := session(a.pos[0], true)
	if err != nil {
		return err
	}
	title, err := w.title(a.pos[0])
	if err != nil {
		return err
	}
	body, err := c.postWithToken(url.Values{
		"action": {"delete"},
		"title":  {title},
		"reason": {a.vals["reason"]},
	})
	if err != nil {
		return err
	}
	var r struct {
		Delete struct {
			Title string `json:"title"`
		} `json:"delete"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return err
	}
	fmt.Printf("Deleted %q\n", r.Delete.Title)
	return nil
}

func summary(s string) string {
	if s == "" {
		return defaultSummary
	}
	return s
}

// readInput returns the content of a local file, or of standard input for "-".
func readInput(path string) (string, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	return string(data), err
}

func prompt(in *bufio.Reader, label string) (string, error) {
	fmt.Print(label)
	line, err := in.ReadString('\n')
	if err != nil && (err != io.EOF || line == "") {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// promptPassword reads a password without echoing it when stdin is a
// terminal, or as a plain line otherwise (when piped).
func promptPassword(in *bufio.Reader, label string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return prompt(in, label)
	}
	fmt.Print(label)
	b, err := term.ReadPassword(fd)
	fmt.Println()
	return strings.TrimSpace(string(b)), err
}

// openBrowser opens url in the default browser, on a best-effort basis: the
// link is printed anyway.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
