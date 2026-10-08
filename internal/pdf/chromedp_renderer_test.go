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

func TestChromedpRenderer_RenderHTML_AbortsPromptlyWhenCallerContextIsCancelled(t *testing.T) {
	if !chromeAvailable() {
		t.Skip("no Chrome/Chromium binary found on PATH")
	}

	r := NewChromedpRenderer("")
	defer r.Close()

	// Wrap the whole call in a 2s test-level timeout to prove the render
	// aborts promptly on caller-context cancellation rather than running
	// to the hardcoded 30s upper bound.
	testCtx, testCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer testCancel()

	callerCtx, callerCancel := context.WithCancel(context.Background())
	callerCancel() // already cancelled before the call starts

	done := make(chan struct{})
	var err error
	go func() {
		_, err = r.RenderHTML(callerCtx, "<html><body><h1>hello</h1></body></html>")
		close(done)
	}()

	select {
	case <-done:
		require.Error(t, err, "RenderHTML should return an error when the caller's context is already cancelled")
	case <-testCtx.Done():
		t.Fatal("RenderHTML did not return promptly after caller context cancellation; it appears to have ignored ctx and run to the full 30s timeout")
	}
}
