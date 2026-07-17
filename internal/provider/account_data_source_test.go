package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAccountDataSource(t *testing.T) {
	srv := newAccountsMockServer()
	t.Cleanup(srv.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{ // lookup by name
				Config: testProviderConfig(srv.URL) + heredoc.Doc(`
					data "mailtrap_account" "test" {
					  name = "Acme"
					}
				`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.mailtrap_account.test", "id", "101"),
					resource.TestCheckResourceAttr("data.mailtrap_account.test", "access_levels.#", "1"),
				),
			},
			{ // no filters with multiple accessible accounts is ambiguous
				Config: testProviderConfig(srv.URL) + heredoc.Doc(`
					data "mailtrap_account" "test" {
					}
				`),
				ExpectError: regexp.MustCompile(`Multiple accounts match`),
			},
			{ // lookup by id
				Config: testProviderConfig(srv.URL) + heredoc.Doc(`
					data "mailtrap_account" "test" {
					  id = 102
					}
				`),
				Check: resource.TestCheckResourceAttr("data.mailtrap_account.test", "name", "Beta Corp"),
			},
		},
	})
}

// newAccountsMockServer serves a fixed two-account list, mirroring a token
// with access to more than one account.
func newAccountsMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/accounts", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 101, "name": "Acme", "access_levels": []int{1000}},
			{"id": 102, "name": "Beta Corp", "access_levels": []int{100}},
		})
	})
	return httptest.NewServer(mux)
}
