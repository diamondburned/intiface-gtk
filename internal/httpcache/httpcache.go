package httpcache

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/diamondburned/go-lovense/api"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
)

// Path is the cache path.
var Path = filepath.Join(os.TempDir(), "intiface-gtk", "cache")

// Client is the cached client.
var Client = http.Client{
	Transport: httpcache.NewTransport(diskcache.New(Path)),
}

// DownloadPatternBytes downloads the given pattern into bytes. It does not
// parse it.
func DownloadPatternBytes(ctx context.Context, apiPattern *api.Pattern) ([]byte, error) {
	q, err := http.NewRequestWithContext(ctx, "GET", apiPattern.CDNPath, nil)
	if err != nil {
		return nil, err
	}

	r, err := Client.Do(q)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r.Body); err != nil {
		return nil, fmt.Errorf("cannot download request: %w", err)
	}

	return buf.Bytes(), nil
}

/*
// DownloadPattern downloads the givne pattern and parses it.
func DownloadPattern(ctx context.Context, apiPattern *api.Pattern) (*pattern.Pattern, error) {
	q, err := http.NewRequestWithContext(ctx, "GET", apiPattern.CDNPath, nil)
	if err != nil {
		return nil, err
	}

	r, err := Client.Do(q)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	p, err := pattern.Parse(r.Body)
	if err != nil {
		return nil, errors.Wrap(err, "invalid pattern")
	}

	return p, nil
}
*/
