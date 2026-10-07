package reportquery

import (
	"testing"

	"bom-zustand-api/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestFilterDeclaredParams_DropsUndeclaredKeyAndKeepsDeclaredKey(t *testing.T) {
	schema := []model.ReportParam{{Name: "status", Type: "string", Required: true}}
	params := map[string]interface{}{
		"status":    "paid",
		"malicious": map[string]interface{}{"$ne": nil},
	}

	filtered := FilterDeclaredParams(schema, params)

	assert.Equal(t, map[string]interface{}{"status": "paid"}, filtered)
}

func TestFilterDeclaredParams_EmptySchemaDropsEverything(t *testing.T) {
	params := map[string]interface{}{"status": "paid", "limit": float64(10)}

	filtered := FilterDeclaredParams(nil, params)

	assert.Empty(t, filtered)
}

func TestFilterDeclaredParams_NilParamsReturnsEmptyMap(t *testing.T) {
	schema := []model.ReportParam{{Name: "status", Type: "string", Required: true}}

	filtered := FilterDeclaredParams(schema, nil)

	assert.Empty(t, filtered)
}
