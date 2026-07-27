package openapiv1_test

import (
	"context"
	"net/http"
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
		var present bool
		if pathItem != nil {
			if route.Method == http.MethodGet {
				present = pathItem.Get != nil
			} else if route.Method == http.MethodPost {
				present = pathItem.Post != nil
			} else if route.Method == http.MethodPatch {
				present = pathItem.Patch != nil
			} else if route.Method == http.MethodPut {
				present = pathItem.Put != nil
			} else if route.Method == http.MethodDelete {
				present = pathItem.Delete != nil
			}
		}
		if !present {
			t.Errorf("OpenAPI is missing %s %s", route.Method, route.Path)
		}
	}
}
