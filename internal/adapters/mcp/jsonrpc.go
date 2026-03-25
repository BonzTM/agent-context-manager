package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
)

const jsonRPCVersion = "2.0"

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`

	hasID bool
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

const (
	JSONRPCParseError     = -32700
	JSONRPCInvalidRequest = -32600
	JSONRPCMethodNotFound = -32601
	JSONRPCInvalidParams  = -32602
	JSONRPCInternalError  = -32603
)

func (r *JSONRPCRequest) UnmarshalJSON(data []byte) error {
	type requestAlias JSONRPCRequest
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var alias requestAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*r = JSONRPCRequest(alias)
	_, r.hasID = raw["id"]
	return nil
}

func (r JSONRPCRequest) IsNotification() bool {
	return !r.hasID || r.ID == nil
}

func (r JSONRPCRequest) ResponseID() any {
	if !r.hasID {
		return nil
	}
	return r.ID
}

func ParseJSONRPCRequest(data []byte) (JSONRPCRequest, *JSONRPCError) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return JSONRPCRequest{}, &JSONRPCError{Code: JSONRPCInvalidRequest, Message: "request must not be empty"}
	}

	var raw any
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return JSONRPCRequest{}, &JSONRPCError{Code: JSONRPCParseError, Message: "parse error", Data: err.Error()}
	}
	if _, ok := raw.([]any); ok {
		return JSONRPCRequest{}, &JSONRPCError{Code: JSONRPCInvalidRequest, Message: "batch requests are not supported"}
	}
	if _, ok := raw.(map[string]any); !ok {
		return JSONRPCRequest{}, &JSONRPCError{Code: JSONRPCInvalidRequest, Message: "request must be a JSON object"}
	}

	var req JSONRPCRequest
	if err := json.Unmarshal(trimmed, &req); err != nil {
		return JSONRPCRequest{}, &JSONRPCError{Code: JSONRPCInvalidRequest, Message: "invalid request", Data: err.Error()}
	}
	if rpcErr := validateJSONRPCRequest(req); rpcErr != nil {
		return req, rpcErr
	}
	return req, nil
}

func SerializeJSONRPCResponse(resp JSONRPCResponse) ([]byte, error) {
	return json.Marshal(resp)
}

func NewJSONRPCResultResponse(id any, result any) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: jsonRPCVersion, ID: id, Result: result}
}

func NewJSONRPCErrorResponse(id any, code int, message string, data any) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: strings.TrimSpace(message),
			Data:    data,
		},
	}
}

func validateJSONRPCRequest(req JSONRPCRequest) *JSONRPCError {
	if req.JSONRPC != jsonRPCVersion {
		return &JSONRPCError{Code: JSONRPCInvalidRequest, Message: `jsonrpc must be "2.0"`}
	}
	if strings.TrimSpace(req.Method) == "" {
		return &JSONRPCError{Code: JSONRPCInvalidRequest, Message: "method is required"}
	}
	if !req.hasID {
		return nil
	}
	if req.ID == nil {
		return nil
	}
	switch req.ID.(type) {
	case string, float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, json.Number:
		return nil
	default:
		return &JSONRPCError{Code: JSONRPCInvalidRequest, Message: "id must be a string, number, or null"}
	}
}
