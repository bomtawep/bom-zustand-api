# bom-zustand-api

## Runtime dependencies

PDF report generation (`internal/pdf/chromedp_renderer.go`) renders HTML to PDF
using headless Chrome via [`chromedp`](https://github.com/chromedp/chromedp).
This requires a **Chrome or Chromium binary installed in the runtime
environment** — without one, PDF generation (and tests that exercise it) will
fail or be skipped.

By default, `chromedp` auto-detects common Chrome/Chromium install locations
and binary names (`google-chrome`, `chromium`, etc.) on `PATH`. If Chrome is
installed in a non-default location, set the optional `CHROME_EXEC_PATH`
environment variable to the full path of the Chrome/Chromium executable.
