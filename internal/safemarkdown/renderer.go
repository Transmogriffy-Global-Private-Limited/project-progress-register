// Package safemarkdown renders stored Markdown into allowlist-sanitized HTML.
package safemarkdown

import (
	"bytes"
	"fmt"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// Renderer is immutable after construction and safe to share across services.
type Renderer struct {
	markdown goldmark.Markdown
	policy   *bluemonday.Policy
}

func New() *Renderer {
	return &Renderer{
		markdown: goldmark.New(goldmark.WithExtensions(extension.GFM)),
		policy:   bluemonday.UGCPolicy(),
	}
}

// Render preserves Markdown as the caller's source of truth and returns only sanitized HTML.
func (r *Renderer) Render(source string) (string, error) {
	var rendered bytes.Buffer
	if err := r.markdown.Convert([]byte(source), &rendered); err != nil {
		return "", fmt.Errorf("render Markdown: %w", err)
	}
	return string(r.policy.SanitizeBytes(rendered.Bytes())), nil
}
