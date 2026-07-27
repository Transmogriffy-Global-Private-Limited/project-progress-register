package openapiv1_test

import (
	"context"
	"testing"

	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/api/openapi/v1"
	"github.com/Transmogriffy-Global-Private-Limited/project-progress-register/internal/httpserver"
	"github.com/getkin/kin-openapi/openapi3"
)

func TestOpenAPIContract(t *testing.T) {
	t.Parallel()

	loader := openapi3.NewLoader()
	document, err := loader.LoadFromData(openapiv1.Document())
	if err != nil {
		t.Fatalf("parse OpenAPI document: %v", err)
	}
	if err := document.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI document: %v", err)
	}
	for _, route := range httpserver.ContractRoutes() {
		pathItem := document.Paths.Find(route.Path)
		if pathItem == nil || pathItem.Get == nil {
			t.Errorf("OpenAPI is missing GET %s", route.Path)
		}
	}
}
