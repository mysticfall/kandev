package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestImportOpenCodeCredentialsRequiresAcknowledgement(t *testing.T) {
	for _, body := range []string{`{"success":false}`, `synthetic-secret`, `{"success":true}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			err := newHTTPOnlyClient(server.URL).ImportOpenCodeCredentials(context.Background(), []byte(`[]`))
			if (err == nil) != (body == `{"success":true}`) {
				t.Fatalf("unexpected import result: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatal("peer output leaked into diagnostics")
			}
		})
	}
}
