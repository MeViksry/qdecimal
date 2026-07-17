//go:build releasehelper

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultAPIVersion = "2022-11-28"

var newHTTPClient = func() *http.Client {
	return &http.Client{Timeout: 90 * time.Second}
}

type release struct {
	ID        int64  `json:"id"`
	UploadURL string `json:"upload_url"`
}

type releaseAsset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type releaseNotes struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

type releasePayload struct {
	TagName              string `json:"tag_name,omitempty"`
	TargetCommitish      string `json:"target_commitish,omitempty"`
	Name                 string `json:"name,omitempty"`
	Body                 string `json:"body,omitempty"`
	Draft                bool   `json:"draft"`
	Prerelease           bool   `json:"prerelease"`
	MakeLatest           string `json:"make_latest,omitempty"`
	GenerateReleaseNotes bool   `json:"generate_release_notes,omitempty"`
}

type client struct {
	httpClient *http.Client
	apiBase    string
	token      string
	repository string
	apiVersion string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "qdecimal release: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		tag           = flag.String("tag", "", "release tag")
		name          = flag.String("name", "", "release name")
		body          = flag.String("body", "", "release body")
		prerelease    = flag.Bool("prerelease", false, "mark release as prerelease")
		makeLatest    = flag.String("make-latest", "", "GitHub make_latest value: true, false, or legacy")
		generateNotes = flag.Bool("generate-notes", false, "generate release notes through GitHub REST API")
		target        = flag.String("target", os.Getenv("GITHUB_SHA"), "target commitish")
	)
	flag.Parse()

	if strings.TrimSpace(*tag) == "" {
		return errors.New("missing -tag")
	}
	if *makeLatest != "" && *makeLatest != "true" && *makeLatest != "false" && *makeLatest != "legacy" {
		return fmt.Errorf("invalid -make-latest %q", *makeLatest)
	}
	files := flag.Args()
	if len(files) == 0 {
		return errors.New("at least one release asset is required")
	}
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			return fmt.Errorf("asset %q: %w", file, err)
		}
		if info.IsDir() {
			return fmt.Errorf("asset %q is a directory", file)
		}
	}

	c, err := newClient()
	if err != nil {
		return err
	}

	releaseBody := *body
	if *generateNotes {
		notes, err := c.generateNotes(*tag, *target)
		if err != nil {
			return err
		}
		if *name == "" {
			*name = notes.Name
		}
		releaseBody = joinReleaseBody(releaseBody, notes.Body)
	}

	rel, exists, err := c.getReleaseByTag(*tag)
	if err != nil {
		return err
	}
	payload := releasePayload{
		TagName:         *tag,
		TargetCommitish: *target,
		Name:            *name,
		Body:            releaseBody,
		Draft:           false,
		Prerelease:      *prerelease,
		MakeLatest:      *makeLatest,
	}
	if exists {
		rel, err = c.updateRelease(rel.ID, payload)
	} else {
		rel, err = c.createRelease(payload)
	}
	if err != nil {
		return err
	}
	if rel.ID == 0 || rel.UploadURL == "" {
		return errors.New("GitHub returned incomplete release metadata")
	}

	assets, err := c.listAssets(rel.ID)
	if err != nil {
		return err
	}
	assetByName := make(map[string]int64, len(assets))
	for _, asset := range assets {
		assetByName[asset.Name] = asset.ID
	}
	for _, file := range files {
		name := filepath.Base(file)
		if id, ok := assetByName[name]; ok {
			if err := c.deleteAsset(id); err != nil {
				return err
			}
		}
		if err := c.uploadAsset(rel.UploadURL, file); err != nil {
			return err
		}
	}

	fmt.Printf("qdecimal release %s published with %d asset(s)\n", *tag, len(files))
	return nil
}

func newClient() (*client, error) {
	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if token == "" {
		return nil, errors.New("GITHUB_TOKEN is required")
	}
	repository := strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY"))
	if repository == "" || strings.Count(repository, "/") != 1 {
		return nil, errors.New("GITHUB_REPOSITORY must be owner/repo")
	}
	apiBase := strings.TrimRight(os.Getenv("GITHUB_API_URL"), "/")
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	apiVersion := os.Getenv("GITHUB_API_VERSION")
	if apiVersion == "" {
		apiVersion = defaultAPIVersion
	}
	return &client{
		httpClient: newHTTPClient(),
		apiBase:    apiBase,
		token:      token,
		repository: repository,
		apiVersion: apiVersion,
	}, nil
}

func (c *client) getReleaseByTag(tag string) (release, bool, error) {
	var rel release
	status, err := c.apiJSON(http.MethodGet, "/repos/"+c.repository+"/releases/tags/"+url.PathEscape(tag), nil, &rel)
	if status == http.StatusNotFound {
		return release{}, false, nil
	}
	if err != nil {
		return release{}, false, err
	}
	return rel, true, nil
}

