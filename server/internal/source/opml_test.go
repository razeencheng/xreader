package source

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const testOPML = `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head><title>Test Feeds</title></head>
  <body>
    <outline text="Tech" title="Tech">
      <outline text="Hacker News" title="Hacker News" type="rss" xmlUrl="https://news.ycombinator.com/rss" htmlUrl="https://news.ycombinator.com"/>
      <outline text="Lobsters" title="Lobsters" type="rss" xmlUrl="https://lobste.rs/rss"/>
    </outline>
    <outline text="Blog" title="Blog" type="rss" xmlUrl="https://example.com/blog/feed"/>
  </body>
</opml>`

func TestOPML_ParseNestedFolders(t *testing.T) {
	opml, err := ParseOPML([]byte(testOPML))
	require.NoError(t, err)
	require.Equal(t, "Test Feeds", opml.Head.Title)

	feeds := FlattenOPML(opml)
	require.Len(t, feeds, 3)

	require.Equal(t, "Hacker News", feeds[0].Title)
	require.Equal(t, "Tech", feeds[0].Folder)
	require.Equal(t, "https://news.ycombinator.com/rss", feeds[0].XMLURL)

	require.Equal(t, "Lobsters", feeds[1].Title)
	require.Equal(t, "Tech", feeds[1].Folder)

	require.Equal(t, "Blog", feeds[2].Title)
	require.Equal(t, "", feeds[2].Folder)
}

func TestOPMLExport_RoundtripsWithImport(t *testing.T) {
	opml, err := ParseOPML([]byte(testOPML))
	require.NoError(t, err)
	feeds := FlattenOPML(opml)

	exported, err := GenerateOPML("Test Feeds", feeds)
	require.NoError(t, err)

	opml2, err := ParseOPML(exported)
	require.NoError(t, err)
	feeds2 := FlattenOPML(opml2)
	require.Len(t, feeds2, 3)

	urlSet := make(map[string]bool)
	for _, f := range feeds {
		urlSet[f.XMLURL] = true
	}
	for _, f := range feeds2 {
		require.True(t, urlSet[f.XMLURL], "missing URL in roundtrip: %s", f.XMLURL)
	}
}
