package tool

import "testing"

func TestParseKlineResponse(t *testing.T) {
	body := []byte(`{"data":{"klines":["2024-02-01,10,10.2,10.3,9.9,1000,10000,1.5","2024-02-02,10.2,10.1,10.4,10.0,2000,20000,2.0"]}}`)
	points, err := parseKlineResponse(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("unexpected length: %d", len(points))
	}
	if points[0].Open != 10 || points[0].Close != 10.2 {
		t.Fatalf("unexpected values")
	}
}

func TestParseIntradayResponse(t *testing.T) {
	body := []byte(`{"data":{"trends":["2024-02-01 09:30,10.1,100,10.05,1010","2024-02-01 09:31,10.2,200,10.08,2040"]}}`)
	points, err := parseIntradayResponse(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("unexpected length: %d", len(points))
	}
	if points[0].Price != 10.1 || points[0].Volume != 100 {
		t.Fatalf("unexpected values")
	}
}
