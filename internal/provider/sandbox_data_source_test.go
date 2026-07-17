package provider

import (
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSandboxDataSource(t *testing.T) {
	srv := newSandboxesMockServer()
	t.Cleanup(srv.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{ // lookup by name
				Config: testProviderConfig(srv.URL) + heredoc.Doc(`
					resource "mailtrap_sandbox" "test" {
					  project_id = 1
					  name       = "Staging"
					}

					data "mailtrap_sandbox" "by_name" {
					  name = mailtrap_sandbox.test.name
					}
				`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.mailtrap_sandbox.by_name", "id", "mailtrap_sandbox.test", "id"),
					resource.TestCheckResourceAttr("data.mailtrap_sandbox.by_name", "project_id", "1"),
					resource.TestCheckResourceAttrSet("data.mailtrap_sandbox.by_name", "username"),
				),
			},
			{ // lookup by id
				Config: testProviderConfig(srv.URL) + heredoc.Doc(`
					resource "mailtrap_sandbox" "test" {
					  project_id = 1
					  name       = "Staging"
					}

					data "mailtrap_sandbox" "by_id" {
					  id = mailtrap_sandbox.test.id
					}
				`),
				Check: resource.TestCheckResourceAttr("data.mailtrap_sandbox.by_id", "name", "Staging"),
			},
		},
	})
}
