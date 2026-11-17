// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// API is the GitHub REST endpoint of the lazynet repository. It is a variable
// only so tests (and local end-to-end checks, through -ldflags -X) can point
// it at a local server.
var API = "https://api.github.com/repos/Camilo-ovalle/lazynet"

// ReleasesPage is where users read about every release.
const ReleasesPage = "https://github.com/Camilo-ovalle/lazynet/releases"

// Release is a published lazynet release.
type Release struct {
	Version string            // "0.2.0"
	URL     string            // release page
	Notes   string            // release notes (markdown)
	Assets  map[string]string // file name -> download URL
}

// Client reads releases from GitHub.
type Client struct {
	API       string
	UserAgent string
	HTTP      *http.Client
}

// NewClient returns a client for the public lazynet repository. The user
// agent only names the program and its version.
func NewClient(version string) Client {
	return Client{
		API:       API,
		UserAgent: "lazynet/" + version,
		HTTP:      &http.Client{Timeout: 10 * time.Minute}, // downloads; requests also carry a context
	}
}

// Latest returns the newest published release (GitHub excludes drafts and
// pre-releases from /releases/latest).
func (c Client) Latest(ctx context.Context) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := c.request(ctx, c.API+"/releases/latest")
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("check for updates: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("check for updates: GitHub answered %s", resp.Status)
	}
	var body struct {
		TagName    string `json:"tag_name"`
		HTMLURL    string `json:"html_url"`
		Body       string `json:"body"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&body); err != nil {
		return Release{}, fmt.Errorf("check for updates: invalid answer: %w", err)
	}
	if body.Draft || body.Prerelease || !IsRelease(body.TagName) {
		return Release{}, fmt.Errorf("check for updates: the latest release %q is not a stable version", body.TagName)
	}
	v, _ := ParseVersion(body.TagName)
	r := Release{Version: v.String(), URL: body.HTMLURL, Notes: body.Body, Assets: map[string]string{}}
	if r.URL == "" {
		r.URL = ReleasesPage + "/tag/v" + r.Version
	}
	for _, a := range body.Assets {
		r.Assets[a.Name] = a.URL
	}
	return r, nil
}

// errTooLarge protects the disk from a broken or hostile server.
var errTooLarge = errors.New("download is larger than expected")

// download streams url into w, reading at most limit bytes.
func (c Client) download(ctx context.Context, url string, w io.Writer, limit int64) error {
	req, err := c.request(ctx, url)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return err
	}
	if n > limit {
		return errTooLarge
	}
	return nil
}

func (c Client) request(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	return req, nil
}
