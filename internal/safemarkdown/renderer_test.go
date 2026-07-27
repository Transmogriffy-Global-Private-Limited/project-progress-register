package safemarkdown

import (
	"strings"
	"testing"
)

func TestRendererAllowsMarkdownAndRemovesUnsafeContent(t *testing.T) {
	t.Parallel()
	rendered, err := New().Render("**safe** [bad](javascript:alert(1)) <script>alert(1)</script>")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "<strong>safe</strong>") || strings.Contains(strings.ToLower(rendered), "javascript:") || strings.Contains(strings.ToLower(rendered), "<script") {
		t.Fatalf("rendered unsafe HTML: %s", rendered)
	}
}