func (c *client) createRelease(payload releasePayload) (release, error) {
	var rel release
	status, err := c.apiJSON(http.MethodPost, "/repos/"+c.repository+"/releases", payload, &rel)
	if err != nil {
		return release{}, err
	}
	if status != http.StatusCreated {
		return release{}, fmt.Errorf("create release returned HTTP %d", status)
	}
	return rel, nil
}

func (c *client) updateRelease(id int64, payload releasePayload) (release, error) {
	payload.TagName = ""
	payload.TargetCommitish = ""
	var rel release
	status, err := c.apiJSON(http.MethodPatch, fmt.Sprintf("/repos/%s/releases/%d", c.repository, id), payload, &rel)
	if err != nil {
		return release{}, err
	}
	if status != http.StatusOK {
		return release{}, fmt.Errorf("update release returned HTTP %d", status)
	}
	return rel, nil
}

func (c *client) generateNotes(tag, target string) (releaseNotes, error) {
	payload := map[string]string{"tag_name": tag}
	if target != "" {
		payload["target_commitish"] = target
	}
	var notes releaseNotes
	status, err := c.apiJSON(http.MethodPost, "/repos/"+c.repository+"/releases/generate-notes", payload, &notes)
	if err != nil {
		return releaseNotes{}, err
	}
	if status != http.StatusOK {
		return releaseNotes{}, fmt.Errorf("generate release notes returned HTTP %d", status)
	}
	return notes, nil
}

func (c *client) listAssets(releaseID int64) ([]releaseAsset, error) {
	var assets []releaseAsset
	status, err := c.apiJSON(http.MethodGet, fmt.Sprintf("/repos/%s/releases/%d/assets?per_page=100", c.repository, releaseID), nil, &assets)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("list release assets returned HTTP %d", status)
	}
	return assets, nil
}

func (c *client) deleteAsset(assetID int64) error {
	status, err := c.apiJSON(http.MethodDelete, fmt.Sprintf("/repos/%s/releases/assets/%d", c.repository, assetID), nil, nil)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent {
		return fmt.Errorf("delete release asset returned HTTP %d", status)
	}
	return nil
}

func (c *client) uploadAsset(uploadTemplate, file string) error {
	uploadBase := strings.Split(uploadTemplate, "{")[0]
	u, err := url.Parse(uploadBase)
	if err != nil {
		return fmt.Errorf("parse upload URL: %w", err)
	}
	name := filepath.Base(file)
	query := u.Query()
	query.Set("name", name)
	u.RawQuery = query.Encode()

	body, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("open asset %q: %w", file, err)
	}
	defer body.Close()
	info, err := body.Stat()
	if err != nil {
		return fmt.Errorf("stat asset %q: %w", file, err)
	}

	req, err := http.NewRequest(http.MethodPost, u.String(), body)
	if err != nil {
		return err
	}
	c.authorize(req)
	req.ContentLength = info.Size()
	req.Header.Set("Content-Type", contentTypeFor(file))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload asset %q: %w", file, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("upload asset %q returned HTTP %d: %s", file, resp.StatusCode, responseSnippet(resp.Body))
	}
	return nil
}

func (c *client) apiJSON(method, path string, payload any, out any) (int, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.apiBase+path, body)
	if err != nil {
		return 0, err
	}
	c.authorize(req)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusNotFound {
			return resp.StatusCode, nil
		}
		return resp.StatusCode, fmt.Errorf("%s %s returned HTTP %d: %s", method, path, resp.StatusCode, responseSnippet(resp.Body))
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

func (c *client) authorize(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-GitHub-Api-Version", c.apiVersion)
	req.Header.Set("User-Agent", "qdecimal-release")
}

func responseSnippet(r io.Reader) string {
	data, _ := io.ReadAll(io.LimitReader(r, 4096))
	return strings.TrimSpace(string(data))
}

func contentTypeFor(file string) string {
	if strings.HasSuffix(file, ".tar.gz") {
		return "application/gzip"
	}
	if strings.HasSuffix(file, ".txt") || strings.HasSuffix(file, ".sha256") {
		return "text/plain; charset=utf-8"
	}
	if contentType := mime.TypeByExtension(filepath.Ext(file)); contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}

func joinReleaseBody(prefix, generated string) string {
	prefix = strings.TrimSpace(prefix)
	generated = strings.TrimSpace(generated)
	switch {
	case prefix == "":
		return generated
	case generated == "":
		return prefix
	default:
		return prefix + "\n\n" + generated
	}
}
