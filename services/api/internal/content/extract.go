package content

import (
	"bytes"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"strings"
	"time"
	"unicode"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/registry"
)

const extractorVersion = "snap-static-v1"

type Extraction struct {
	Title        string     `json:"title"`
	Author       string     `json:"author"`
	PublishedAt  *time.Time `json:"published_at"`
	BodyText     string     `json:"body_text"`
	Method       string     `json:"method"`
	Characters   int        `json:"characters"`
	SinhalaRatio float64    `json:"sinhala_ratio"`
}

func extractStaticHTML(document []byte, config registry.CollectionConfig) (Extraction, error) {
	root, err := html.Parse(bytes.NewReader(document))
	if err != nil {
		return Extraction{}, fmt.Errorf("parse article HTML: %w", err)
	}
	result := extractJSONLD(root)
	if result.BodyText != "" {
		result.Method = "json_ld"
	}

	if config.TitleSelector != "" {
		if node, err := selectOne(root, config.TitleSelector); err != nil {
			return Extraction{}, fmt.Errorf("title selector: %w", err)
		} else if node != nil {
			result.Title = nodeText(node)
		}
	}
	if config.AuthorSelector != "" {
		if node, err := selectOne(root, config.AuthorSelector); err != nil {
			return Extraction{}, fmt.Errorf("author selector: %w", err)
		} else if node != nil {
			result.Author = nodeText(node)
		}
	}
	if config.PublishedSelector != "" {
		if node, err := selectOne(root, config.PublishedSelector); err != nil {
			return Extraction{}, fmt.Errorf("published selector: %w", err)
		} else if node != nil {
			value := attribute(node, "datetime")
			if value == "" {
				value = attribute(node, "content")
			}
			if value == "" {
				value = nodeText(node)
			}
			result.PublishedAt = parseDate(value)
		}
	}

	var contentNode *html.Node
	if config.ContentSelector != "" {
		contentNode, err = selectOne(root, config.ContentSelector)
		if err != nil {
			return Extraction{}, fmt.Errorf("content selector: %w", err)
		}
		if contentNode == nil {
			return Extraction{}, fmt.Errorf("content selector matched no element")
		}
		result.Method = "css_selector"
	} else if result.BodyText == "" {
		for _, selector := range []string{
			`[itemprop="articleBody"]`, "article", ".entry-content", ".post-content", ".article-content", "main",
		} {
			contentNode, _ = selectOne(root, selector)
			if contentNode != nil {
				result.Method = "generic_static"
				break
			}
		}
	}
	if contentNode != nil {
		for _, selector := range config.ExcludeSelectors {
			matches, selectErr := selectAll(contentNode, selector)
			if selectErr != nil {
				return Extraction{}, fmt.Errorf("exclude selector: %w", selectErr)
			}
			for _, match := range matches {
				detachNode(match)
			}
		}
		removeElements(contentNode, map[string]bool{
			"script": true, "style": true, "noscript": true, "svg": true,
			"form": true, "nav": true, "aside": true, "footer": true,
		})
		result.BodyText = nodeText(contentNode)
	}
	if result.Title == "" {
		if title, _ := selectOne(root, "h1"); title != nil {
			result.Title = nodeText(title)
		}
	}
	result.BodyText = normalizeText(result.BodyText)
	result.Title = normalizeText(result.Title)
	result.Author = normalizeText(result.Author)
	result.Characters = len([]rune(result.BodyText))
	result.SinhalaRatio = sinhalaRatio(result.BodyText)
	if result.BodyText == "" {
		return Extraction{}, fmt.Errorf("article body was not found")
	}
	return result, nil
}

func extractStructuredText(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !strings.Contains(value, "<") {
		return normalizeText(stdhtml.UnescapeString(value)), nil
	}
	root, err := html.Parse(strings.NewReader(value))
	if err != nil {
		return "", fmt.Errorf("parse structured content: %w", err)
	}
	removeElements(root, map[string]bool{
		"script": true, "style": true, "noscript": true, "svg": true,
		"form": true, "nav": true, "aside": true, "footer": true,
	})
	return normalizeText(nodeText(root)), nil
}

