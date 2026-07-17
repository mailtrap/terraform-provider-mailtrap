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

func TestAccEmailTemplateResource(t *testing.T) {
	srv := newEmailTemplatesMockServer()
	t.Cleanup(srv.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(srv.URL) + heredoc.Doc(`
					resource "mailtrap_email_template" "test" {
					  name      = "Welcome"
					  category  = "Onboarding"
					  subject   = "Welcome aboard!"
					  body_html = "<h1>Hello</h1>"
					}
				`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mailtrap_email_template.test", "name", "Welcome"),
					resource.TestCheckResourceAttr("mailtrap_email_template.test", "category", "Onboarding"),
					resource.TestCheckResourceAttr("mailtrap_email_template.test", "subject", "Welcome aboard!"),
					resource.TestCheckResourceAttr("mailtrap_email_template.test", "body_html", "<h1>Hello</h1>"),
					resource.TestCheckNoResourceAttr("mailtrap_email_template.test", "body_text"),
					resource.TestCheckResourceAttrSet("mailtrap_email_template.test", "id"),
					resource.TestCheckResourceAttrSet("mailtrap_email_template.test", "uuid"),
				),
			},
			{
				ResourceName:      "mailtrap_email_template.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{ // update subject and add a text body in place
				Config: testProviderConfig(srv.URL) + heredoc.Doc(`
					resource "mailtrap_email_template" "test" {
					  name      = "Welcome"
					  category  = "Onboarding"
					  subject   = "Welcome!"
					  body_text = "Hello"
					  body_html = "<h1>Hello</h1>"
					}
				`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mailtrap_email_template.test", "subject", "Welcome!"),
					resource.TestCheckResourceAttr("mailtrap_email_template.test", "body_text", "Hello"),
				),
			},
		},
	})
}

// newEmailTemplatesMockServer is a minimal in-memory stand-in for the Mailtrap
// email templates API.
func newEmailTemplatesMockServer() *httptest.Server {
	var (
		mu        sync.Mutex
		templates = map[int64]map[string]any{}
		lastID    int64
	)

	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/email_templates", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			EmailTemplate map[string]any `json:"email_template"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		defer mu.Unlock()
		lastID++
		tpl := map[string]any{
			"id":         lastID,
			"uuid":       "00000000-0000-0000-0000-000000000001",
			"created_at": "2026-01-01T00:00:00Z",
			"updated_at": "2026-01-01T00:00:00Z",
		}
		for k, v := range body.EmailTemplate {
			tpl[k] = v
		}
		templates[lastID] = tpl
		writeJSON(w, tpl)
	})

	mux.HandleFunc("GET /api/email_templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		tpl, ok := templates[pathID(r)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, tpl)
	})

	mux.HandleFunc("PATCH /api/email_templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			EmailTemplate map[string]any `json:"email_template"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		defer mu.Unlock()
		tpl, ok := templates[pathID(r)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		for k, v := range body.EmailTemplate {
			tpl[k] = v
		}
		tpl["updated_at"] = "2026-01-02T00:00:00Z"
		writeJSON(w, tpl)
	})

	mux.HandleFunc("DELETE /api/email_templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		delete(templates, pathID(r))
		w.WriteHeader(http.StatusNoContent)
	})

	return httptest.NewServer(mux)
}
