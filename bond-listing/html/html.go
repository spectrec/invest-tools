package html

import (
	"golang.org/x/net/html"
)

func ExtractTextNode(node *html.Node) *html.Node {
	if node.Type == html.TextNode {
		return node
	}

	if node.FirstChild == nil {
		return nil
	}

	return ExtractTextNode(node.FirstChild)
}

func ExtractNodeByPath(root *html.Node, path []string) *html.Node {
	return extractNodeByPathIntl(root, path, 0)
}

func extractNodeByPathIntl(node *html.Node, path []string, depth int) *html.Node {
	if len(path) == depth {
		return node
	}

	for node = node.FirstChild; node != nil; node = node.NextSibling {
		if node.Type != html.ElementNode {
			continue
		}

		if node.Data != path[depth] {
			continue
		}

		if r := extractNodeByPathIntl(node, path, depth+1); r != nil {
			return r
		}
	}

	return nil
}
