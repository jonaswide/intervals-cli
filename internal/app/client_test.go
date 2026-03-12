package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestBearerAuthTakesPrecedence(t *testing.T) {
	t.Setenv("INTERVALS_ACCESS_TOKEN", "token-123")
	t.Setenv("INTERVALS_API_KEY", "key-456")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
			t.Fatalf("unexpected auth header %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != "intervals/test" {
			t.Fatalf("unexpected user-agent %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"athlete": map[string]any{"id": "0"}})
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, Timeout: time.Second, UserAgent: "intervals/test", Stderr: io.Discard})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.WhoAmI(context.Background()); err != nil {
		t.Fatalf("WhoAmI: %v", err)
	}
}

func TestAPIKeyFallbackUsesBasicAuth(t *testing.T) {
	t.Setenv("INTERVALS_ACCESS_TOKEN", "")
	t.Setenv("INTERVALS_API_KEY", "key-456")
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("API_KEY:key-456"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != want {
			t.Fatalf("unexpected auth header %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"athlete": map[string]any{"id": "0"}})
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, Timeout: time.Second, UserAgent: "intervals/test", Stderr: io.Discard})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.WhoAmI(context.Background()); err != nil {
		t.Fatalf("WhoAmI: %v", err)
	}
}

func TestSafeReadRetries(t *testing.T) {
	t.Setenv("INTERVALS_ACCESS_TOKEN", "token-123")
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			http.Error(w, "try again", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"athlete": map[string]any{"id": "0"}})
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, Timeout: 3 * time.Second, UserAgent: "intervals/test", Stderr: io.Discard})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.WhoAmI(context.Background()); err != nil {
		t.Fatalf("WhoAmI: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 calls, got %d", got)
	}
}

func TestMutatingRequestIsNotRetried(t *testing.T) {
	t.Setenv("INTERVALS_ACCESS_TOKEN", "token-123")
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "try again", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, Timeout: time.Second, UserAgent: "intervals/test", Stderr: io.Discard})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = client.EventsCreate(context.Background(), []byte(`{"name":"x"}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 call, got %d", got)
	}
}

func TestBinaryDownload(t *testing.T) {
	t.Setenv("INTERVALS_ACCESS_TOKEN", "token-123")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fit-bytes"))
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, Timeout: time.Second, UserAgent: "intervals/test", Stderr: io.Discard})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	data, err := client.ActivityDownload(context.Background(), "abc", "original")
	if err != nil {
		t.Fatalf("ActivityDownload: %v", err)
	}
	if string(data) != "fit-bytes" {
		t.Fatalf("unexpected bytes %q", string(data))
	}
}

func TestFilterActivitySearchResultsByNameAndTag(t *testing.T) {
	activities := []any{
		map[string]any{
			"id":               "1",
			"name":             "Tempo Run",
			"start_date_local": "2026-03-02",
			"type":             "Run",
			"tags":             []any{"#tempo", "#block"},
			"description":      "steady",
		},
		map[string]any{
			"id":               "2",
			"name":             "Easy Ride",
			"start_date_local": "2026-03-03",
			"type":             "Ride",
			"tags":             []any{"#easy"},
		},
	}

	nameMatches := filterActivitySearchResults(activities, "tempo", nil)
	if len(nameMatches) != 1 {
		t.Fatalf("expected 1 name match, got %d", len(nameMatches))
	}
	row := nameMatches[0].(map[string]any)
	if _, ok := row["start_date_local"]; !ok {
		t.Fatalf("expected projected search result shape, got %#v", row)
	}
	if _, ok := row["icu_training_load"]; ok {
		t.Fatalf("expected projected result, got extra list fields %#v", row)
	}

	tagMatches := filterActivitySearchResults(activities, "#tempo", nil)
	if len(tagMatches) != 1 {
		t.Fatalf("expected 1 tag match, got %d", len(tagMatches))
	}

	noMatches := filterActivitySearchResults(activities, "#threshold", nil)
	if len(noMatches) != 0 {
		t.Fatalf("expected no matches, got %d", len(noMatches))
	}
}
