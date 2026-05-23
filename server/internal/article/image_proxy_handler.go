package article

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	imageProxyTimeout = 8 * time.Second
	imageProxyMaxBody = 8 * 1024 * 1024
)

type ProxiedImage struct {
	ContentType string
	Body        []byte
}

type ImageProxyHandler struct {
	fetchImage func(context.Context, string) (ProxiedImage, error)
}

func NewImageProxyHandler() *ImageProxyHandler {
	return &ImageProxyHandler{fetchImage: fetchProxiedImage}
}

func (h *ImageProxyHandler) Proxy(c *gin.Context) {
	rawURL := strings.TrimSpace(c.Query("url"))
	if rawURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing url"})
		return
	}

	image, err := h.fetchImage(c.Request.Context(), rawURL)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, errOriginalUnsupportedURL) || errors.Is(err, errOriginalUnsafeURL) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "failed to proxy image"})
		return
	}

	contentType := image.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=86400")
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(image.Body)
}

func fetchProxiedImage(ctx context.Context, rawURL string) (ProxiedImage, error) {
	u, err := parseSafeOriginalURL(rawURL)
	if err != nil {
		return ProxiedImage{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, imageProxyTimeout)
	defer cancel()

	client := &http.Client{
		Timeout: imageProxyTimeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&safeDialer{
				resolver: net.DefaultResolver,
				dialer:   &net.Dialer{Timeout: 3 * time.Second},
			}).DialContext,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			_, err := parseSafeOriginalURL(req.URL.String())
			return err
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return ProxiedImage{}, err
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; xReader/1.0)")
	if u.Host != "" {
		req.Header.Set("Referer", u.Scheme+"://"+u.Host+"/")
	}

	resp, err := client.Do(req)
	if err != nil {
		return ProxiedImage{}, fmt.Errorf("fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ProxiedImage{}, fmt.Errorf("fetch image: status %d", resp.StatusCode)
	}

	body, err := readLimited(resp.Body, imageProxyMaxBody)
	if err != nil {
		return ProxiedImage{}, err
	}

	contentType, err := normalizeProxiedImageContentType(resp.Header.Get("Content-Type"), body)
	if err != nil {
		return ProxiedImage{}, err
	}

	return ProxiedImage{ContentType: contentType, Body: body}, nil
}

func normalizeProxiedImageContentType(header string, body []byte) (string, error) {
	contentType := strings.ToLower(strings.TrimSpace(header))
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = mediaType
	}

	if strings.HasPrefix(contentType, "image/svg") {
		return "", fmt.Errorf("fetch image: unsupported content type %s", contentType)
	}
	if strings.HasPrefix(contentType, "image/") {
		return contentType, nil
	}

	if contentType != "" && contentType != "application/octet-stream" && contentType != "binary/octet-stream" {
		return "", fmt.Errorf("fetch image: unsupported content type %s", contentType)
	}

	sniffed := strings.ToLower(http.DetectContentType(body))
	if strings.HasPrefix(sniffed, "image/svg") {
		return "", fmt.Errorf("fetch image: unsupported content type %s", sniffed)
	}
	if strings.HasPrefix(sniffed, "image/") {
		return sniffed, nil
	}

	if len(body) >= 12 && string(body[0:4]) == "RIFF" && string(body[8:12]) == "WEBP" {
		return "image/webp", nil
	}

	return "", fmt.Errorf("fetch image: unsupported content type %s", sniffed)
}
