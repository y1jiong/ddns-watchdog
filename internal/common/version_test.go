package common

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const testUserAgent = "ddns-watchdog-test/" + Version

func TestLatestVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The version service keys its answer on the agent: an unknown one gets
		// "426 Upgrade Required" even though the client is current.
		if got := r.Header.Get("User-Agent"); got != testUserAgent {
			t.Errorf("User-Agent = %q, want %q", got, testUserAgent)
		}
		_, _ = fmt.Fprint(w, `{"ip":"203.0.113.7","latest_version":"9.9.9"}`)
	}))
	defer srv.Close()

	got, err := LatestVersion(srv.URL, testUserAgent)
	if err != nil {
		t.Fatalf("LatestVersion: %v", err)
	}
	if got != "9.9.9" {
		t.Errorf("LatestVersion = %q, want 9.9.9", got)
	}
}

// A timeout is reported as a timeout.
func TestLatestVersionTimeoutIsNamed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	_, err := fetchLatestVersion(srv.URL, testUserAgent, 50*time.Millisecond)
	if err == nil {
		t.Fatal("LatestVersion succeeded against a server that never replies")
	}
	if want := "timed out after 50ms"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not mention %q", err, want)
	}
}

func TestLatestVersionReportsResponseProblems(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{
			name:    "http error status",
			status:  http.StatusServiceUnavailable,
			body:    `{"message":"maintenance"}`,
			wantErr: "HTTP 503 Service Unavailable",
		},
		{
			name:    "invalid json",
			status:  http.StatusOK,
			body:    `{`,
			wantErr: "invalid JSON response",
		},
		{
			name:    "missing version field",
			status:  http.StatusOK,
			body:    `{"ip":"203.0.113.7"}`,
			wantErr: "latest_version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = fmt.Fprint(w, tt.body)
			}))
			defer srv.Close()

			_, err := LatestVersion(srv.URL, testUserAgent)
			if err == nil {
				t.Fatal("LatestVersion succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not mention %q", err, tt.wantErr)
			}
		})
	}
}

// lineSignalWriter closes threeLines once three lines have been written.
type lineSignalWriter struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	lines    int
	three    chan struct{}
	threeOne sync.Once
}

func newLineSignalWriter() *lineSignalWriter {
	return &lineSignalWriter{three: make(chan struct{})}
}

func (w *lineSignalWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n, err := w.buf.Write(p)
	w.lines += bytes.Count(p, []byte("\n"))
	if w.lines >= 3 {
		w.threeOne.Do(func() { close(w.three) })
	}
	return n, err
}

func (w *lineSignalWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// The local build information must be printed while the lookup is still in
// flight, otherwise -V blocks on the network before showing anything.
func TestVersionTipsPrintsLocalInfoBeforeLookupCompletes(t *testing.T) {
	w := newLineSignalWriter()
	fetch := func() (string, error) {
		select {
		case <-w.three:
			return "9.9.9", nil
		case <-time.After(2 * time.Second):
			return "", errors.New("local build info was not printed before the lookup")
		}
	}

	printVersionTips(w, "https://version.example.com", fetch)

	out := w.String()
	for _, want := range []string{
		"current version: " + Version,
		"git commit: ",
		"build time: ",
		"checking latest version at https://version.example.com ...",
		"latest version: 9.9.9",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain %q", out, want)
		}
	}
}

// The default endpoint is used so often that echoing it only adds noise.
func TestVersionTipsHidesDefaultAPIURL(t *testing.T) {
	var buf bytes.Buffer
	printVersionTips(&buf, DefaultAPIUrl, func() (string, error) {
		return Version, nil
	})

	out := buf.String()
	if !strings.Contains(out, "checking latest version ...") {
		t.Errorf("output %q does not contain the URL-less status line", out)
	}
	if strings.Contains(out, "checking latest version at") {
		t.Errorf("output %q echoes the default API URL", out)
	}
}

// A failed lookup reports the concrete reason.
func TestVersionTipsReportsLookupError(t *testing.T) {
	var buf bytes.Buffer
	printVersionTips(&buf, "https://version.example.com", func() (string, error) {
		return "", errors.New("request timed out after 10s")
	})

	out := buf.String()
	for _, want := range []string{
		"latest version: unavailable",
		"error: request timed out after 10s",
		"check for updates manually at " + projectUrl,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain %q", out, want)
		}
	}
}

// A non-root server answers "N/A (...)" when its own upstream is down; that is a
// failed lookup too, not a version number.
func TestVersionTipsTreatsPlaceholderAsFailure(t *testing.T) {
	var buf bytes.Buffer
	printVersionTips(&buf, "https://version.example.com", func() (string, error) {
		return "N/A (upstream unavailable)", nil
	})

	out := buf.String()
	for _, want := range []string{
		"latest version: unavailable",
		"error: N/A (upstream unavailable)",
		"check for updates manually at " + projectUrl,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain %q", out, want)
		}
	}
}
