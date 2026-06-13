// Command team-sample-app is the Phase-1 golden-path sample workload (D-004).
//
// It is deliberately minimal — standard library only, scratch/distroless image —
// so it cold-starts instantly and is trivial to build and test. Its one job
// beyond serving health probes is to PROVE the secret-consumption path end to
// end (D-006): it reads APP_SECRET (sourced from a Sealed Secret) and echoes a
// proof-of-read on "/" without ever leaking the secret value.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
)

const (
	defaultPort  = "8080"
	secretEnvVar = "APP_SECRET"
	portEnvVar   = "PORT"
)

func main() {
	secret := os.Getenv(secretEnvVar)
	port := resolvePort(os.Getenv(portEnvVar))

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: newMux(secret),
	}

	log.Printf("team-sample-app listening on :%s — %s", port, secretProof(secret))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

// newMux wires the two routes. The secret is captured once at startup so the
// hot path performs no env lookups.
func newMux(secret string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/", rootHandler(secret))
	return mux
}

// healthzHandler is the liveness/readiness probe. It must succeed independently
// of secret state so the platform can tell "process up" from "secret missing".
func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

// rootHandler returns the "/" handler closed over the secret read at startup.
// 404s any path other than exactly "/" (ServeMux routes "/" as a catch-all).
func rootHandler(secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "team-sample-app")
		fmt.Fprintln(w, secretProof(secret))
	}
}

// secretProof builds a human-readable proof that the secret was read, without
// disclosing it: a loaded flag, the byte length, and the first 8 hex chars of
// its SHA-256 digest. An unset secret reports loaded:false / length=0 so the
// failure is observable rather than crashing the app.
func secretProof(secret string) string {
	if secret == "" {
		return "secret loaded: false, length=0, sha256=none"
	}
	sum := sha256.Sum256([]byte(secret))
	prefix := hex.EncodeToString(sum[:])[:8]
	return fmt.Sprintf("secret loaded: true, length=%d, sha256=%s", len(secret), prefix)
}

// resolvePort returns the validated listen port. The container port is declared
// in app-metadata.yaml and injected via PORT; anything missing, non-numeric, or
// out of range falls back to the default so the app always binds somewhere sane.
func resolvePort(env string) string {
	if env == "" {
		return defaultPort
	}
	n, err := strconv.Atoi(env)
	if err != nil || n < 1 || n > 65535 {
		return defaultPort
	}
	return env
}
