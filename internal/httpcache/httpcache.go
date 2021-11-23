package httpcache

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"github.com/diamondburned/go-lovense/api"
	"github.com/diamondburned/go-lovense/pattern"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/pkg/errors"
)

// Path is the cache path.
var Path = filepath.Join(os.TempDir(), "intiface-gtk", "cache")

// Client is the cached client.
var Client = http.Client{
	Transport: httpcache.NewTransport(diskcache.New(Path)),
}

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
