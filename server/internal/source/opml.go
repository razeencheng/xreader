package source

import (
	"encoding/xml"
	"strings"
)

type OPML struct {
	XMLName xml.Name   `xml:"opml"`
	Version string     `xml:"version,attr"`
	Head    OPMLHead   `xml:"head"`
	Body    OPMLBody   `xml:"body"`
}

type OPMLHead struct {
	Title string `xml:"title"`
}

type OPMLBody struct {
	Outlines []OPMLOutline `xml:"outline"`
}

type OPMLOutline struct {
	Text     string        `xml:"text,attr,omitempty"`
	Title    string        `xml:"title,attr,omitempty"`
	Type     string        `xml:"type,attr,omitempty"`
	XMLURL   string        `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string        `xml:"htmlUrl,attr,omitempty"`
	Children []OPMLOutline `xml:"outline,omitempty"`
}

type FlatFeed struct {
	Title  string
	XMLURL string
	Folder string
}

// OPMLFeed is kept as a compatibility alias for earlier task scaffolding.
type OPMLFeed = FlatFeed

func ParseOPML(data []byte) (*OPML, error) {
	var opml OPML
	if err := xml.Unmarshal(data, &opml); err != nil {
		return nil, err
	}
	return &opml, nil
}

func FlattenOPML(opml *OPML) []FlatFeed {
	if opml == nil {
		return nil
	}

	feeds := make([]FlatFeed, 0)
	for _, outline := range opml.Body.Outlines {
		flattenOutline(outline, "", &feeds)
	}
	return feeds
}

func flattenOutline(outline OPMLOutline, folder string, feeds *[]FlatFeed) {
	if outline.XMLURL != "" {
		title := outline.Text
		if title == "" {
			title = outline.Title
		}
		*feeds = append(*feeds, FlatFeed{
			Title:  title,
			XMLURL: outline.XMLURL,
			Folder: folder,
		})
		return
	}

	folderName := outline.Text
	if folderName == "" {
		folderName = outline.Title
	}
	for _, child := range outline.Children {
		flattenOutline(child, folderName, feeds)
	}
}

func GenerateOPML(title string, feeds []FlatFeed) ([]byte, error) {
	folders := make(map[string][]FlatFeed)
	folderOrder := make([]string, 0)
	noFolder := make([]FlatFeed, 0)

	for _, feed := range feeds {
		if strings.TrimSpace(feed.Folder) == "" {
			noFolder = append(noFolder, feed)
			continue
		}
		if _, ok := folders[feed.Folder]; !ok {
			folderOrder = append(folderOrder, feed.Folder)
		}
		folders[feed.Folder] = append(folders[feed.Folder], feed)
	}

	outlines := make([]OPMLOutline, 0, len(feeds))
	for _, folder := range folderOrder {
		children := make([]OPMLOutline, 0, len(folders[folder]))
		for _, feed := range folders[folder] {
			children = append(children, OPMLOutline{
				Text:   feed.Title,
				Title:  feed.Title,
				Type:   "rss",
				XMLURL: feed.XMLURL,
			})
		}
		outlines = append(outlines, OPMLOutline{
			Text:     folder,
			Title:    folder,
			Children: children,
		})
	}

	for _, feed := range noFolder {
		outlines = append(outlines, OPMLOutline{
			Text:   feed.Title,
			Title:  feed.Title,
			Type:   "rss",
			XMLURL: feed.XMLURL,
		})
	}

	opml := OPML{
		Version: "2.0",
		Head:    OPMLHead{Title: title},
		Body:    OPMLBody{Outlines: outlines},
	}

	data, err := xml.MarshalIndent(opml, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), data...), nil
}
