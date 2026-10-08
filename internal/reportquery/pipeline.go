package reportquery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"text/template"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

// pipelineRegistry decodes BSON embedded documents into bson.M (rather than
// the driver's default bson.D) so pipeline stages can be asserted/indexed as
// maps. This only changes the Go type used for untyped embedded documents;
// explicitly-typed Extended JSON values (e.g. {"$date": ...}) still decode
// via their own dedicated codec (e.g. into primitive.DateTime).
var pipelineRegistry = newPipelineRegistry()

func newPipelineRegistry() *bsoncodec.Registry {
	reg := bson.NewRegistry()
	reg.RegisterTypeMapEntry(bsontype.EmbeddedDocument, reflect.TypeOf(bson.M{}))
	return reg
}

// BuildPipeline executes pipelineTemplate (a MongoDB Extended JSON string
// containing {{json .paramName}} placeholders) against params, then parses
// the result into a bson.A aggregation pipeline. Every param is JSON-encoded
// before substitution via the "json" template func, so a param value can
// never break out of its JSON string/number/bool position into surrounding
// pipeline syntax.
func BuildPipeline(pipelineTemplate string, params map[string]interface{}) (bson.A, error) {
	tmpl, err := template.New("pipeline").Funcs(template.FuncMap{
		"json": func(v interface{}) (string, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
	}).Parse(pipelineTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse pipeline template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return nil, fmt.Errorf("execute pipeline template: %w", err)
	}

	var pipeline bson.A
	if err := bson.UnmarshalExtJSONWithRegistry(pipelineRegistry, buf.Bytes(), false, &pipeline); err != nil {
		return nil, fmt.Errorf("parse pipeline json: %w", err)
	}
	return pipeline, nil
}
