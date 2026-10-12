// internal/handler/swagger_handler.go
package handler

import (
	"net/http"
	"strings"

	swaggerdocs "bom-zustand-api/docs/swagger"

	"github.com/labstack/echo/v5"
)

const swaggerUITemplate = `<!DOCTYPE html>
<html>
<head>
	<title>bom-zustand-api docs</title>
	<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
	<style>
		#theme-toggle {
			position: fixed;
			top: 10px;
			right: 20px;
			z-index: 10000;
			padding: 6px 14px;
			border-radius: 6px;
			border: 1px solid #888;
			background: #fff;
			color: #333;
			font-size: 13px;
			cursor: pointer;
		}
		html.dark-mode #theme-toggle {
			background: #1c2022;
			color: #e4e6e6;
			border-color: #555;
		}
	</style>
</head>
<body>
	<button id="theme-toggle" onclick="toggleTheme()">☀️ Light mode</button>
	<div id="swagger-ui"></div>
	<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
	<script>
		function applyTheme(theme) {
			document.documentElement.classList.toggle('dark-mode', theme === 'dark');
			document.getElementById('theme-toggle').textContent = theme === 'dark' ? '☀️ Light mode' : '🌙 Dark mode';
		}
		function toggleTheme() {
			const next = document.documentElement.classList.contains('dark-mode') ? 'light' : 'dark';
			localStorage.setItem('swagger-theme', next);
			applyTheme(next);
		}
		// Defaults to dark on first visit; remembers the user's choice after that.
		applyTheme(localStorage.getItem('swagger-theme') || 'dark');

		window.onload = () => {
			window.ui = SwaggerUIBundle({
				url: '/swagger/doc.json',
				dom_id: '#swagger-ui',
			});
		};
	</script>
</body>
</html>`

// SwaggerUI serves the generated OpenAPI document and a Swagger UI page to
// browse it. Mount it at a wildcard route, e.g. e.GET("/swagger/*", handler.SwaggerUI).
func SwaggerUI(c *echo.Context) error {
	path := strings.TrimPrefix(c.Param("*"), "/")

	if path == "doc.json" {
		return c.Blob(http.StatusOK, "application/json", []byte(swaggerdocs.SwaggerInfo.ReadDoc()))
	}

	return c.HTML(http.StatusOK, swaggerUITemplate)
}
