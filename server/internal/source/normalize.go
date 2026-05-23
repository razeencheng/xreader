package source

import (
    "fmt"
    "net/url"
    "sort"
    "strings"
)

func Normalize(raw string) (string, error) {
    parsed, err := url.Parse(raw)
    if err != nil {
        return "", fmt.Errorf("parse url: %w", err)
    }

    parsed.Scheme = strings.ToLower(parsed.Scheme)
    parsed.Host = strings.TrimPrefix(strings.ToLower(parsed.Host), "www.")
    parsed.Fragment = ""
    parsed.Path = normalizePath(parsed.Path)
    parsed.RawQuery = normalizeQuery(parsed.Query())

    if parsed.Path == "/" {
        parsed.Path = ""
    }

    return parsed.String(), nil
}

func normalizePath(path string) string {
    if path == "" {
        return ""
    }

    parts := strings.Split(path, "/")
    cleaned := make([]string, 0, len(parts))
    for _, part := range parts {
        if part == "" {
            continue
        }
        cleaned = append(cleaned, part)
    }

    if len(cleaned) == 0 {
        return "/"
    }

    return "/" + strings.Join(cleaned, "/")
}

func normalizeQuery(values url.Values) string {
    if len(values) == 0 {
        return ""
    }

    filtered := make(url.Values, len(values))
    keys := make([]string, 0, len(values))
    for key := range values {
        if shouldDropQueryParam(key) {
            continue
        }
        keys = append(keys, key)
    }
    sort.Strings(keys)

    for _, key := range keys {
        filtered[key] = append([]string(nil), values[key]...)
    }

    return filtered.Encode()
}

func shouldDropQueryParam(key string) bool {
    return strings.HasPrefix(key, "utm_") || key == "ref" || key == "fbclid"
}
