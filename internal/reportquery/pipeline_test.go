package reportquery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestBuildPipeline_SubstitutesStringParamAsQuotedJSON(t *testing.T) {
	tmpl := `[{"$match": {"status": {{json .status}}}}]`

	pipeline, err := BuildPipeline(tmpl, map[string]interface{}{"status": "paid"})

	require.NoError(t, err)
	require.Len(t, pipeline, 1)
	stage := pipeline[0].(bson.M)
	match := stage["$match"].(bson.M)
	assert.Equal(t, "paid", match["status"])
}

func TestBuildPipeline_SubstitutesDateParamAsExtendedJSONDate(t *testing.T) {
	tmpl := `[{"$match": {"createdAt": {"$gte": {"$date": {{json .from}}}}}}]`

	pipeline, err := BuildPipeline(tmpl, map[string]interface{}{"from": "2026-09-01T00:00:00Z"})

	require.NoError(t, err)
	stage := pipeline[0].(bson.M)
	match := stage["$match"].(bson.M)
	createdAt := match["createdAt"].(bson.M)
	_, isDateTime := createdAt["$gte"].(primitive.DateTime)
	assert.True(t, isDateTime, "expected $gte to decode as a BSON DateTime, got %T", createdAt["$gte"])
}

func TestBuildPipeline_FailsOnMalformedTemplateSyntax(t *testing.T) {
	_, err := BuildPipeline(`[{"$match": {{.unterminated}`, map[string]interface{}{})

	require.Error(t, err)
}

func TestBuildPipeline_FailsOnInvalidJSONAfterSubstitution(t *testing.T) {
	_, err := BuildPipeline(`[{"$match": not valid json}]`, map[string]interface{}{})

	require.Error(t, err)
}

func TestBuildPipeline_StringParamCannotBreakOutOfItsJSONPosition(t *testing.T) {
	tmpl := `[{"$match": {"status": {{json .status}}}}]`

	pipeline, err := BuildPipeline(tmpl, map[string]interface{}{"status": `", "$where": "malicious"`})

	require.NoError(t, err)
	stage := pipeline[0].(bson.M)
	match := stage["$match"].(bson.M)
	assert.Equal(t, `", "$where": "malicious"`, match["status"], "the injected value must stay a literal string value, not become new pipeline syntax")
	_, hasWhere := match["$where"]
	assert.False(t, hasWhere)
}
