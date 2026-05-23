package source

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"
)

var sanitizer *bluemonday.Policy

func init() {
	sanitizer = bluemonday.UGCPolicy()
	sanitizer.AllowAttrs("class").OnElements("span", "div", "p", "pre", "code")
}

func SanitizeHTML(html string) string {
	return stripHeadingAnchorArtifacts(sanitizer.Sanitize(html))
}

func stripHeadingAnchorArtifacts(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<body>" + html + "</body>"))
	if err != nil {
		return html
	}

	doc.Find("h1, h2, h3, h4, h5, h6").Each(func(_ int, heading *goquery.Selection) {
		heading.Find("a").Each(func(_ int, anchor *goquery.Selection) {
			href, _ := anchor.Attr("href")
			if strings.TrimSpace(anchor.Text()) == "#" && strings.HasPrefix(href, "#") {
				anchor.Remove()
			}
		})

		text := strings.TrimSpace(heading.Text())
		if len([]rune(text)) > 2 && strings.HasSuffix(text, "#") && !strings.HasSuffix(text, "C#") && !strings.HasSuffix(text, "F#") {
			heading.SetText(strings.TrimSpace(strings.TrimSuffix(text, "#")))
		}
	})

	bodyHTML, err := doc.Find("body").Html()
	if err != nil {
		return html
	}
	return bodyHTML
}
