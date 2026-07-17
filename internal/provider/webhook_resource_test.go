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

func TestAccWebhookResource(t *testing.T) {
	srv := newWebhooksMockServer()
	t.Cleanup(srv.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(srv.URL) + heredoc.Doc(`
					resource "mailtrap_webhook" "test" {
					  url          = "https://example.com/hook"
					  webhook_type = "email_sending"
					  event_types  = ["delivery", "bounce"]
					}
				`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mailtrap_webhook.test", "url", "https://example.com/hook"),
					resource.TestCheckResourceAttr("mailtrap_webhook.test", "webhook_type", "email_sending"),
					resource.TestCheckResourceAttr("mailtrap_webhook.test", "active", "true"),
					resource.TestCheckResourceAttr("mailtrap_webhook.test", "payload_format", "json"),
					resource.TestCheckResourceAttr("mailtrap_webhook.test", "event_types.#", "2"),
					resource.TestCheckResourceAttrSet("mailtrap_webhook.test", "id"),
					resource.TestCheckResourceAttrSet("mailtrap_webhook.test", "signing_secret"),
				),
			},
			{
				ResourceName:      "mailtrap_webhook.test",
				ImportState:       true,
				ImportStateVerify: true,
				// The signing secret is only returned on creation, so an
				// imported webhook can never recover it.
				ImportStateVerifyIgnore: []string{"signing_secret"},
			},
			{ // change url and deactivate in place; signing_secret must survive
				Config: testProviderConfig(srv.URL) + heredoc.Doc(`
					resource "mailtrap_webhook" "test" {
					  url          = "https://example.com/hook2"
					  webhook_type = "email_sending"
					  active       = false
					  event_types  = ["delivery", "bounce"]
					}
				`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mailtrap_webhook.test", "url", "https://example.com/hook2"),
					resource.TestCheckResourceAttr("mailtrap_webhook.test", "active", "false"),
					resource.TestCheckResourceAttrSet("mailtrap_webhook.test", "signing_secret"),
				),
			},
		},
	})
}

// newWebhooksMockServer is a minimal in-memory stand-in for the Mailtrap
// webhooks API. Single-webhook responses use the {"data": ...} envelope, and
// signing_secret is included only in the create response.
func newWebhooksMockServer() *httptest.Server {
	var (
		mu       sync.Mutex
		webhooks = map[int64]map[string]any{}
		lastID   int64
	)

	writeData := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": v})
	}

	// withoutSecret copies a webhook, dropping the create-only signing secret.
	withoutSecret := func(wh map[string]any) map[string]any {
		out := make(map[string]any, len(wh))
		for k, v := range wh {
			if k != "signing_secret" {
				out[k] = v
			}
		}
		return out
	}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/webhooks", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Webhook map[string]any `json:"webhook"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		defer mu.Unlock()
		lastID++
		wh := map[string]any{
			"id":             lastID,
			"active":         true,
			"payload_format": "json",
			"sending_stream": "transactional",
			"signing_secret": "super-secret",
		}
		for k, v := range body.Webhook {
			wh[k] = v
		}
		webhooks[lastID] = wh
		writeData(w, wh)
	})

	mux.HandleFunc("GET /api/webhooks/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		wh, ok := webhooks[pathID(r)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeData(w, withoutSecret(wh))
	})

	mux.HandleFunc("PATCH /api/webhooks/{id}", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Webhook map[string]any `json:"webhook"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		defer mu.Unlock()
		wh, ok := webhooks[pathID(r)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		for k, v := range body.Webhook {
			wh[k] = v
		}
		writeData(w, withoutSecret(wh))
	})

	mux.HandleFunc("DELETE /api/webhooks/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		wh, ok := webhooks[pathID(r)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		delete(webhooks, pathID(r))
		writeData(w, withoutSecret(wh))
	})

	return httptest.NewServer(mux)
}
