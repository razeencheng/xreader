package source

import (
	"context"
	"time"
)

type RawItem struct {
	ExternalID   string
	Link         string
	Title        string
	ContentHTML  string
	PublishedAt  time.Time
	LanguageHint string
}

type SourceMetadata struct {
	Title        string
	IconURL      string
	LanguageHint string
}

type Source struct {
	ID            int64
	URL           string
	NormalizedURL string
	Title         string
	Kind          string
}

type SourceAdapter interface {
	Kind() string
	Fetch(ctx context.Context, src Source) ([]RawItem, error)
	Validate(ctx context.Context, url string) (SourceMetadata, error)
}
