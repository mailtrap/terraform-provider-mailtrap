package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSandboxResource(t *testing.T) {
	srv := newSandboxesMockServer()
	t.Cleanup(srv.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{ // create + follow-up email_username update
				Config: testProviderConfig(srv.URL) + `
resource "mailtrap_sandbox" "test" {
  project_id     = 1
  name           = "Staging"
  email_username = "staging-mail"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mailtrap_sandbox.test", "name", "Staging"),
					resource.TestCheckResourceAttr("mailtrap_sandbox.test", "project_id", "1"),
					resource.TestCheckResourceAttr("mailtrap_sandbox.test", "email_username", "staging-mail"),
					resource.TestCheckResourceAttrSet("mailtrap_sandbox.test", "id"),
					resource.TestCheckResourceAttrSet("mailtrap_sandbox.test", "username"),
					resource.TestCheckResourceAttrSet("mailtrap_sandbox.test", "password"),
					resource.TestCheckResourceAttr("mailtrap_sandbox.test", "smtp_ports.#", "2"),
				),
			},
			{
				ResourceName:      "mailtrap_sandbox.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{ // rename in place
				Config: testProviderConfig(srv.URL) + `
resource "mailtrap_sandbox" "test" {
  project_id     = 1
  name           = "Staging 2"
  email_username = "staging-mail"
}`,
				Check: resource.TestCheckResourceAttr("mailtrap_sandbox.test", "name", "Staging 2"),
			},
		},
	})
}

// newSandboxesMockServer is a minimal in-memory stand-in for the Mailtrap
// sandboxes API.
func newSandboxesMockServer() *httptest.Server {
	var (
		mu        sync.Mutex
		sandboxes = map[int64]map[string]any{}
		lastID    int64
	)

	newSandbox := func(id, projectID int64, name string) map[string]any {
		return map[string]any{
			"id":                     id,
			"name":                   name,
			"project_id":             projectID,
			"username":               "smtp-user",
			"password":               "smtp-pass",
			"status":                 "active",
			"email_username":         "generated",
			"email_username_enabled": false,
			"domain":                 "smtp.mailtrap.io",
			"email_domain":           "inbox.mailtrap.io",
			"pop3_domain":            "pop3.mailtrap.io",
			"api_domain":             "mailtrap.io",
			"max_size":               100,
			"max_message_size":       5242880,
			"smtp_ports":             []int{25, 587},
			"pop3_ports":             []int{1100, 9950},
		}
	}

	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/projects/{id}/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Sandbox struct {
				Name string `json:"name"`
			} `json:"sandbox"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		defer mu.Unlock()
		lastID++
		s := newSandbox(lastID, pathID(r), body.Sandbox.Name)
		sandboxes[lastID] = s
		writeJSON(w, s)
	})

	mux.HandleFunc("GET /api/sandboxes/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		s, ok := sandboxes[pathID(r)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, s)
	})

	mux.HandleFunc("PATCH /api/sandboxes/{id}", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Sandbox map[string]any `json:"sandbox"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		defer mu.Unlock()
		s, ok := sandboxes[pathID(r)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		for k, v := range body.Sandbox {
			s[k] = v
		}
		writeJSON(w, s)
	})

	mux.HandleFunc("DELETE /api/sandboxes/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		s, ok := sandboxes[pathID(r)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		delete(sandboxes, pathID(r))
		writeJSON(w, s)
	})

	return httptest.NewServer(mux)
}
