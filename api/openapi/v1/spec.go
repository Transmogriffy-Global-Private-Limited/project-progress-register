// Package openapiv1 owns the authoritative version-one HTTP API contract.
package openapiv1

import _ "embed"

//go:embed openapi.yaml
var document []byte

// Document returns a defensive copy of the authoritative OpenAPI document.
func Document() []byte {
	return append([]byte(nil), document...)
}
