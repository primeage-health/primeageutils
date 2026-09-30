// Package httpserver is the REST plumbing the internal services share: the
// service-token gate, the JSON response writer and a server that drains on
// shutdown.
package httpserver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/primeage-health/primeageutils/errs"
)

// TokenHeader carries the shared secret sibling services present.
const TokenHeader = "X-Service-Token"

// TokenGate rejects any request whose TokenHeader is not token, except
// healthPath, which the orchestrator reaches with no token to present.
//
// The compare is constant-time: a byte-by-byte one leaks the token's prefix
// through response timing. An empty token panics, since it would admit a
// request carrying no header at all.
func TokenGate(token, healthPath string) func(http.Handler) http.Handler {
	if strings.TrimSpace(token) == "" {
		panic("httpserver: the service token is empty; refusing to serve an unauthenticated surface")
	}
	want := []byte(token)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == healthPath {
				next.ServeHTTP(w, r)
				return
			}

			presented := strings.TrimSpace(r.Header.Get(TokenHeader))
			if subtle.ConstantTimeCompare([]byte(presented), want) != 1 {
				WriteJSON(w, http.StatusUnauthorized, map[string]string{"message": "Not Authorized"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// WriteJSON writes data as the JSON response body with the given status.
//
// Service errors are written lowercase, Go style, but shown verbatim in a toast,
// so a *errs.RestError's message is capitalized here — the one point every one
// passes through — rather than at each call site.
func WriteJSON(w http.ResponseWriter, code int, data any) {
	if e, ok := data.(*errs.RestError); ok {
		e.Message = capitalizeFirst(e.Message)
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		panic(err)
	}
}

func capitalizeFirst(message string) string {
	r, size := utf8.DecodeRuneInString(message)
	if size == 0 || !unicode.IsLower(r) {
		return message
	}
	return string(unicode.ToUpper(r)) + message[size:]
}

// Start serves handler on port until ctx is done, then shuts down. It panics
// if the listener fails.
func Start(ctx context.Context, port string, handler http.Handler) {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: handler,
	}

	log.Println("starting rest server on port: " + port)
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Panic("rest server closed: " + err.Error())
		}
	}()

	<-ctx.Done()
	srv.Shutdown(context.Background())
}
