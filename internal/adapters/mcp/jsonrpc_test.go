package mcp

import (
	"encoding/json"
	"testing"
)

func TestParseJSONRPCRequest_ValidJSON(t *testing.T) {
	req, rpcErr := ParseJSONRPCRequest([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	if rpcErr != nil {
		t.Fatalf("unexpected parse error: %+v", rpcErr)
	}
	if req.JSONRPC != jsonRPCVersion {
		t.Fatalf("unexpected jsonrpc version: %q", req.JSONRPC)
	}
	if req.Method != "tools/list" {
		t.Fatalf("unexpected method: %q", req.Method)
	}
	if req.IsNotification() {
		t.Fatal("expected request with id, got notification")
	}
}

func TestParseJSONRPCRequest_InvalidJSON(t *testing.T) {
	_, rpcErr := ParseJSONRPCRequest([]byte(`{`))
	if rpcErr == nil {
		t.Fatal("expected parse error")
	}
	if rpcErr.Code != JSONRPCParseError {
		t.Fatalf("unexpected error code: got %d want %d", rpcErr.Code, JSONRPCParseError)
	}
}

func TestParseJSONRPCRequest_MissingJSONRPCField(t *testing.T) {
	_, rpcErr := ParseJSONRPCRequest([]byte(`{"id":"req-1","method":"tools/list"}`))
	if rpcErr == nil {
		t.Fatal("expected invalid request error")
	}
	if rpcErr.Code != JSONRPCInvalidRequest {
		t.Fatalf("unexpected error code: got %d want %d", rpcErr.Code, JSONRPCInvalidRequest)
	}
}

func TestSerializeJSONRPCResponse_Success(t *testing.T) {
	encoded, err := SerializeJSONRPCResponse(JSONRPCResponse{
		JSONRPC: jsonRPCVersion,
		ID:      "req-1",
		Result: struct {
			OK bool `json:"ok"`
		}{OK: true},
	})
	if err != nil {
		t.Fatalf("serialize success response: %v", err)
	}
	if string(encoded) != `{"jsonrpc":"2.0","id":"req-1","result":{"ok":true}}` {
		t.Fatalf("unexpected serialized response: %s", string(encoded))
	}
	var roundTrip JSONRPCResponse
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatalf("round trip unmarshal: %v", err)
	}
}

func TestSerializeJSONRPCResponse_Error(t *testing.T) {
	encoded, err := SerializeJSONRPCResponse(NewJSONRPCErrorResponse("req-1", JSONRPCMethodNotFound, "method not found", nil))
	if err != nil {
		t.Fatalf("serialize error response: %v", err)
	}
	if string(encoded) != `{"jsonrpc":"2.0","id":"req-1","error":{"code":-32601,"message":"method not found"}}` {
		t.Fatalf("unexpected serialized response: %s", string(encoded))
	}
}

func TestJSONRPCErrorCodeConstants(t *testing.T) {
	if JSONRPCParseError != -32700 {
		t.Fatalf("unexpected parse error code: %d", JSONRPCParseError)
	}
	if JSONRPCInvalidRequest != -32600 {
		t.Fatalf("unexpected invalid request code: %d", JSONRPCInvalidRequest)
	}
	if JSONRPCMethodNotFound != -32601 {
		t.Fatalf("unexpected method not found code: %d", JSONRPCMethodNotFound)
	}
	if JSONRPCInvalidParams != -32602 {
		t.Fatalf("unexpected invalid params code: %d", JSONRPCInvalidParams)
	}
	if JSONRPCInternalError != -32603 {
		t.Fatalf("unexpected internal error code: %d", JSONRPCInternalError)
	}
}
