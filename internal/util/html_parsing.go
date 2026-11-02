package util

import (
	"strings"

	"golang.org/x/net/html"
)

func ExtractHTMLContainerByAttr(rawHtml string, target string, value string) string {
	doc, err := html.Parse(strings.NewReader(rawHtml))

	if err != nil {
		return ""
	}

	var find func(*html.Node) *html.Node

	find = func(node *html.Node) *html.Node {
		if node.Type == html.ElementNode {
			for _, attr := range node.Attr {
				if attr.Key == target && attr.Val == value {
					return node
				}
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if found := find(child); found != nil {
				return found
			}
		}

		return nil
	}

	node := find(doc)

	if node == nil {
		return ""
	}

	var result strings.Builder
	html.Render(&result, node)

	return result.String()
}
