package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccContactListResource(t *testing.T) {
	srv := newContactListsMockServer()
	t.Cleanup(srv.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(srv.URL) + `
resource "mailtrap_contact_list" "test" {
  name = "Newsletter"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mailtrap_contact_list.test", "name", "Newsletter"),
					resource.TestCheckResourceAttrSet("mailtrap_contact_list.test", "id"),
				),
			},
			{
				ResourceName:      "mailtrap_contact_list.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{ // rename in place
				Config: testProviderConfig(srv.URL) + `
resource "mailtrap_contact_list" "test" {
  name = "Weekly Newsletter"
}`,
				Check: resource.TestCheckResourceAttr("mailtrap_contact_list.test", "name", "Weekly Newsletter"),
			},
		},
	})
}

// newContactListsMockServer is a minimal in-memory stand-in for the Mailtrap
// contact lists API. Requests and responses are plain objects, without the
// wrapping envelope other endpoints use.
func newContactListsMockServer() *httptest.Server {
	var (
		mu     sync.Mutex
		lists  = map[int64]map[string]any{}
		lastID int64
	)

	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/contacts/lists", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string `json:"name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		defer mu.Unlock()
		lastID++
		l := map[string]any{"id": lastID, "name": body.Name}
		lists[lastID] = l
		writeJSON(w, l)
	})

	mux.HandleFunc("GET /api/contacts/lists/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		l, ok := lists[pathID(r)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, l)
	})

	mux.HandleFunc("PATCH /api/contacts/lists/{id}", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string `json:"name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		defer mu.Unlock()
		l, ok := lists[pathID(r)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		l["name"] = body.Name
		writeJSON(w, l)
	})

	mux.HandleFunc("DELETE /api/contacts/lists/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		delete(lists, pathID(r))
		w.WriteHeader(http.StatusNoContent)
	})

	return httptest.NewServer(mux)
}
