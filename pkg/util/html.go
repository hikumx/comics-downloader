package util

import (
	"fmt"
	"strings"

	"github.com/anaskhan96/soup"
)

// SafeFind wraps the soup.Find method to provide a panic-safe way to find HTML elements.
// It checks if the found element's pointer is nil and returns an error if it is.
func SafeFind(doc soup.Root, tag string, attrs ...string) (soup.Root, error) {
	findArgs := []string{tag}
	findArgs = append(findArgs, attrs...)

	element := doc.Find(findArgs...)

	if element.Pointer == nil {
		attrStr := strings.Join(attrs, " ")
		return soup.Root{}, fmt.Errorf("HTML element not found: <%s %s>", tag, attrStr)
	}

	return element, nil
}
