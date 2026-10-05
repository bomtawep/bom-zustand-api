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
</head>
<body>
	<div id="swagger-ui"></div>
	<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
	<script>
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
