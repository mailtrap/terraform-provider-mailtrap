package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProjectResource(t *testing.T) {
	srv := newProjectsMockServer()
	t.Cleanup(srv.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(srv.URL) + `
resource "mailtrap_project" "test" {
  name = "My Project"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mailtrap_project.test", "name", "My Project"),
					resource.TestCheckResourceAttrSet("mailtrap_project.test", "id"),
					resource.TestCheckResourceAttrSet("mailtrap_project.test", "share_links.admin"),
				),
			},
			{
				ResourceName:      "mailtrap_project.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{ // rename in place
				Config: testProviderConfig(srv.URL) + `
resource "mailtrap_project" "test" {
  name = "Renamed Project"
}`,
				Check: resource.TestCheckResourceAttr("mailtrap_project.test", "name", "Renamed Project"),
			},
		},
	})
}

// newProjectsMockServer is a minimal in-memory stand-in for the Mailtrap
// projects API.
func newProjectsMockServer() *httptest.Server {
	var (
		mu       sync.Mutex
		projects = map[int64]map[string]any{}
		lastID   int64
	)

	newProject := func(id int64, name string) map[string]any {
		return map[string]any{
			"id":   id,
			"name": name,
			"share_links": map[string]any{
				"admin":  "https://mailtrap.io/share/admin",
				"viewer": "https://mailtrap.io/share/viewer",
			},
		}
	}

	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/projects", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Project struct {
				Name string `json:"name"`
			} `json:"project"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		defer mu.Unlock()
		lastID++
		p := newProject(lastID, body.Project.Name)
		projects[lastID] = p
		writeJSON(w, p)
	})

	mux.HandleFunc("GET /api/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		p, ok := projects[pathID(r)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, p)
	})

	mux.HandleFunc("PATCH /api/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Project struct {
				Name string `json:"name"`
			} `json:"project"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		defer mu.Unlock()
		p, ok := projects[pathID(r)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		p["name"] = body.Project.Name
		writeJSON(w, p)
	})

	mux.HandleFunc("DELETE /api/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		delete(projects, pathID(r))
		w.WriteHeader(http.StatusNoContent)
	})

	return httptest.NewServer(mux)
}
