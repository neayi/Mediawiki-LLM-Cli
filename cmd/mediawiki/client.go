package main

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var userAgent = "mediawiki-cli/" + version + " (https://github.com/neayi/Mediawiki-LLM-Cli)"

// Client talks to the action API of one wiki. Session cookies are kept for the
// lifetime of the client, so a login applies to all subsequent calls.
type Client struct {
	api  string
	hc   *http.Client
	csrf string
}

type apiError struct {
	Code string `json:"code"`
	Info string `json:"info"`
}

func (e *apiError) Error() string {
	return fmt.Sprintf("%s (%s)", e.Info, e.Code)
}

func newHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, Timeout: 10 * time.Minute}
}

func newClient(api string) *Client {
	return &Client{api: api, hc: newHTTPClient()}
}

func (c *Client) get(params url.Values) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.api+"?"+withFormat(params).Encode(), nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *Client) post(params url.Values) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, c.api, strings.NewReader(withFormat(params).Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.do(req)
}

// postFile sends params as multipart/form-data, with the content of the local
// file at path in the form field named field. The file is streamed.
func (c *Client) postFile(params url.Values, field, path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		for k, vs := range withFormat(params) {
			for _, v := range vs {
				if err := mw.WriteField(k, v); err != nil {
					pw.CloseWithError(err)
					return
				}
			}
		}
		part, err := mw.CreateFormFile(field, filepath.Base(path))
		if err == nil {
			_, err = io.Copy(part, f)
		}
		if err == nil {
			err = mw.Close()
		}
		pw.CloseWithError(err)
	}()

	req, err := http.NewRequest(http.MethodPost, c.api, pr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return c.do(req)
}

func withFormat(params url.Values) url.Values {
	params.Set("format", "json")
	params.Set("formatversion", "2")
	return params
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %s", c.api, resp.Status)
	}
	var r struct {
		Error *apiError `json:"error"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("%s did not return a MediaWiki API response: %w", c.api, err)
	}
	if r.Error != nil {
		return nil, r.Error
	}
	return body, nil
}

func (c *Client) login(username, password string) error {
	body, err := c.get(url.Values{"action": {"query"}, "meta": {"tokens"}, "type": {"login"}})
	if err != nil {
		return err
	}
	var t struct {
		Query struct {
			Tokens struct {
				LoginToken string `json:"logintoken"`
			} `json:"tokens"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &t); err != nil {
		return err
	}

	body, err = c.post(url.Values{
		"action":     {"login"},
		"lgname":     {username},
		"lgpassword": {password},
		"lgtoken":    {t.Query.Tokens.LoginToken},
	})
	if err != nil {
		return err
	}
	var r struct {
		Login struct {
			Result string `json:"result"`
			Reason string `json:"reason"`
		} `json:"login"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return err
	}
	if r.Login.Result != "Success" {
		return fmt.Errorf("login failed: %s %s", r.Login.Result, r.Login.Reason)
	}
	return nil
}

func (c *Client) csrfToken() (string, error) {
	if c.csrf != "" {
		return c.csrf, nil
	}
	body, err := c.get(url.Values{"action": {"query"}, "meta": {"tokens"}})
	if err != nil {
		return "", err
	}
	var t struct {
		Query struct {
			Tokens struct {
				CSRFToken string `json:"csrftoken"`
			} `json:"tokens"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &t); err != nil {
		return "", err
	}
	if t.Query.Tokens.CSRFToken == "" || t.Query.Tokens.CSRFToken == "+\\" {
		return "", fmt.Errorf("not logged in: the wiki returned an anonymous edit token")
	}
	c.csrf = t.Query.Tokens.CSRFToken
	return c.csrf, nil
}

// postWithToken performs a write action as the logged-in user.
func (c *Client) postWithToken(params url.Values) ([]byte, error) {
	token, err := c.csrfToken()
	if err != nil {
		return nil, err
	}
	params.Set("token", token)
	params.Set("assert", "user")
	return c.post(params)
}

type pageInfo struct {
	NS      int    `json:"ns"`
	Title   string `json:"title"`
	Missing bool   `json:"missing"`
	Invalid bool   `json:"invalid"`
	Reason  string `json:"invalidreason"`
}

// pageInfo returns the normalized title and existence of a page.
func (c *Client) pageInfo(title string) (*pageInfo, error) {
	body, err := c.get(url.Values{"action": {"query"}, "titles": {title}})
	if err != nil {
		return nil, err
	}
	var r struct {
		Query struct {
			Pages []pageInfo `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	if len(r.Query.Pages) == 0 {
		return nil, fmt.Errorf("no page information returned for %q", title)
	}
	p := r.Query.Pages[0]
	if p.Invalid {
		return nil, fmt.Errorf("invalid title %q: %s", title, p.Reason)
	}
	return &p, nil
}

// messages returns the content language of the wiki and the text of the given
// interface messages in that language. It fails if any message is missing.
func (c *Client) messages(names []string) (string, map[string]string, error) {
	body, err := c.get(url.Values{
		"action":     {"query"},
		"meta":       {"siteinfo|allmessages"},
		"ammessages": {strings.Join(names, "|")},
	})
	if err != nil {
		return "", nil, err
	}
	var r struct {
		Query struct {
			General struct {
				Lang string `json:"lang"`
			} `json:"general"`
			AllMessages []struct {
				Name    string `json:"name"`
				Content string `json:"content"`
				Missing bool   `json:"missing"`
			} `json:"allmessages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", nil, err
	}
	msgs := map[string]string{}
	for _, m := range r.Query.AllMessages {
		if !m.Missing && m.Content != "" {
			msgs[m.Name] = m.Content
		}
	}
	for _, n := range names {
		if _, ok := msgs[n]; !ok {
			return "", nil, fmt.Errorf("message %s not found", n)
		}
	}
	return r.Query.General.Lang, msgs, nil
}
