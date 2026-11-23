// Package api embeds the OpenAPI specification so the server can serve it at /api/openapi.yaml.
package api

import _ "embed"

//go:embed openapi.yaml
var OpenAPISpec []byte
