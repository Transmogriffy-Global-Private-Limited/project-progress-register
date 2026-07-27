// Package openapiv1 owns the authoritative version-one HTTP API contract.
package openapiv1

import (
	"bytes"
	_ "embed"
	"fmt"
	"strconv"
)

//go:embed openapi.yaml
var document []byte

// Document returns a defensive copy of the authoritative OpenAPI document.
func Document() []byte {
	return append([]byte(nil), document...)
}

// DocumentForBasePath returns the authoritative document with only the
// deployment-specific server-variable default resolved. Operation paths and
// every request, response, and error schema remain the committed source bytes.
func DocumentForBasePath(basePath string) ([]byte, error) {
	if basePath == "" {
		return Document(), nil
	}
	defaultLine := []byte("        default: /")
	if bytes.Count(document, defaultLine) != 1 {
		return nil, fmt.Errorf("OpenAPI basePath server default marker is missing or ambiguous")
	}
	replacement := []byte("        default: " + strconv.Quote(basePath))
	return bytes.Replace(document, defaultLine, replacement, 1), nil
}
