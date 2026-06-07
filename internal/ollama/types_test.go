package ollama

import (
	"encoding/json"
	"testing"
	"time"
)

func TestListResponseUnmarshal(t *testing.T) {
	const fixture = `{
		"models": [
			{
				"name": "qwen2.5-coder:latest",
				"model": "qwen2.5-coder:latest",
				"modified_at": "2024-11-11T09:14:17.071291239-08:00",
				"size": 4683087332,
				"details": {
					"parameter_size": "7.6B",
					"quantization_level": "Q4_K_M"
				}
			},
			{
				"name": "llama3.2:latest",
				"model": "llama3.2:latest",
				"modified_at": "2024-10-22T13:39:22.713784865-07:00",
				"size": 2019393189,
				"details": {
					"parameter_size": "3.2B",
					"quantization_level": "Q4_K_M"
				}
			}
		]
	}`

	var resp ListResponse
	if err := json.Unmarshal([]byte(fixture), &resp); err != nil {
		t.Fatalf("unmarshal ListResponse: %v", err)
	}

	if len(resp.Models) != 2 {
		t.Fatalf("models len = %d, want 2", len(resp.Models))
	}

	wantModified, _ := time.Parse(time.RFC3339Nano, "2024-11-11T09:14:17.071291239-08:00")
	m0 := resp.Models[0]
	if m0.Name != "qwen2.5-coder:latest" {
		t.Errorf("name = %q, want qwen2.5-coder:latest", m0.Name)
	}
	if m0.Size != 4683087332 {
		t.Errorf("size = %d, want 4683087332", m0.Size)
	}
	if !m0.ModifiedAt.Equal(wantModified) {
		t.Errorf("modified_at = %v, want %v", m0.ModifiedAt, wantModified)
	}
	if m0.Details.ParameterSize != "7.6B" {
		t.Errorf("parameter_size = %q, want 7.6B", m0.Details.ParameterSize)
	}
	if m0.Details.QuantizationLevel != "Q4_K_M" {
		t.Errorf("quantization_level = %q, want Q4_K_M", m0.Details.QuantizationLevel)
	}
}

func TestProcessResponseUnmarshal(t *testing.T) {
	const fixture = `{
		"models": [
			{
				"name": "qwen2.5-coder:latest",
				"model": "qwen2.5-coder:latest",
				"size": 4683087332,
				"expires_at": "2024-11-11T10:14:17.071291239-08:00",
				"size_vram": 5368709120,
				"context_length": 32768,
				"details": {
					"parameter_size": "7.6B",
					"quantization_level": "Q4_K_M"
				}
			}
		]
	}`

	var resp ProcessResponse
	if err := json.Unmarshal([]byte(fixture), &resp); err != nil {
		t.Fatalf("unmarshal ProcessResponse: %v", err)
	}

	if len(resp.Models) != 1 {
		t.Fatalf("models len = %d, want 1", len(resp.Models))
	}

	wantExpires, _ := time.Parse(time.RFC3339Nano, "2024-11-11T10:14:17.071291239-08:00")
	m := resp.Models[0]
	if m.Name != "qwen2.5-coder:latest" {
		t.Errorf("name = %q, want qwen2.5-coder:latest", m.Name)
	}
	if m.SizeVRAM != 5368709120 {
		t.Errorf("size_vram = %d, want 5368709120", m.SizeVRAM)
	}
	if m.ContextLength != 32768 {
		t.Errorf("context_length = %d, want 32768", m.ContextLength)
	}
	if !m.ExpiresAt.Equal(wantExpires) {
		t.Errorf("expires_at = %v, want %v", m.ExpiresAt, wantExpires)
	}
}
