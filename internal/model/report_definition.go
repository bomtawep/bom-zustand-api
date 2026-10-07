package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ReportDefinition struct {
	ID               primitive.ObjectID `bson:"_id" json:"id"`
	Name             string             `bson:"name" json:"name"`
	TemplateID       primitive.ObjectID `bson:"template_id" json:"templateId"`
	Collection       string             `bson:"collection" json:"collection"`
	PipelineTemplate string             `bson:"pipeline_template" json:"pipelineTemplate"`
	ParamSchema      []ReportParam      `bson:"param_schema" json:"paramSchema"`
	CreatedAt        time.Time          `bson:"created_at" json:"createdAt"`
	UpdatedAt        time.Time          `bson:"updated_at" json:"updatedAt"`
}

type ReportParam struct {
	Name     string `bson:"name" json:"name"`
	Type     string `bson:"type" json:"type"`
	Required bool   `bson:"required" json:"required"`
	Label    string `bson:"label" json:"label"`
}
