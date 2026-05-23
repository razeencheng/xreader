package ai

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

type Paragraph struct {
	Index    int    `json:"index"`
	Original string `json:"original"`
}

var blockTags = map[string]struct{}{
	"article": {}, "aside": {}, "blockquote": {}, "details": {}, "div": {},
	"figure": {}, "footer": {}, "h1": {}, "h2": {}, "h3": {}, "h4": {}, "h5": {}, "h6": {},
	"header": {}, "li": {}, "main": {}, "ol": {}, "p": {}, "pre": {}, "section": {}, "table": {}, "ul": {},
}

var wrapperTags = map[string]struct{}{
	"article": {}, "aside": {}, "body": {}, "div": {}, "footer": {}, "header": {}, "main": {}, "ol": {}, "section": {}, "ul": {},
}

var skipEmptyTags = map[string]struct{}{
	"br": {}, "ins": {},
}

func SplitParagraphs(source string) []Paragraph {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil
	}

	root, err := html.Parse(strings.NewReader("<body>" + source + "</body>"))
	if err != nil {
		return splitPlainText(source)
	}

	body := findNode(root, "body")
	if body == nil {
		return splitPlainText(source)
	}

	paragraphs := make([]Paragraph, 0)
	collectParagraphs(body, &paragraphs)
	return paragraphs
}

func splitPlainText(source string) []Paragraph {
	text := normalizeText(source)
	if text == "" {
		return nil
	}
	return []Paragraph{{Index: 0, Original: text}}
}

func collectParagraphs(node *html.Node, paragraphs *[]Paragraph) {
	if node == nil {
		return
	}

	if node.Type == html.TextNode {
		if text := normalizeText(node.Data); text != "" {
			*paragraphs = append(*paragraphs, Paragraph{Index: len(*paragraphs), Original: text})
		}
		return
	}

	if node.Type == html.ElementNode {
		tag := strings.ToLower(node.Data)
		if _, skipIfEmpty := skipEmptyTags[tag]; skipIfEmpty && normalizeText(nodeText(node)) == "" {
			return
		}

		if _, isWrapper := wrapperTags[tag]; isWrapper {
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				collectParagraphs(child, paragraphs)
			}
			return
		}

		if _, isBlock := blockTags[tag]; isBlock {
			if text := normalizeText(nodeText(node)); text != "" {
				*paragraphs = append(*paragraphs, Paragraph{Index: len(*paragraphs), Original: text})
			}
			return
		}

		if text := normalizeText(nodeText(node)); text != "" {
			*paragraphs = append(*paragraphs, Paragraph{Index: len(*paragraphs), Original: text})
		}
		return
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		collectParagraphs(child, paragraphs)
	}
}

func findNode(root *html.Node, tag string) *html.Node {
	if root == nil {
		return nil
	}
	if root.Type == html.ElementNode && root.Data == tag {
		return root
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if found := findNode(child, tag); found != nil {
			return found
		}
	}
	return nil
}

func nodeText(node *html.Node) string {
	var buf bytes.Buffer
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}
		switch n.Type {
		case html.TextNode:
			buf.WriteString(n.Data)
		case html.ElementNode:
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				walk(child)
			}
		}
	}
	walk(node)
	return buf.String()
}

func normalizeText(raw string) string {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}
