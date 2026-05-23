package source

import (
    "testing"

    "github.com/stretchr/testify/require"
)

func TestNormalize(t *testing.T) {
    cases := []struct{ name, in, want string }{
        {"lowercase scheme+host", "HTTPS://Example.COM/Path/?utm_source=x", "https://example.com/Path"},
        {"strip ref param keep others", "https://example.com/a?ref=foo&x=1", "https://example.com/a?x=1"},
        {"collapse double slashes", "https://example.com//a//b/", "https://example.com/a/b"},
        {"strip fragment", "https://example.com/path#frag", "https://example.com/path"},
        {"strip trailing slash", "https://example.com/path/", "https://example.com/path"},
        {"strip utm_medium", "https://example.com/p?utm_medium=email&q=1", "https://example.com/p?q=1"},
        {"strip fbclid", "https://example.com/p?fbclid=abc123", "https://example.com/p"},
        {"no-op clean URL", "https://example.com/clean", "https://example.com/clean"},
        {"strip www prefix", "https://www.v2ex.com/t/1234567", "https://v2ex.com/t/1234567"},
        {"strip www and fragment", "https://www.v2ex.com/t/1234567#reply3", "https://v2ex.com/t/1234567"},
        {"no www stays the same", "https://v2ex.com/t/1234567", "https://v2ex.com/t/1234567"},
        {"www with port preserved", "https://www.example.com:8080/path", "https://example.com:8080/path"},
        {"root path", "https://example.com/", "https://example.com"},
        {"multiple utm params", "https://example.com/p?utm_source=a&utm_medium=b&utm_campaign=c&real=1", "https://example.com/p?real=1"},
    }
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            got, err := Normalize(c.in)
            require.NoError(t, err)
            require.Equal(t, c.want, got, "input=%s", c.in)
        })
    }
}

func TestNormalize_InvalidURL(t *testing.T) {
    _, err := Normalize("://not-a-url")
    require.Error(t, err)
}
