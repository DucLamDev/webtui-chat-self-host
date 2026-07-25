package application

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	automationSchemaResource = "https://vpsttt.invalid/automation-config-schema.json"
	maxAutomationSchemaBytes = 256 * 1024
	maxAutomationConfigBytes = 1024 * 1024
)

// ValidateAutomationConfig compiles the template schema locally and validates
// the merged installation config. External references are rejected so a
// database-controlled schema cannot turn validation into an outbound request.
func ValidateAutomationConfig(schema map[string]any, config map[string]any) error {
	if len(schema) == 0 {
		return nil
	}
	if containsExternalSchemaReference(schema) {
		return errors.New("external JSON Schema references are not supported")
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil || len(schemaJSON) > maxAutomationSchemaBytes {
		return errors.New("automation JSON Schema is invalid or too large")
	}
	configJSON, err := json.Marshal(config)
	if err != nil || len(configJSON) > maxAutomationConfigBytes {
		return errors.New("automation config is invalid or too large")
	}

	schemaDocument, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		return errors.New("automation JSON Schema cannot be decoded")
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	compiler.AssertFormat()
	if err := compiler.AddResource(automationSchemaResource, schemaDocument); err != nil {
		return errors.New("automation JSON Schema cannot be loaded")
	}
	compiled, err := compiler.Compile(automationSchemaResource)
	if err != nil {
		return errors.New("automation JSON Schema cannot be compiled")
	}

	var normalizedConfig any
	if err := json.Unmarshal(configJSON, &normalizedConfig); err != nil {
		return errors.New("automation config cannot be decoded")
	}
	if err := compiled.Validate(normalizedConfig); err != nil {
		return errors.New("automation config does not match the template schema")
	}
	return nil
}

func containsExternalSchemaReference(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if key == "$ref" {
				reference, ok := item.(string)
				if !ok || !strings.HasPrefix(strings.TrimSpace(reference), "#") {
					return true
				}
			}
			if containsExternalSchemaReference(item) {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			if containsExternalSchemaReference(item) {
				return true
			}
		}
	}
	return false
}
