package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccProviderHeaders(t *testing.T) {
	var (
		mu   sync.Mutex
		seen []http.Header
	)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/accounts", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Clone())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 101, "name": "Acme", "access_levels": []int{1000}},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	config := heredoc.Docf(`
		provider "mailtrap" {
		  api_token = "test-token"
		  base_url  = %q
		  headers = {
		    "X-Gateway-Id"     = "gateway-id"
		    "x-gateway-secret" = "gateway-secret"
		  }
		}

		data "mailtrap_account" "test" {
		}
	`, srv.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.mailtrap_account.test", "id", "101"),
					func(*terraform.State) error {
						mu.Lock()
						defer mu.Unlock()
						if len(seen) == 0 {
							t.Fatal("mock server received no requests")
						}
						for _, h := range seen {
							if got := h.Get("X-Gateway-Id"); got != "gateway-id" {
								t.Errorf("X-Gateway-Id = %q, want %q", got, "gateway-id")
							}
							if got := h.Get("X-Gateway-Secret"); got != "gateway-secret" {
								t.Errorf("X-Gateway-Secret = %q, want %q", got, "gateway-secret")
							}
							if got := h.Get("Authorization"); got != "Bearer test-token" {
								t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
							}
						}
						return nil
					},
				),
			},
		},
	})
}

func TestAccProviderHeaders_reserved(t *testing.T) {
	for _, name := range []string{"Authorization", "user-agent", "CONTENT-TYPE"} {
		t.Run(name, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: heredoc.Docf(`
							provider "mailtrap" {
							  api_token = "test-token"
							  base_url  = "http://127.0.0.1:1"
							  headers   = { %q = "x" }
							}

							data "mailtrap_account" "test" {
							}
						`, name),
						ExpectError: regexp.MustCompile(`Reserved header`),
					},
				},
			})
		})
	}
}

func TestHeadersTransport_doesNotMutateRequest(t *testing.T) {
	var seen *http.Request
	tr := &headersTransport{
		headers: map[string]string{"X-Extra": "1"},
		base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			seen = r
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
		}),
	}

	req := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if got := seen.Header.Get("X-Extra"); got != "1" {
		t.Errorf("X-Extra = %q, want %q", got, "1")
	}
	if got := req.Header.Get("X-Extra"); got != "" {
		t.Errorf("original request mutated: X-Extra = %q", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
