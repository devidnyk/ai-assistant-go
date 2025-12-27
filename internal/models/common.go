package models

import (
	"ai-assistant/configs"

	"github.com/qdrant/go-client/qdrant"
)

type Payload struct {
	Context    string
	DataSource string
	SourceType configs.SourceDataType
	Command    string
	Metadata   map[string]any
}

func CreatePayload(context string, dataSource string, sourceType configs.SourceDataType, cmd string, metadata map[string]any) Payload {
	return Payload{
		Context:    context,
		DataSource: dataSource,
		SourceType: sourceType,
		Command:    cmd,
		Metadata:   metadata,
	}
}

func (p *Payload) ToMap() map[string]*qdrant.Value {
	mp := map[string]any{
		"context":    p.Context,
		"dataSource": p.DataSource,
		"sourceType": p.SourceType.String(),
		"command":    p.Command,
		"metadata":   p.Metadata,
	}

	return qdrant.NewValueMap(mp)
}

// FromMap converts a Qdrant value map back to a Payload struct
func FromMap(m map[string]*qdrant.Value) Payload {
	payload := Payload{
		Metadata: make(map[string]any),
	}

	if val, ok := m["context"]; ok && val.GetStringValue() != "" {
		payload.Context = val.GetStringValue()
	}

	if val, ok := m["dataSource"]; ok && val.GetStringValue() != "" {
		payload.DataSource = val.GetStringValue()
	}

	if val, ok := m["sourceType"]; ok && val.GetStringValue() != "" {
		sourceTypeStr := val.GetStringValue()
		payload.SourceType = configs.ParseSourceDataType(sourceTypeStr)
	}

	if val, ok := m["command"]; ok && val.GetStringValue() != "" {
		payload.Command = val.GetStringValue()
	}

	if val, ok := m["metadata"]; ok {
		if structVal := val.GetStructValue(); structVal != nil {
			for key, value := range structVal.GetFields() {
				if value.GetStringValue() != "" {
					payload.Metadata[key] = value.GetStringValue()
				} else if value.GetIntegerValue() != 0 {
					payload.Metadata[key] = value.GetIntegerValue()
				} else if value.GetBoolValue() {
					payload.Metadata[key] = value.GetBoolValue()
				}
			}
		}
	}

	return payload
}
