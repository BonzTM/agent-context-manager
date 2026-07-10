package llmmap

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const maxSchemaBytes = 1 << 20

// LoadJSONSchema compiles a bounded local JSON Schema and returns its content
// hash for inclusion in a resumable run's processing contract.
func LoadJSONSchema(path string) (Validator, string, error) {
	if path == "" {
		return nil, "", nil
	}
	content, err := readFileBounded(path, maxSchemaBytes)
	if err != nil {
		return nil, "", fmt.Errorf("llmmap: read output schema: %w", err)
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(content))
	if err != nil {
		return nil, "", fmt.Errorf("llmmap: decode output schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(deniedSchemaLoader{})
	const location = "urn:acm:map-output-schema"
	if resourceErr := compiler.AddResource(location, document); resourceErr != nil {
		return nil, "", fmt.Errorf("llmmap: load output schema: %w", resourceErr)
	}
	schema, err := compiler.Compile(location)
	if err != nil {
		return nil, "", fmt.Errorf("llmmap: compile output schema: %w", err)
	}
	hash := sha256.Sum256(content)
	return schemaValidator(schema), hex.EncodeToString(hash[:]), nil
}

func schemaValidator(schema *jsonschema.Schema) Validator {
	return func(output json.RawMessage) error {
		value, err := jsonschema.UnmarshalJSON(bytes.NewReader(output))
		if err != nil {
			return fmt.Errorf("schema output decode: %w", err)
		}
		if err := schema.Validate(value); err != nil {
			return fmt.Errorf("schema validation: %w", err)
		}
		return nil
	}
}

type deniedSchemaLoader struct{}

func (deniedSchemaLoader) Load(location string) (any, error) {
	return nil, fmt.Errorf("llmmap: external schema reference %q is disabled", location)
}

// CombineValidators runs each non-nil validator in declaration order.
func CombineValidators(validators ...Validator) Validator {
	return func(output json.RawMessage) error {
		for _, validate := range validators {
			if validate == nil {
				continue
			}
			if err := validate(output); err != nil {
				return err
			}
		}
		return nil
	}
}
