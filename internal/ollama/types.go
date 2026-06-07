package ollama

import "time"

// ListResponse is the JSON body from GET /api/tags.
type ListResponse struct {
	Models []TagModel `json:"models"`
}

// TagModel describes an installed model from /api/tags.
type TagModel struct {
	Name       string       `json:"name"`
	Model      string       `json:"model"`
	ModifiedAt time.Time    `json:"modified_at"`
	Size       int64        `json:"size"`
	Details    ModelDetails `json:"details"`
}

// ProcessResponse is the JSON body from GET /api/ps.
type ProcessResponse struct {
	Models []RunningModel `json:"models"`
}

// RunningModel describes a loaded model from /api/ps.
type RunningModel struct {
	Name          string       `json:"name"`
	Model         string       `json:"model"`
	Size          int64        `json:"size"`
	ExpiresAt     time.Time    `json:"expires_at"`
	SizeVRAM      int64        `json:"size_vram"`
	ContextLength int          `json:"context_length"`
	Details       ModelDetails `json:"details"`
}

// ModelDetails holds parameter and quantization metadata shared by tag and running models.
type ModelDetails struct {
	ParameterSize     string `json:"parameter_size"`
	QuantizationLevel string `json:"quantization_level"`
}
