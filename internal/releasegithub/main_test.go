//go:build releasehelper

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRunCreatesReleaseWithGeneratedNotesAndUploadsAssets(t *testing.T) {
	archive := writeTestAsset(t, "qdecimal-v1.2.3.tar.gz", "archive")
	checksum := writeTestAsset(t, "qdecimal-v1.2.3.sha256", "checksum")

	var calls []string
	var uploaded []string
	var createPayload releasePayload
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assertGitHubHeaders(t, r)
		calls = append(calls, r.Method+" "+r.URL.Path)

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/releases/generate-notes":
			var payload map[string]string
			decodeRequestJSON(t, r, &payload)
			if payload["tag_name"] != "v1.2.3" || payload["target_commitish"] != "abc123" {
				t.Fatalf("generate notes payload got %#v", payload)
			}
			return jsonResponse(http.StatusOK, releaseNotes{Name: "qdecimal v1.2.3", Body: "generated notes"}), nil
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/tags/"):
			return textResponse(http.StatusNotFound, "not found"), nil
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/releases":
			decodeRequestJSON(t, r, &createPayload)
			return jsonResponse(http.StatusCreated, release{ID: 99, UploadURL: "https://uploads.github.test/upload/assets{?name,label}"}), nil
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/releases/99/assets":
			if r.URL.Query().Get("per_page") != "100" {
				t.Fatalf("assets query got %s", r.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, []releaseAsset{{ID: 10, Name: filepath.Base(archive)}}), nil
		case r.Method == http.MethodDelete && r.URL.Path == "/repos/owner/repo/releases/assets/10":
			return textResponse(http.StatusNoContent, ""), nil
		case r.Method == http.MethodPost && r.URL.Host == "uploads.github.test" && r.URL.Path == "/upload/assets":
			uploaded = append(uploaded, r.URL.Query().Get("name")+":"+r.Header.Get("Content-Type"))
			return textResponse(http.StatusCreated, ""), nil
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	withRunEnvironment(t, "https://api.github.test", []string{
		"releasegithub",
		"-tag", "v1.2.3",
		"-body", "manual notes",
		"-generate-notes=true",
		"-make-latest=true",
		"-target", "abc123",
		archive,
		checksum,
	}, transport, func() {
		if err := run(); err != nil {
			t.Fatal(err)
		}
	})

	if createPayload.TagName != "v1.2.3" || createPayload.TargetCommitish != "abc123" {
		t.Fatalf("create tag/target got %#v", createPayload)
	}
	if createPayload.Name != "qdecimal v1.2.3" {
		t.Fatalf("create name got %q", createPayload.Name)
	}
	if createPayload.Body != "manual notes\n\ngenerated notes" {
		t.Fatalf("create body got %q", createPayload.Body)
	}
	if createPayload.MakeLatest != "true" || createPayload.Prerelease {
		t.Fatalf("create release flags got %#v", createPayload)
	}
	wantUploads := []string{
		filepath.Base(archive) + ":application/gzip",
		filepath.Base(checksum) + ":text/plain; charset=utf-8",
	}
	if !reflect.DeepEqual(uploaded, wantUploads) {
		t.Fatalf("uploaded assets got %#v want %#v", uploaded, wantUploads)
	}
	wantCalls := []string{
		"POST /repos/owner/repo/releases/generate-notes",
		"GET /repos/owner/repo/releases/tags/v1.2.3",
		"POST /repos/owner/repo/releases",
		"GET /repos/owner/repo/releases/99/assets",
		"DELETE /repos/owner/repo/releases/assets/10",
		"POST /upload/assets",
		"POST /upload/assets",
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls got %#v want %#v", calls, wantCalls)
	}
}

func TestRunUpdatesExistingReleaseWithoutImmutableFields(t *testing.T) {
	asset := writeTestAsset(t, "qdecimal-nightly.txt", "nightly")
	var patchPayload releasePayload
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assertGitHubHeaders(t, r)
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/tags/"):
			return jsonResponse(http.StatusOK, release{ID: 77, UploadURL: "https://uploads.github.test/upload/assets{?name,label}"}), nil
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/owner/repo/releases/77":
			decodeRequestJSON(t, r, &patchPayload)
			return jsonResponse(http.StatusOK, release{ID: 77, UploadURL: "https://uploads.github.test/upload/assets{?name,label}"}), nil
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/releases/77/assets":
			return jsonResponse(http.StatusOK, []releaseAsset{}), nil
		case r.Method == http.MethodPost && r.URL.Host == "uploads.github.test" && r.URL.Path == "/upload/assets":
			if r.URL.Query().Get("name") != filepath.Base(asset) {
				t.Fatalf("upload name got %q", r.URL.Query().Get("name"))
			}
			return textResponse(http.StatusCreated, ""), nil
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	withRunEnvironment(t, "https://api.github.test", []string{
		"releasegithub",
		"-tag", "nightly",
		"-name", "qdecimal nightly",
		"-body", "nightly body",
		"-prerelease=true",
		"-make-latest=false",
		asset,
	}, transport, func() {
		if err := run(); err != nil {
			t.Fatal(err)
		}
	})

	if patchPayload.TagName != "" || patchPayload.TargetCommitish != "" {
		t.Fatalf("update should omit immutable fields, got %#v", patchPayload)
	}
	if patchPayload.Name != "qdecimal nightly" || patchPayload.Body != "nightly body" {
		t.Fatalf("update metadata got %#v", patchPayload)
	}
	if !patchPayload.Prerelease || patchPayload.MakeLatest != "false" {
		t.Fatalf("update flags got %#v", patchPayload)
	}
}

func TestReleaseHelperValidationAndSmallHelpers(t *testing.T) {
	if contentTypeFor("source.tar.gz") != "application/gzip" {
		t.Fatal("tar.gz content type mismatch")
	}
	if contentTypeFor("checksums.sha256") != "text/plain; charset=utf-8" {
		t.Fatal("sha256 content type mismatch")
	}
	if contentTypeFor("asset.unknown-extension") != "application/octet-stream" {
		t.Fatal("unknown content type mismatch")
	}
	if got := joinReleaseBody("manual", "generated"); got != "manual\n\ngenerated" {
		t.Fatalf("join body got %q", got)
	}

	withRunEnvironment(t, "https://api.github.test", []string{"releasegithub", "-tag", "v1.0.0", "-make-latest=bad", "asset"}, nil, func() {
		if err := run(); err == nil || !strings.Contains(err.Error(), "invalid -make-latest") {
			t.Fatalf("expected make-latest validation, got %v", err)
		}
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func writeTestAsset(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func withRunEnvironment(t *testing.T, apiURL string, args []string, transport http.RoundTripper, fn func()) {
	t.Helper()
	oldArgs := os.Args
	oldFlags := flag.CommandLine
	oldNewHTTPClient := newHTTPClient
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldFlags
		newHTTPClient = oldNewHTTPClient
	}()

	os.Args = args
	flag.CommandLine = flag.NewFlagSet(args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	if transport != nil {
		newHTTPClient = func() *http.Client {
			return &http.Client{Transport: transport}
		}
	}

	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	t.Setenv("GITHUB_API_URL", apiURL)
	t.Setenv("GITHUB_API_VERSION", "test-version")
	t.Setenv("GITHUB_SHA", "abc123")
	fn()
}

func assertGitHubHeaders(t *testing.T, r *http.Request) {
	t.Helper()
	if r.Header.Get("Authorization") != "Bearer test-token" {
		t.Fatalf("authorization header got %q", r.Header.Get("Authorization"))
	}
	if r.Header.Get("X-GitHub-Api-Version") != "test-version" {
		t.Fatalf("api version header got %q", r.Header.Get("X-GitHub-Api-Version"))
	}
	if r.Header.Get("User-Agent") != "qdecimal-release" {
		t.Fatalf("user agent got %q", r.Header.Get("User-Agent"))
	}
}

func decodeRequestJSON(t *testing.T, r *http.Request, out any) {
	t.Helper()
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		t.Fatal(err)
	}
}

func jsonResponse(status int, value any) *http.Response {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	resp := textResponse(status, string(data))
	resp.Header.Set("Content-Type", "application/json")
	return resp
}

func textResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}
