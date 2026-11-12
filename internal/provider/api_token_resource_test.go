package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// mockDefaultTokenExpiry is what the mock API sets when a create request
// carries no expires_at, standing in for the server's default lifetime.
const mockDefaultTokenExpiry = "2027-11-11T00:00:00.000Z"

func TestAccAPITokenResource(t *testing.T) {
	srv := newAPITokensMockServer()
	t.Cleanup(srv.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(srv.URL) + heredoc.Doc(`
					resource "mailtrap_api_token" "test" {
					  name = "ci-token"
					  resources = [{
					    resource_type = "account"
					    resource_id   = 1
					    access_level  = 100
					  }]
					}
				`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mailtrap_api_token.test", "name", "ci-token"),
					resource.TestCheckResourceAttr("mailtrap_api_token.test", "resources.#", "1"),
					resource.TestCheckResourceAttr("mailtrap_api_token.test", "resources.0.resource_type", "account"),
					resource.TestCheckResourceAttr("mailtrap_api_token.test", "token", "full-token-value-1234"),
					resource.TestCheckResourceAttr("mailtrap_api_token.test", "last_4_digits", "1234"),
					// the API's default, stored in canonical UTC form
					resource.TestCheckResourceAttr("mailtrap_api_token.test", "expires_at", "2027-11-11T00:00:00Z"),
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
				Config: testProviderConfig(srv.URL) + heredoc.Doc(`
					resource "mailtrap_api_token" "test" {
					  name = "ci-token-v2"
					  resources = [{
					    resource_type = "account"
					    resource_id   = 1
					    access_level  = 100
					  }]
					}
				`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mailtrap_api_token.test", "name", "ci-token-v2"),
					resource.TestCheckResourceAttrSet("mailtrap_api_token.test", "token"),
				),
			},
		},
	})
}

func TestAccAPITokenResource_expiresAt(t *testing.T) {
	srv := newAPITokensMockServer()
	t.Cleanup(srv.Close)

	config := func(expiresAt string) string {
		return testProviderConfig(srv.URL) + heredoc.Docf(`
			resource "mailtrap_api_token" "test" {
			  name = "ci-token"
			  %s
			}
		`, expiresAt)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{ // first, so the post-test destroy runs with a valid config
				Config:      config(`expires_at = "next year"`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`RFC 3339`),
			},
			{
				Config: config(`expires_at = "never"`),
				Check:  resource.TestCheckResourceAttr("mailtrap_api_token.test", "expires_at", "never"),
			},
			{ // an imported token that never expires reads back as "never"
				ResourceName:            "mailtrap_api_token.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"token"},
			},
			{ // a timestamp replaces the token; the API's reformatted value is not a diff
				Config: config(`expires_at = "2027-06-01T00:00:00Z"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("mailtrap_api_token.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.TestCheckResourceAttr("mailtrap_api_token.test", "expires_at", "2027-06-01T00:00:00Z"),
			},
			{ // a value known only at apply may be any instant, so it replaces
				Config: config(`expires_at = terraform_data.expiry.output`) + heredoc.Doc(`
					resource "terraform_data" "expiry" {
					  input = "2028-01-01T00:00:00Z"
					}
				`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectUnknownValue("mailtrap_api_token.test", tfjsonpath.New("expires_at")),
						plancheck.ExpectResourceAction("mailtrap_api_token.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.TestCheckResourceAttr("mailtrap_api_token.test", "expires_at", "2028-01-01T00:00:00Z"),
			},
			{ // removing expires_at keeps the token
				Config: config(""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("mailtrap_api_token.test", plancheck.ResourceActionNoop),
					},
				},
				Check: resource.TestCheckResourceAttr("mailtrap_api_token.test", "expires_at", "2028-01-01T00:00:00Z"),
			},
		},
	})
}

// An imported token holds the API's spelling of its expiry. Config naming the
// same instant differently must not replace it; a different instant must.
func TestAccAPITokenResource_expiresAtImported(t *testing.T) {
	srv := newAPITokensMockServer()
	t.Cleanup(srv.Close)

	// seed creates the token; imported then adopts it through an import block,
	// so its state starts from the API's 2027-06-01T00:00:00.000Z, and seed is
	// forgotten without deleting the token.
	config := func(expiresAt string) string {
		return testProviderConfig(srv.URL) + heredoc.Docf(`
			removed {
			  from = mailtrap_api_token.seed
			  lifecycle {
			    destroy = false
			  }
			}

			import {
			  to = mailtrap_api_token.imported
			  id = "1" # seed's id in the mock
			}

			resource "mailtrap_api_token" "imported" {
			  name       = "ci-token"
			  expires_at = %q
			}
		`, expiresAt)
	}
	const imported = "mailtrap_api_token.imported"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(srv.URL) + heredoc.Doc(`
					resource "mailtrap_api_token" "seed" {
					  name       = "ci-token"
					  expires_at = "2027-06-01T00:00:00Z"
					}
				`),
			},
			{ // the same instant as the API's spelling: import only
				Config: config("2027-06-01T00:00:00Z"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(imported, plancheck.ResourceActionNoop),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(imported, "id", "1"),
					resource.TestCheckResourceAttr(imported, "expires_at", "2027-06-01T00:00:00Z"),
				),
			},
			{ // the same instant with an offset: state only, same token
				Config: config("2027-06-01T02:00:00+02:00"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(imported, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(imported, "id", "1"),
					resource.TestCheckResourceAttr(imported, "expires_at", "2027-06-01T02:00:00+02:00"),
				),
			},
			{ // a different instant replaces the token
				Config: config("2028-01-01T00:00:00Z"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(imported, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(imported, "id", "2"),
					resource.TestCheckResourceAttr(imported, "expires_at", "2028-01-01T00:00:00Z"),
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
		// Like the API: no expires_at means the default lifetime, null means
		// none, and timestamps come back with fractional seconds.
		switch v, ok := body["expires_at"]; {
		case !ok:
			t["expires_at"] = mockDefaultTokenExpiry
		case v != nil:
			if at, err := time.Parse(time.RFC3339, v.(string)); err == nil {
				t["expires_at"] = at.UTC().Format("2006-01-02T15:04:05.000Z")
			}
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
