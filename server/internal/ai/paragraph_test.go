package ai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitParagraphs_BasicHTML(t *testing.T) {
	paragraphs := SplitParagraphs("<p>First</p><p>Second</p>")
	require.Len(t, paragraphs, 2)
	require.Equal(t, 0, paragraphs[0].Index)
	require.Equal(t, "First", paragraphs[0].Original)
	require.Equal(t, 1, paragraphs[1].Index)
	require.Equal(t, "Second", paragraphs[1].Original)
}

func TestSplitParagraphs_SkipsEmpty(t *testing.T) {
	paragraphs := SplitParagraphs("<p></p><p>Content</p>")
	require.Len(t, paragraphs, 1)
	require.Equal(t, 0, paragraphs[0].Index)
	require.Equal(t, "Content", paragraphs[0].Original)
}

func TestSplitParagraphs_SplitsListItems(t *testing.T) {
	source := "<p>Intro</p><ul><li>One</li><li>Two</li></ul><p>Outro</p>"
	paragraphs := SplitParagraphs(source)
	require.Len(t, paragraphs, 4)
	require.Equal(t, "Intro", paragraphs[0].Original)
	require.Equal(t, "One", paragraphs[1].Original)
	require.Equal(t, "Two", paragraphs[2].Original)
	require.Equal(t, "Outro", paragraphs[3].Original)
}
