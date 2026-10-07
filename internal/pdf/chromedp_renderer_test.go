package pdf

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chromeAvailable() bool {
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
}

func TestChromedpRenderer_RenderHTML_ReturnsValidPDFBytes(t *testing.T) {
	if !chromeAvailable() {
		t.Skip("no Chrome/Chromium binary found on PATH")
	}

	r := NewChromedpRenderer("")
	defer r.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pdfBytes, err := r.RenderHTML(ctx, "<html><body><h1>hello</h1></body></html>")

	require.NoError(t, err)
	require.True(t, len(pdfBytes) > 4)
	assert.Equal(t, "%PDF", string(pdfBytes[:4]))
}
