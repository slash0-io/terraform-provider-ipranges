package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/slash0-io/terraform-provider-ipranges/internal/feedschema"
)

const maxFeedBytes = 32 << 20

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// FeedClient reads the published feed. baseURL points at the v1 root (the
// directory containing index.json) and may be http(s):// or file://.
type FeedClient struct {
	baseURL   string
	userAgent string
	http      *http.Client
}

func NewFeedClient(baseURL, userAgent string) *FeedClient {
	return &FeedClient{
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		userAgent: userAgent,
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

type errNotFound struct{ rel string }

func (e errNotFound) Error() string { return e.rel + ": not found in feed" }

func (c *FeedClient) get(ctx context.Context, rel string) ([]byte, error) {
	if root, ok := strings.CutPrefix(c.baseURL, "file://"); ok {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if os.IsNotExist(err) {
			return nil, errNotFound{rel}
		}
		return b, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/"+rel, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errNotFound{rel}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s/%s: %s", c.baseURL, rel, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes))
}

func (c *FeedClient) Index(ctx context.Context) (*feedschema.Index, error) {
	b, err := c.get(ctx, "index.json")
	if err != nil {
		return nil, err
	}
	var idx feedschema.Index
	if err := json.Unmarshal(b, &idx); err != nil {
		return nil, fmt.Errorf("parse index.json: %w", err)
	}
	if idx.SchemaVersion != feedschema.SchemaVersion {
		return nil, fmt.Errorf("feed schema version %d not supported by this provider (want %d)", idx.SchemaVersion, feedschema.SchemaVersion)
	}
	return &idx, nil
}

func (c *FeedClient) Service(ctx context.Context, slug string) (*feedschema.Service, error) {
	if !slugRe.MatchString(slug) {
		return nil, fmt.Errorf("invalid service slug %q", slug)
	}
	b, err := c.get(ctx, "services/"+slug+".json")
	if err != nil {
		return nil, err
	}
	var svc feedschema.Service
	if err := json.Unmarshal(b, &svc); err != nil {
		return nil, fmt.Errorf("parse service %s: %w", slug, err)
	}
	return &svc, nil
}
