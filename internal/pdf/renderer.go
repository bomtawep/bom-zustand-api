package pdf

import "context"

// Renderer converts a complete HTML document into PDF bytes.
type Renderer interface {
	RenderHTML(ctx context.Context, html string) ([]byte, error)
}
