package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const tagsFixture = `{
	"models": [
		{
			"name": "qwen2.5-coder:3b",
			"model": "qwen2.5-coder:3b",
			"modified_at": "2024-11-11T09:14:17.071291239-08:00",
			"size": 1929912432,
			"details": {
				"parameter_size": "3B",
				"quantization_level": "Q4_K_M"
			}
		}
	]
}`

const psFixture = `{
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

func newTestServer(t *testing.T, tagsBody, psBody string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(tagsBody))
		case "/api/ps":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(psBody))
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestTags(t *testing.T) {
	srv := newTestServer(t, tagsFixture, psFixture)
	defer srv.Close()

	client := NewClient(srv.URL)
	tags, err := client.Tags(context.Background())
	if err != nil {
		t.Fatalf("Tags() error = %v", err)
	}

	if len(tags) != 1 {
		t.Fatalf("len(tags) = %d, want 1", len(tags))
	}
	if tags[0].Name != "qwen2.5-coder:3b" {
		t.Errorf("name = %q, want qwen2.5-coder:3b", tags[0].Name)
	}
}

func TestPs(t *testing.T) {
	srv := newTestServer(t, tagsFixture, psFixture)
	defer srv.Close()

	client := NewClient(srv.URL)
	running, err := client.Ps(context.Background())
	if err != nil {
		t.Fatalf("Ps() error = %v", err)
	}

	if len(running) != 1 {
		t.Fatalf("len(running) = %d, want 1", len(running))
	}
	if running[0].ContextLength != 32768 {
		t.Errorf("context_length = %d, want 32768", running[0].ContextLength)
	}
}

func TestReachableConnectionRefused(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")

	if client.Reachable(context.Background()) {
		t.Fatal("Reachable() = true, want false for refused connection")
	}

	_, err := client.Tags(context.Background())
	if err == nil {
		t.Fatal("Tags() error = nil, want connection error")
	}
}

func TestFetchSnapshotSuccess(t *testing.T) {
	srv := newTestServer(t, tagsFixture, psFixture)
	defer srv.Close()

	before := time.Now()
	snap := FetchSnapshot(context.Background(), NewClient(srv.URL), DefaultModel)

	if !snap.Reachable {
		t.Error("Reachable = false, want true")
	}
	if snap.Error != "" {
		t.Errorf("Error = %q, want empty", snap.Error)
	}
	if len(snap.Tags) != 1 {
		t.Fatalf("len(Tags) = %d, want 1", len(snap.Tags))
	}
	if len(snap.Running) != 1 {
		t.Fatalf("len(Running) = %d, want 1", len(snap.Running))
	}
	if snap.ModelMissing {
		t.Error("ModelMissing = true, want false")
	}
	if snap.FetchedAt.Before(before) {
		t.Errorf("FetchedAt = %v, want >= %v", snap.FetchedAt, before)
	}
}

func TestFetchSnapshotUnreachable(t *testing.T) {
	snap := FetchSnapshot(context.Background(), NewClient("http://127.0.0.1:1"), DefaultModel)

	if snap.Reachable {
		t.Error("Reachable = true, want false")
	}
	if snap.Error == "" {
		t.Fatal("Error = empty, want connection error")
	}
	if len(snap.Tags) != 0 {
		t.Errorf("len(Tags) = %d, want 0", len(snap.Tags))
	}
	if len(snap.Running) != 0 {
		t.Errorf("len(Running) = %d, want 0", len(snap.Running))
	}
	if snap.FetchedAt.IsZero() {
		t.Error("FetchedAt is zero, want timestamp")
	}
}

func TestModelMissing(t *testing.T) {
	const tagsWithoutDefault = `{
		"models": [
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

	srv := newTestServer(t, tagsWithoutDefault, `{"models":[]}`)
	defer srv.Close()

	snap := FetchSnapshot(context.Background(), NewClient(srv.URL), DefaultModel)
	if !snap.ModelMissing {
		t.Error("ModelMissing = false, want true")
	}
}

func TestHasModel_ExactTag(t *testing.T) {
	tags := []TagModel{{Name: "qwen2.5-coder:3b"}}
	if !HasModel(tags, "qwen2.5-coder:3b") {
		t.Fatal("expected exact tag match")
	}
	if HasModel(tags, "qwen2.5-coder:latest") {
		t.Fatal("different tag should not match")
	}
}

func TestHasModel_BaseName(t *testing.T) {
	tags := []TagModel{{Name: "qwen2.5-coder:latest"}}
	if !HasModel(tags, "qwen2.5-coder") {
		t.Fatal("base name should match tagged variant")
	}
	if HasModel(tags, "qwen2.5-coder:3b") {
		t.Fatal("specific tag should not match different variant")
	}
}

func TestNewClientUsesOLLAMAHost(t *testing.T) {
	srv := newTestServer(t, tagsFixture, psFixture)
	defer srv.Close()

	t.Setenv("OLLAMA_HOST", srv.URL)

	client := NewClient("")
	tags, err := client.Tags(context.Background())
	if err != nil {
		t.Fatalf("Tags() error = %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("len(tags) = %d, want 1", len(tags))
	}
}
