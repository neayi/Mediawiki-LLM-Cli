package main

import (
	"bufio"
	"bytes"
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

	in := bufio.NewReader(os.Stdin)
	// The instructions follow the language of the system, but the labels of
	// the grants and buttons to click are read from the wiki in its own
	// language, and uselang shows the page in that language so that they match.
	t := loginTexts["en"]
	if strings.HasPrefix(userLanguage(), "fr") {
		t = loginTexts["fr"]
	}
	pageLang, labels := t.lang, t.labels
	if lang, msgs, err := newClient(w.API).messages(botPasswordMessages); err == nil {
		pageLang, labels = lang, msgs
	}
	link := w.pageURL("Special:BotPasswords")
	if strings.Contains(link, "?") {
		link += "&uselang=" + pageLang
	} else {
		link += "?uselang=" + pageLang
	}
	fmt.Printf(t.intro, w.Server)
	// Wait for the user before opening the browser, unless the input is piped
	// (it then holds the password).
	opened := false
	if !a.bools["no-browser"] && term.IsTerminal(int(os.Stdin.Fd())) {
		if _, err := prompt(in, t.pressEnter); err != nil {
			return err
		}
		opened = openBrowser(link) == nil
	}
	if opened {
		fmt.Printf(t.ifNotOpened, link)
	} else {
		fmt.Printf(t.openManually, link)
	}
	var grants strings.Builder
	for _, m := range botPasswordMessages[1:] {
		fmt.Fprintf(&grants, "       - %s\n", labels[m])
	}
	fmt.Printf(t.steps, labels[botPasswordMessages[0]], grants.String())

	username := a.vals["username"]
	if username == "" {
		if username, err = prompt(in, t.username); err != nil {
			return err
		}
	}
	password, err := promptPassword(in, t.password)
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

// cmdAPI calls any API action given as an api.php URL, e.g.
// https://host/api.php?action=linktitles&page=Main_Page. The wiki is
// identified by the host only; parameters come from the query string. Actions
// that take a token are posted with a token of the right type.
func cmdAPI(a *args) error {
	u, err := url.Parse(a.pos[0])
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("%q is not an API URL (expected https://host/api.php?action=...)", a.pos[0])
	}
	params := u.Query()
	action := params.Get("action")
	if action == "" {
		return fmt.Errorf("no action parameter in %q", a.pos[0])
	}
	w, c, err := session(u.Scheme+"://"+u.Host+"/", false)
	if err != nil {
		return err
	}
	m, err := c.moduleInfo(action)
	if err != nil {
		return err
	}
	if m.TokenType != "" && params.Get("token") == "" {
		if w.Username == "" {
			return fmt.Errorf("action %s needs a token, but you are not logged in to %s; run: mediawiki login %s", action, w.Server, w.Server)
		}
		token, err := c.token(m.TokenType)
		if err != nil {
			return err
		}
		params.Set("token", token)
		params.Set("assert", "user")
	}
	var body []byte
	if m.MustBePosted || params.Has("token") {
		body, err = c.post(params)
	} else {
		body, err = c.get(params)
	}
	if err != nil {
		return err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, body, "", "  "); err != nil {
		return err
	}
	out.WriteByte('\n')
	_, err = out.WriteTo(os.Stdout)
	return err
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

// openBrowser opens url in the default browser. It reports whether the
// browser could be launched, not whether the page was displayed.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// botPasswordMessages are the MediaWiki messages for the labels the user
// sees on Special:BotPasswords: the create button, then the grants needed.
var botPasswordMessages = []string{
	"botpasswords-label-create",
	"grant-highvolume",
	"grant-editpage",
	"grant-createeditmovepage",
	"grant-uploadfile",
	"grant-uploadeditmovefile",
	"grant-patrol",
	"grant-rollback",
}

// loginTexts holds the login instructions, by language. labels are used when
// the wiki's own labels cannot be read; they are the botPasswordMessages in
// lang.
var loginTexts = map[string]struct {
	intro, pressEnter, ifNotOpened, openManually, steps, username, password string
	lang                                                                    string
	labels                                                                  map[string]string
}{
	"en": {
		intro: `To use this tool on %s, you need to create a bot password on the wiki.
The bot password creation page will open in your browser (log in to the wiki first if needed).
`,
		pressEnter:   "Press Enter to open it...",
		ifNotOpened:  "\nIf the page did not open, copy this address into your browser:\n\n  %s\n",
		openManually: "\nOpen this address in your browser:\n\n  %s\n",
		steps: `
On this page:
  1. Choose a name for the bot (for example mediawiki-cli), then click "%[1]s".
  2. Check the following grants, then click "%[1]s":
%[2]s  3. Copy the username shown (of the form YourName@mediawiki-cli) and the
     password, and paste them below.

`,
		username: "Bot username: ",
		password: "Bot password: ",
		lang:     "en",
		labels: map[string]string{
			"botpasswords-label-create": "Create",
			"grant-highvolume":          "High-volume (bot) access",
			"grant-editpage":            "Edit existing pages",
			"grant-createeditmovepage":  "Create, edit, and move pages",
			"grant-uploadfile":          "Upload new files",
			"grant-uploadeditmovefile":  "Upload, replace, and move files",
			"grant-patrol":              "Patrol changes to pages",
			"grant-rollback":            "Rollback changes to pages",
		},
	},
	"fr": {
		intro: `Pour utiliser cet outil sur %s, vous devez créer un mot de passe de robot sur le wiki.
La page de création des mots de passe de robot va s'ouvrir dans votre navigateur (connectez-vous d'abord au wiki si nécessaire).
`,
		pressEnter:   "Appuyez sur Entrée pour l'ouvrir...",
		ifNotOpened:  "\nSi la page ne s'est pas ouverte, copiez cette adresse dans votre navigateur :\n\n  %s\n",
		openManually: "\nOuvrez cette adresse dans votre navigateur :\n\n  %s\n",
		steps: `
Sur cette page :
  1. Choisissez un nom pour le robot (par exemple mediawiki-cli), puis cliquez sur « %[1]s ».
  2. Cochez les droits suivants, puis cliquez sur « %[1]s » :
%[2]s  3. Copiez l'identifiant affiché (de la forme VotreNom@mediawiki-cli) et le
     mot de passe, puis collez-les ci-dessous.

`,
		username: "Identifiant du robot : ",
		password: "Mot de passe du robot : ",
		lang:     "fr",
		labels: map[string]string{
			"botpasswords-label-create": "Créer",
			"grant-highvolume":          "Accès à haut volume (robot)",
			"grant-editpage":            "Modifier des pages existantes",
			"grant-createeditmovepage":  "Créer, modifier et déplacer des pages",
			"grant-uploadfile":          "Téléverser de nouveaux fichiers",
			"grant-uploadeditmovefile":  "Téléverser, remplacer et renommer des fichiers",
			"grant-patrol":              "Vérifier les modifications de pages",
			"grant-rollback":            "Révoquer des modifications sur des pages",
		},
	},
}
