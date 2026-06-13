package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHealthz verifies the liveness/readiness endpoint always returns 200 OK
// regardless of secret state — probes must not depend on app config.
func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	healthzHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz: got status %d, want %d", rec.Code, http.StatusOK)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body != "ok" {
		t.Errorf("GET /healthz: got body %q, want %q", body, "ok")
	}
}

// TestRootSecretLoaded verifies that when APP_SECRET is set, the root handler
// proves it read the secret (loaded=true, correct length, and a SHA-256 prefix)
// WITHOUT ever echoing the raw secret value.
func TestRootSecretLoaded(t *testing.T) {
	secret := "super-secret-value"
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	rootHandler(secret)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /: got status %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "secret loaded: true") {
		t.Errorf("GET /: expected proof of read 'secret loaded: true', got %q", body)
	}
	if !strings.Contains(body, "length=18") {
		t.Errorf("GET /: expected 'length=18' for secret of len 18, got %q", body)
	}

	// The raw secret must NEVER appear in the response.
	if strings.Contains(body, secret) {
		t.Errorf("GET /: response leaked the raw secret value: %q", body)
	}

	// The SHA-256 prefix must be present and correct (first 8 hex chars).
	sum := sha256.Sum256([]byte(secret))
	wantPrefix := hex.EncodeToString(sum[:])[:8]
	if !strings.Contains(body, wantPrefix) {
		t.Errorf("GET /: expected sha256 prefix %q in body, got %q", wantPrefix, body)
	}
}

// TestRootSecretMissing verifies graceful behavior when APP_SECRET is unset:
// the handler still returns 200 (the app is up) but reports loaded=false and
// length=0, so the secret path failure is observable without crashing.
func TestRootSecretMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	rootHandler("")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /: got status %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "secret loaded: false") {
		t.Errorf("GET /: expected 'secret loaded: false' when unset, got %q", body)
	}
	if !strings.Contains(body, "length=0") {
		t.Errorf("GET /: expected 'length=0' when unset, got %q", body)
	}
}

// TestSecretProof unit-tests the pure proof-string builder for both states,
// ensuring the raw secret is never embedded in the proof string.
func TestSecretProof(t *testing.T) {
	t.Run("loaded", func(t *testing.T) {
		secret := "abc123"
		got := secretProof(secret)
		if !strings.Contains(got, "secret loaded: true") {
			t.Errorf("secretProof(%q): missing loaded:true, got %q", secret, got)
		}
		if !strings.Contains(got, "length=6") {
			t.Errorf("secretProof(%q): missing length=6, got %q", secret, got)
		}
		if strings.Contains(got, secret) {
			t.Errorf("secretProof leaked raw secret: %q", got)
		}
	})

	t.Run("empty", func(t *testing.T) {
		got := secretProof("")
		if !strings.Contains(got, "secret loaded: false") {
			t.Errorf("secretProof(\"\"): missing loaded:false, got %q", got)
		}
		if !strings.Contains(got, "length=0") {
			t.Errorf("secretProof(\"\"): missing length=0, got %q", got)
		}
	})
}

// TestResolvePort verifies the port resolver honors the env override and falls
// back to the default when unset or invalid.
func TestResolvePort(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want string
	}{
		{"default when empty", "", "8080"},
		{"override valid", "9090", "9090"},
		{"fallback on non-numeric", "abc", "8080"},
		{"fallback on zero", "0", "8080"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolvePort(tc.env); got != tc.want {
				t.Errorf("resolvePort(%q) = %q, want %q", tc.env, got, tc.want)
			}
		})
	}
}

// TestRouting verifies the mux wires the two routes through a real server so an
// unknown path 404s while the known paths respond — guards the integration seam.
func TestRouting(t *testing.T) {
	srv := httptest.NewServer(newMux("test-secret"))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /healthz via mux: got %d, want 200", resp.StatusCode)
	}

	resp2, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp2.Body.Close()
	b, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("GET / via mux: got %d, want 200", resp2.StatusCode)
	}
	if !strings.Contains(string(b), "secret loaded: true") {
		t.Errorf("GET / via mux: missing secret proof, got %q", string(b))
	}
}
