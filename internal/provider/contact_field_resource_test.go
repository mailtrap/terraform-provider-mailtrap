package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccContactFieldResource(t *testing.T) {
	srv := newContactFieldsMockServer()
	t.Cleanup(srv.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(srv.URL) + heredoc.Doc(`
					resource "mailtrap_contact_field" "test" {
					  name      = "Plan"
					  data_type = "text"
					  merge_tag = "plan"
					}
				`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mailtrap_contact_field.test", "name", "Plan"),
					resource.TestCheckResourceAttr("mailtrap_contact_field.test", "data_type", "text"),
					resource.TestCheckResourceAttr("mailtrap_contact_field.test", "merge_tag", "plan"),
					resource.TestCheckResourceAttrSet("mailtrap_contact_field.test", "id"),
				),
			},
			{
				ResourceName:      "mailtrap_contact_field.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{ // rename and change merge tag in place; data_type unchanged
				Config: testProviderConfig(srv.URL) + heredoc.Doc(`
					resource "mailtrap_contact_field" "test" {
					  name      = "Subscription Plan"
					  data_type = "text"
					  merge_tag = "subscription_plan"
					}
				`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mailtrap_contact_field.test", "name", "Subscription Plan"),
					resource.TestCheckResourceAttr("mailtrap_contact_field.test", "merge_tag", "subscription_plan"),
				),
			},
		},
	})
}

// newContactFieldsMockServer is a minimal in-memory stand-in for the Mailtrap
// contact fields API. Requests and responses are plain objects, without the
// wrapping envelope other endpoints use.
func newContactFieldsMockServer() *httptest.Server {
	var (
		mu     sync.Mutex
		fields = map[int64]map[string]any{}
		lastID int64
	)

	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/contacts/fields", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		defer mu.Unlock()
		lastID++
		f := map[string]any{"id": lastID}
		for k, v := range body {
			f[k] = v
		}
		fields[lastID] = f
		writeJSON(w, f)
	})

	mux.HandleFunc("GET /api/contacts/fields/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		f, ok := fields[pathID(r)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, f)
	})

	mux.HandleFunc("PATCH /api/contacts/fields/{id}", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		defer mu.Unlock()
		f, ok := fields[pathID(r)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		for k, v := range body {
			f[k] = v
		}
		writeJSON(w, f)
	})

	mux.HandleFunc("DELETE /api/contacts/fields/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		delete(fields, pathID(r))
		w.WriteHeader(http.StatusNoContent)
	})

	return httptest.NewServer(mux)
}
