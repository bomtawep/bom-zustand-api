package reportquery

import (
	"fmt"
	"time"

	"bom-zustand-api/internal/model"
)

// ValidateParams checks that params satisfies schema: every required param
// is present, and every present param's decoded JSON type matches its
// declared schema type ("string", "number", "bool", or "date" — an
// RFC3339 string).
func ValidateParams(schema []model.ReportParam, params map[string]interface{}) error {
	for _, p := range schema {
		v, ok := params[p.Name]
		if !ok {
			if p.Required {
				return fmt.Errorf("missing required param %q", p.Name)
			}
			continue
		}
		if err := checkParamType(p, v); err != nil {
			return err
		}
	}
	return nil
}

func checkParamType(p model.ReportParam, v interface{}) error {
	switch p.Type {
	case "string":
		if _, ok := v.(string); !ok {
			return fmt.Errorf("param %q must be a string", p.Name)
		}
	case "number":
		if _, ok := v.(float64); !ok {
			return fmt.Errorf("param %q must be a number", p.Name)
		}
	case "bool":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("param %q must be a bool", p.Name)
		}
	case "date":
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("param %q must be an RFC3339 date string", p.Name)
		}
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			return fmt.Errorf("param %q must be an RFC3339 date string: %w", p.Name, err)
		}
	default:
		return fmt.Errorf("param %q has unknown schema type %q", p.Name, p.Type)
	}
	return nil
}