func validateExtraction(result Extraction, config registry.CollectionConfig) error {
	minimum := config.MinContentCharacters
	if minimum == 0 {
		minimum = 200
	}
	if result.Characters < minimum {
		return fmt.Errorf("extracted body has %d characters; minimum is %d", result.Characters, minimum)
	}
	if result.SinhalaRatio < config.MinimumSinhalaRatio {
		return fmt.Errorf("extracted body Sinhala ratio %.2f is below %.2f", result.SinhalaRatio, config.MinimumSinhalaRatio)
	}
	return nil
}

func extractJSONLD(root *html.Node) Extraction {
	selector, _ := cascadia.Parse(`script[type="application/ld+json"]`)
	for _, node := range cascadia.QueryAll(root, selector) {
		var payload any
		if err := json.Unmarshal([]byte(rawNodeText(node)), &payload); err != nil {
			continue
		}
		if result, ok := findArticleJSONLD(payload); ok {
			body, _ := extractStructuredText(result.BodyText)
			result.BodyText = body
			return result
		}
	}
	return Extraction{}
}

func findArticleJSONLD(value any) (Extraction, bool) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if result, ok := findArticleJSONLD(item); ok {
				return result, true
			}
		}
	case map[string]any:
		if graph, ok := typed["@graph"]; ok {
			if result, found := findArticleJSONLD(graph); found {
				return result, true
			}
		}
		body, _ := typed["articleBody"].(string)
		if body == "" {
			return Extraction{}, false
		}
		result := Extraction{BodyText: body}
		result.Title, _ = typed["headline"].(string)
		result.Author = jsonLDAuthor(typed["author"])
		if published, _ := typed["datePublished"].(string); published != "" {
			result.PublishedAt = parseDate(published)
		}
		return result, true
	}
	return Extraction{}, false
}

func jsonLDAuthor(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case map[string]any:
		name, _ := typed["name"].(string)
		return name
	case []any:
		names := make([]string, 0, len(typed))
		for _, item := range typed {
			if name := jsonLDAuthor(item); name != "" {
				names = append(names, name)
			}
		}
		return strings.Join(names, ", ")
	}
	return ""
}

func selectOne(root *html.Node, expression string) (*html.Node, error) {
	selector, err := cascadia.Parse(expression)
	if err != nil {
		return nil, err
	}
	return cascadia.Query(root, selector), nil
}

func selectAll(root *html.Node, expression string) ([]*html.Node, error) {
	selector, err := cascadia.Parse(expression)
	if err != nil {
		return nil, err
	}
	return cascadia.QueryAll(root, selector), nil
}

func removeElements(root *html.Node, names map[string]bool) {
	for child := root.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == html.ElementNode && names[strings.ToLower(child.Data)] {
			detachNode(child)
		} else {
			removeElements(child, names)
		}
		child = next
	}
}

func detachNode(node *html.Node) {
	if node.Parent != nil {
		node.Parent.RemoveChild(node)
	}
}

func rawNodeText(node *html.Node) string {
	var builder strings.Builder
	var visit func(*html.Node)
	visit = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(node)
	return builder.String()
}

func nodeText(node *html.Node) string {
	var builder strings.Builder
	var visit func(*html.Node)
	visit = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
			builder.WriteByte(' ')
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
		if current.Type == html.ElementNode && blockElement(current.Data) {
			builder.WriteByte('\n')
		}
	}
	visit(node)
	return builder.String()
}

func blockElement(name string) bool {
	switch strings.ToLower(name) {
	case "p", "div", "section", "article", "main", "li", "br", "h1", "h2", "h3", "h4", "blockquote":
		return true
	default:
		return false
	}
}

func normalizeText(value string) string {
	lines := strings.Split(strings.ReplaceAll(stdhtml.UnescapeString(value), "\u00a0", " "), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			result = append(result, line)
		}
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}

func sinhalaRatio(value string) float64 {
	var letters, sinhala int
	for _, character := range value {
		if unicode.IsLetter(character) {
			letters++
			if character >= '\u0D80' && character <= '\u0DFF' {
				sinhala++
			}
		}
	}
	if letters == 0 {
		return 0
	}
	return float64(sinhala) / float64(letters)
}

func attribute(node *html.Node, name string) string {
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, name) {
			return strings.TrimSpace(attribute.Val)
		}
	}
	return ""
}

func parseDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			parsed = parsed.UTC()
			return &parsed
		}
	}
	return nil
}
