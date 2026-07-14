package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSendingDomainResource(t *testing.T) {
	srv := newDomainsMockServer()
	t.Cleanup(srv.Close)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{ // create + follow-up tracking update
				Config: testProviderConfig(srv.URL) + `
resource "mailtrap_sending_domain" "test" {
  domain_name           = "example.com"
  open_tracking_enabled = true
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mailtrap_sending_domain.test", "domain_name", "example.com"),
					resource.TestCheckResourceAttr("mailtrap_sending_domain.test", "open_tracking_enabled", "true"),
					resource.TestCheckResourceAttr("mailtrap_sending_domain.test", "click_tracking_enabled", "true"),
					resource.TestCheckResourceAttr("mailtrap_sending_domain.test", "compliance_status", "pending"),
					resource.TestCheckResourceAttrSet("mailtrap_sending_domain.test", "id"),
					resource.TestCheckResourceAttr("mailtrap_sending_domain.test", "dns_records.#", "1"),
				),
			},
			{ // import round-trips through Read
				ResourceName:      "mailtrap_sending_domain.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{ // update the tracking flag in place
				Config: testProviderConfig(srv.URL) + `
resource "mailtrap_sending_domain" "test" {
  domain_name           = "example.com"
  open_tracking_enabled = false
}`,
				Check: resource.TestCheckResourceAttr("mailtrap_sending_domain.test", "open_tracking_enabled", "false"),
			},
		},
	})
}

func testProviderConfig(baseURL string) string {
	return fmt.Sprintf(`
provider "mailtrap" {
  api_token = "test-token"
  base_url  = %q
}
`, baseURL)
}

// newDomainsMockServer is a minimal in-memory stand-in for the Mailtrap
// sending-domains API — enough create/get/update/delete to drive the resource
// without real credentials.
func newDomainsMockServer() *httptest.Server {
	var (
		mu      sync.Mutex
		domains = map[int64]map[string]any{}
		lastID  int64
	)

	newDomain := func(id int64, name string) map[string]any {
		return map[string]any{
			"id":                            id,
			"domain_name":                   name,
			"demo":                          false,
			"compliance_status":             "pending",
			"dns_verified":                  false,
			"open_tracking_enabled":         false,
			"click_tracking_enabled":        true,
			"auto_unsubscribe_link_enabled": false,
			"dns_records": []any{map[string]any{
				"key": "dkim", "domain": name, "name": "dkim._domainkey",
				"status": "pending", "type": "CNAME", "value": "dkim.mailtrap.io",
			}},
		}
	}

	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/domains", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Domain struct {
				DomainName string `json:"domain_name"`
			} `json:"domain"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		defer mu.Unlock()
		lastID++
		d := newDomain(lastID, body.Domain.DomainName)
		domains[lastID] = d
		writeJSON(w, d)
	})

	mux.HandleFunc("GET /api/domains/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		d, ok := domains[pathID(r)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, d)
	})

	mux.HandleFunc("PATCH /api/domains/{id}", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Domain map[string]any `json:"domain"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		defer mu.Unlock()
		d, ok := domains[pathID(r)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		for k, v := range body.Domain {
			d[k] = v
		}
		writeJSON(w, d)
	})

	mux.HandleFunc("DELETE /api/domains/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		delete(domains, pathID(r))
		w.WriteHeader(http.StatusNoContent)
	})

	return httptest.NewServer(mux)
}

func pathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}
