package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAPITokenResource(t *testing.T) {
	srv := newAPITokensMockServer()
	t.Cleanup(srv.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(srv.URL) + `
resource "mailtrap_api_token" "test" {
  name = "ci-token"
  resources = [{
    resource_type = "account"
    resource_id   = 1
    access_level  = 100
  }]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mailtrap_api_token.test", "name", "ci-token"),
					resource.TestCheckResourceAttr("mailtrap_api_token.test", "resources.#", "1"),
					resource.TestCheckResourceAttr("mailtrap_api_token.test", "resources.0.resource_type", "account"),
					resource.TestCheckResourceAttr("mailtrap_api_token.test", "token", "full-token-value-1234"),
					resource.TestCheckResourceAttr("mailtrap_api_token.test", "last_4_digits", "1234"),
					resource.TestCheckResourceAttrSet("mailtrap_api_token.test", "id"),
				),
			},
			{
				ResourceName:      "mailtrap_api_token.test",
				ImportState:       true,
				ImportStateVerify: true,
				// The full token value is only returned on creation, so an
				// imported token can never recover it.
				ImportStateVerifyIgnore: []string{"token"},
			},
			{ // renaming forces replacement (no update endpoint); a new token is issued
				Config: testProviderConfig(srv.URL) + `
resource "mailtrap_api_token" "test" {
  name = "ci-token-v2"
  resources = [{
    resource_type = "account"
    resource_id   = 1
    access_level  = 100
  }]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mailtrap_api_token.test", "name", "ci-token-v2"),
					resource.TestCheckResourceAttrSet("mailtrap_api_token.test", "token"),
				),
			},
		},
	})
}

// newAPITokensMockServer is a minimal in-memory stand-in for the Mailtrap API
// tokens API. The full token value is included only in the create response.
func newAPITokensMockServer() *httptest.Server {
	var (
		mu     sync.Mutex
		tokens = map[int64]map[string]any{}
		lastID int64
	)

	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	// withoutToken copies a token, dropping the create-only full value.
	withoutToken := func(t map[string]any) map[string]any {
		out := make(map[string]any, len(t))
		for k, v := range t {
			if k != "token" {
				out[k] = v
			}
		}
		return out
	}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/api_tokens", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		defer mu.Unlock()
		lastID++
		t := map[string]any{
			"id":            lastID,
			"token":         "full-token-value-1234",
			"last_4_digits": "1234",
			"created_by":    "terraform",
		}
		for k, v := range body {
			t[k] = v
		}
		tokens[lastID] = t
		writeJSON(w, t)
	})

	mux.HandleFunc("GET /api/api_tokens/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		t, ok := tokens[pathID(r)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, withoutToken(t))
	})

	mux.HandleFunc("DELETE /api/api_tokens/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		delete(tokens, pathID(r))
		w.WriteHeader(http.StatusNoContent)
	})

	return httptest.NewServer(mux)
}
