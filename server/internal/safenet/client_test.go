package safenet

import (
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr error
	}{
		{"valid https", "https://example.com/feed.xml", nil},
		{"valid http", "http://example.com/rss", nil},
		{"no scheme", "example.com/feed", ErrUnsupportedURL},
		{"ftp scheme", "ftp://example.com/feed", ErrUnsupportedURL},
		{"file scheme", "file:///etc/passwd", ErrUnsupportedURL},
		{"empty host", "http:///path", ErrBlockedAddress},
		{"localhost", "http://localhost/feed", ErrBlockedAddress},
		{"localhost with dot", "http://localhost./feed", ErrBlockedAddress},
		{"subdomain of localhost", "http://evil.localhost/feed", ErrBlockedAddress},
		{"loopback IP", "http://127.0.0.1/feed", ErrBlockedAddress},
		{"loopback IP v6", "http://[::1]/feed", ErrBlockedAddress},
		{"private 10.x", "http://10.0.0.1/feed", ErrBlockedAddress},
		{"private 172.16.x", "http://172.16.0.1/feed", ErrBlockedAddress},
		{"private 192.168.x", "http://192.168.1.1/feed", ErrBlockedAddress},
		{"link-local", "http://169.254.169.254/latest/meta-data", ErrBlockedAddress},
		{"userinfo", "http://admin:pass@example.com/feed", ErrBlockedAddress},
		{"unspecified", "http://0.0.0.0/feed", ErrBlockedAddress},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if tt.wantErr == nil && err != nil {
				t.Errorf("ValidateURL(%q) unexpected error: %v", tt.url, err)
			}
			if tt.wantErr != nil && err == nil {
				t.Errorf("ValidateURL(%q) expected error %v, got nil", tt.url, tt.wantErr)
			}
			if tt.wantErr != nil && err != nil && err != tt.wantErr {
				t.Errorf("ValidateURL(%q) expected %v, got %v", tt.url, tt.wantErr, err)
			}
		})
	}
}

func TestIsBlockedIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		blocked bool
	}{
		{"public ipv4", "8.8.8.8", false},
		{"public ipv4 2", "93.184.216.34", false},
		{"loopback", "127.0.0.1", true},
		{"loopback high", "127.255.255.254", true},
		{"private 10", "10.0.0.1", true},
		{"private 172.16", "172.16.0.1", true},
		{"private 172.31", "172.31.255.255", true},
		{"not private 172.32", "172.32.0.1", false},
		{"private 192.168", "192.168.0.1", true},
		{"link-local", "169.254.169.254", true},
		{"multicast", "224.0.0.1", true},
		{"unspecified", "0.0.0.0", true},
		{"ipv6 loopback", "::1", true},
		{"ipv6 link-local", "fe80::1", true},
		{"ipv6 public", "2001:4860:4860::8888", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, err := parseAddr(tt.ip)
			if err != nil {
				t.Fatalf("failed to parse %q: %v", tt.ip, err)
			}
			got := isBlockedIP(ip)
			if got != tt.blocked {
				t.Errorf("isBlockedIP(%s) = %v, want %v", tt.ip, got, tt.blocked)
			}
		})
	}
}

func TestReadLimited(t *testing.T) {
	t.Run("within limit", func(t *testing.T) {
		data := []byte("hello world")
		r := newBytesReader(data)
		got, err := ReadLimited(r, 100)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "hello world" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		data := []byte("hello world this is too long")
		r := newBytesReader(data)
		_, err := ReadLimited(r, 10)
		if err != ErrResponseTooBig {
			t.Errorf("expected ErrResponseTooBig, got %v", err)
		}
	})

	t.Run("zero limit reads all", func(t *testing.T) {
		data := []byte("unlimited read")
		r := newBytesReader(data)
		got, err := ReadLimited(r, 0)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "unlimited read" {
			t.Errorf("got %q", got)
		}
	})
}

func TestNewClient(t *testing.T) {
	c := NewClient(Options{})
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.Timeout == 0 {
		t.Error("expected non-zero timeout")
	}
}
