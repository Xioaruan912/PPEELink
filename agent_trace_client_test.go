package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentTracePreparationProtocol(t *testing.T) {
	nonce := "0123456789abcdef0123456789abcdef"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Agent token" {
			t.Error("missing token")
		}
		if r.Method == "GET" {
			if r.URL.Query().Get("version") != "2" {
				t.Error("missing modern capability")
			}
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"action": "prepare", "prepareId": nonce}}})
			return
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["action"] != "prepare" || body["prepareId"] != nonce || body["error"] != "checksum failed" {
			t.Errorf("report=%+v", body)
		}
	}))
	defer server.Close()
	jobs, err := fetchAgentTraceJobs(server.Client(), server.URL, "token")
	if err != nil || len(jobs) != 1 || jobs[0].Action != "prepare" || jobs[0].PrepareID != nonce {
		t.Fatalf("jobs=%+v err=%v", jobs, err)
	}
	if err := reportAgentTracePreparation(server.Client(), server.URL, "token", nonce, errors.New("checksum failed")); err != nil {
		t.Fatal(err)
	}
}
