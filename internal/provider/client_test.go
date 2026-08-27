package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFeed lays out a minimal v1 tree on disk and returns a file:// base URL.
func writeFeed(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return "file://" + root
}

const stripeJSON = `{
  "schemaVersion": 1, "slug": "stripe", "name": "Stripe",
  "classification": "dedicated", "generatedAt": "2026-08-27T00:00:00Z",
  "syncToken": "1787", "sources": [],
  "purposes": {
    "api":      {"direction": "egress",  "ipv4": ["192.0.2.0/24"], "ipv6": []},
    "webhooks": {"direction": "ingress", "ipv4": ["198.51.100.0/24"], "ipv6": ["2001:db8::/32"]},
    "terminal": {"direction": "egress",  "ipv4": ["203.0.113.0/24"], "ipv6": []}
  }
}`

const indexJSON = `{"schemaVersion":1,"generatedAt":"2026-08-27T00:00:00Z","syncToken":"1787",
  "services":[{"slug":"stripe","name":"Stripe","path":"services/stripe.json","sha256":""}]}`

func testClient(t *testing.T) *FeedClient {
	return NewFeedClient(writeFeed(t, map[string]string{
		"index.json":           indexJSON,
		"services/stripe.json": stripeJSON,
	}), "test")
}

// A slug is interpolated straight into a path, so it has to be validated
// before it can climb out of the feed root.
func TestServiceRejectsPathTraversal(t *testing.T) {
	c := testClient(t)
	for _, bad := range []string{"../../etc/passwd", "a/b", "..", "Stripe", "-lead", ""} {
		if _, err := c.Service(context.Background(), bad); err == nil {
			t.Fatalf("slug %q was accepted; it must be rejected before becoming a path", bad)
		}
	}
}

// A missing service is a distinct error the data source turns into "unknown
// service, here is the catalog" rather than a generic fetch failure.
func TestServiceMissingIsNotFound(t *testing.T) {
	c := testClient(t)
	_, err := c.Service(context.Background(), "nosuch")
	var nf errNotFound
	if !errors.As(err, &nf) {
		t.Fatalf("missing service error = %T (%v), want errNotFound", err, err)
	}
}

// The provider vendors a frozen schema version. A feed that moves past it must
// say so plainly instead of silently decoding into zero values.
func TestIndexRejectsUnsupportedSchemaVersion(t *testing.T) {
	base := writeFeed(t, map[string]string{
		"index.json": `{"schemaVersion":99,"services":[]}`,
	})
	_, err := NewFeedClient(base, "test").Index(context.Background())
	if err == nil {
		t.Fatal("a future schema version must be refused")
	}
	if !strings.Contains(err.Error(), "schema version") {
		t.Fatalf("error should name the schema version, got %v", err)
	}
}

// An HTTP feed that answers with a server error must surface it rather than
// being read as an empty catalog.
func TestHTTPErrorIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := NewFeedClient(srv.URL, "test").Index(context.Background()); err == nil {
		t.Fatal("a 500 must be an error")
	}
}

// 404 over HTTP maps to the same not-found signal as a missing file, so the
// data source's messaging does not depend on the transport.
func TestHTTPNotFoundMapsToErrNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(http.NotFound))
	defer srv.Close()
	_, err := NewFeedClient(srv.URL, "test").Service(context.Background(), "stripe")
	var nf errNotFound
	if !errors.As(err, &nf) {
		t.Fatalf("http 404 = %T (%v), want errNotFound", err, err)
	}
}
