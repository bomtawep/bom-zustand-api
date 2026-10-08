package reportquery

import (
	"testing"

	"bom-zustand-api/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateParams_PassesWhenAllRequiredParamsPresentWithCorrectTypes(t *testing.T) {
	schema := []model.ReportParam{
		{Name: "from", Type: "date", Required: true},
		{Name: "limit", Type: "number", Required: false},
	}
	params := map[string]interface{}{
		"from":  "2026-09-01T00:00:00Z",
		"limit": float64(50),
	}

	assert.NoError(t, ValidateParams(schema, params))
}

func TestValidateParams_FailsWhenRequiredParamMissing(t *testing.T) {
	schema := []model.ReportParam{{Name: "from", Type: "date", Required: true}}

	err := ValidateParams(schema, map[string]interface{}{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"from"`)
}

func TestValidateParams_PassesWhenOptionalParamMissing(t *testing.T) {
	schema := []model.ReportParam{{Name: "limit", Type: "number", Required: false}}

	assert.NoError(t, ValidateParams(schema, map[string]interface{}{}))
}

func TestValidateParams_FailsWhenParamTypeDoesNotMatchSchema(t *testing.T) {
	schema := []model.ReportParam{{Name: "limit", Type: "number", Required: true}}

	err := ValidateParams(schema, map[string]interface{}{"limit": "fifty"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "number")
}

func TestValidateParams_FailsWhenDateParamIsNotRFC3339(t *testing.T) {
	schema := []model.ReportParam{{Name: "from", Type: "date", Required: true}}

	err := ValidateParams(schema, map[string]interface{}{"from": "not-a-date"})

	require.Error(t, err)
}

func TestValidateParams_PassesForBoolAndStringTypes(t *testing.T) {
	schema := []model.ReportParam{
		{Name: "active", Type: "bool", Required: true},
		{Name: "status", Type: "string", Required: true},
	}
	params := map[string]interface{}{"active": true, "status": "paid"}

	assert.NoError(t, ValidateParams(schema, params))
}
