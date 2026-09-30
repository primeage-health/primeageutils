package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/primeage-health/primeageutils/errs"
)

const (
	testToken  = "service-token"
	testHealth = "/svc/health"
)

func TestTokenGate(t *testing.T) {
	gate := TokenGate(testToken, testHealth)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for name, tc := range map[string]struct {
		path, token string
		want        int
	}{
		"missing token":            {"/svc/send", "", http.StatusUnauthorized},
		"wrong token":              {"/svc/send", "wrong-token", http.StatusUnauthorized},
		"right token":              {"/svc/send", " " + testToken + " ", http.StatusNoContent},
		"health without a token":   {testHealth, "", http.StatusNoContent},
		"health prefix is not one": {testHealth + "/x", "", http.StatusUnauthorized},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		if tc.token != "" {
			req.Header.Set(TokenHeader, tc.token)
		}
		rec := httptest.NewRecorder()
		gate.ServeHTTP(rec, req)

		if rec.Code != tc.want {
			t.Errorf("%s: status = %d; want %d", name, rec.Code, tc.want)
		}
		if tc.want == http.StatusUnauthorized && strings.TrimSpace(rec.Body.String()) != `{"message":"Not Authorized"}` {
			t.Errorf("%s: body = %s", name, rec.Body.String())
		}
	}
}

func TestTokenGateRefusesAnEmptyToken(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("an empty token did not panic")
		}
	}()
	TokenGate(" ", testHealth)
}

func TestWriteJSONCapitalizesARestError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusBadRequest, errs.NewValidationError("recipient is required"))

	if rec.Code != http.StatusBadRequest || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status = %d, content type = %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), `"Recipient is required"`) {
		t.Errorf("body = %s; want the message capitalized", rec.Body.String())
	}
}
