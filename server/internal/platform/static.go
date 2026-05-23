package platform

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// NewSPAHandler returns an http.Handler that serves static files from the given
// filesystem and falls back to index.html for SPA-style client-side routing.
//
// It avoids http.FileServer for HTML files to prevent the standard library's
// index.html → / redirect which causes loops in SPA setups.
//
// gaID, when non-empty, is a Google Analytics measurement ID. The matching
// gtag.js snippet is injected into every served HTML document at runtime, so
// analytics can be toggled via an environment variable without rebuilding the
// embedded frontend. Empty gaID means no script is ever injected.
func NewSPAHandler(staticFS fs.FS, gaID string) http.Handler {
	fileServer := http.FileServer(http.FS(staticFS))
	gaSnippet := gaTag(gaID)

	serveFile := func(w http.ResponseWriter, name string) {
		f, err := staticFS.Open(name)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		defer f.Close()

		isHTML := strings.HasSuffix(name, ".html")

		// HTML responses get the gtag snippet injected before </head>. Read the
		// document fully so Content-Length reflects the rewritten body.
		if isHTML && gaSnippet != nil {
			data, err := io.ReadAll(f)
			if err != nil {
				http.Error(w, "read error", http.StatusInternalServerError)
				return
			}
			data = injectBeforeHeadClose(data, gaSnippet)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			return
		}

		stat, _ := f.Stat()

		ct := "application/octet-stream"
		switch {
		case isHTML:
			ct = "text/html; charset=utf-8"
		case strings.HasSuffix(name, ".js"):
			ct = "application/javascript"
		case strings.HasSuffix(name, ".css"):
			ct = "text/css"
		case strings.HasSuffix(name, ".json"):
			ct = "application/json"
		}
		w.Header().Set("Content-Type", ct)
		if isHTML {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}
		if stat != nil {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
		}
		w.WriteHeader(http.StatusOK)
		io.Copy(w, f)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")

		// Root path → index.html
		if urlPath == "" || urlPath == "." {
			serveFile(w, "index.html")
			return
		}

		// Try exact file (skip directories)
		if f, err := staticFS.Open(urlPath); err == nil {
			stat, _ := f.Stat()
			f.Close()
			if stat != nil && !stat.IsDir() {
				// Use fileServer for non-HTML assets (proper caching headers)
				if !strings.HasSuffix(urlPath, ".html") {
					fileServer.ServeHTTP(w, r)
					return
				}
				serveFile(w, urlPath)
				return
			}
		}

		// Try .html extension (Next.js static export: /login → login.html)
		htmlPath := urlPath + ".html"
		if f, err := staticFS.Open(htmlPath); err == nil {
			f.Close()
			serveFile(w, htmlPath)
			return
		}

		// Try path/index.html (/admin → admin/index.html)
		indexPath := urlPath + "/index.html"
		if f, err := staticFS.Open(indexPath); err == nil {
			f.Close()
			serveFile(w, indexPath)
			return
		}

		// SPA fallback: serve root index.html for client-side routing
		serveFile(w, "index.html")
	})
}

// gaTag builds the Google Analytics gtag.js snippet for the given measurement
// ID. It returns nil when the ID is empty or contains characters outside the
// expected GA/GTM alphabet, so a malformed env value can never break the markup
// or smuggle script into the page.
func gaTag(gaID string) []byte {
	if !isValidGAID(gaID) {
		return nil
	}
	s := `<!-- Google tag (gtag.js) -->` +
		`<script async src="https://www.googletagmanager.com/gtag/js?id=` + gaID + `"></script>` +
		`<script>window.dataLayer=window.dataLayer||[];function gtag(){dataLayer.push(arguments);}` +
		`gtag('js',new Date());gtag('config','` + gaID + `');</script>`
	return []byte(s)
}

// isValidGAID reports whether id looks like a Google measurement/container ID
// (e.g. G-XXXXXXXXXX, UA-1234-5, GTM-XXXX): non-empty and limited to letters,
// digits, and dashes.
func isValidGAID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'A' && r <= 'Z',
			r >= 'a' && r <= 'z',
			r >= '0' && r <= '9',
			r == '-':
		default:
			return false
		}
	}
	return true
}

// injectBeforeHeadClose inserts snippet immediately before the first </head> in
// html. If no </head> is present the document is returned unchanged.
func injectBeforeHeadClose(html, snippet []byte) []byte {
	marker := []byte("</head>")
	idx := bytes.Index(html, marker)
	if idx < 0 {
		return html
	}
	out := make([]byte, 0, len(html)+len(snippet))
	out = append(out, html[:idx]...)
	out = append(out, snippet...)
	out = append(out, html[idx:]...)
	return out
}
