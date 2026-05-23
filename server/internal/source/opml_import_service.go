package source

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *SourceService) ImportOPML(ctx context.Context, userID int64, feeds []FlatFeed, jobID string, store JobStore) {
	if store == nil {
		return
	}

	status := JobStatus{Status: "running", Total: len(feeds)}
	store.Set(jobID, status)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		defer func() {
			status.Status = "done"
			store.Set(jobID, status)
		}()

		for _, feed := range feeds {
			if ctx.Err() != nil {
				status.Failed += len(feeds) - status.Succeeded - status.Failed - status.Skipped
				if status.Error == "" {
					status.Error = "import timed out"
				}
				break
			}

			if strings.TrimSpace(feed.XMLURL) == "" {
				status.Failed++
				store.Set(jobID, status)
				continue
			}

			_, err := s.Create(ctx, userID, feed.XMLURL, feed.Folder)
			if err != nil {
				if isUniqueViolation(err) {
					status.Skipped++
				} else {
					status.Failed++
					if status.Error == "" {
						status.Error = err.Error()
					}
				}
			} else {
				status.Succeeded++
			}
			store.Set(jobID, status)
		}
	}()
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key") || strings.Contains(msg, "already exists")
}

func (s *SourceService) ImportOPMLSync(ctx context.Context, userID int64, feeds []OPMLFeed) (JobStatus, error) {
	status := JobStatus{Status: "running", Total: len(feeds)}
	for _, feed := range feeds {
		if strings.TrimSpace(feed.XMLURL) == "" {
			status.Failed++
			continue
		}
		_, err := s.Create(ctx, userID, feed.XMLURL, feed.Folder)
		if err != nil {
			if isUniqueViolation(err) {
				status.Skipped++
			} else {
				status.Failed++
				if status.Error == "" {
					status.Error = err.Error()
				}
			}
		} else {
			status.Succeeded++
		}
	}
	status.Status = "done"
	return status, nil
}

func (s *SourceService) ExportOPML(ctx context.Context, userID int64, title string) ([]byte, error) {
	sources, err := s.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}

	feeds := make([]OPMLFeed, 0, len(sources))
	for _, src := range sources {
		feeds = append(feeds, OPMLFeed{Title: src.Title, XMLURL: src.Url})
	}
	return GenerateOPML(title, feeds)
}
