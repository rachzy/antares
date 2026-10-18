// Package api embeds the OpenAPI document of the Antares public HTTP API.
package api

import _ "embed"

// OpenAPI is the OpenAPI 3.1 document, served at /openapi.yaml.
//
//go:embed openapi.yaml
var OpenAPI []byte
