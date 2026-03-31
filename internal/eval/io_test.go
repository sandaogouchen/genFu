package eval

import (
	"path/filepath"
	"testing"
)

func TestLoadBenchmarkInputs(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "eval")

	scenarios, predictions, err := LoadBenchmarkInputs(
		filepath.Join(root, "scenarios.json"),
		filepath.Join(root, "predictions.json"),
	)
	if err != nil {
		t.Fatalf("LoadBenchmarkInputs returned error: %v", err)
	}
	if len(scenarios) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(scenarios))
	}
	if len(predictions) != 4 {
		t.Fatalf("expected 4 predictions, got %d", len(predictions))
	}
	if predictions[0].System == "" {
		t.Fatalf("expected prediction system to be populated")
	}
}
