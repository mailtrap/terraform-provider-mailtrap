package provider

import (
	"regexp"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccProjectDataSource(t *testing.T) {
	srv := newProjectsMockServer()
	t.Cleanup(srv.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{ // lookup by name
				Config: testProviderConfig(srv.URL) + heredoc.Doc(`
					resource "mailtrap_project" "test" {
					  name = "My Project"
					}

					data "mailtrap_project" "by_name" {
					  name = mailtrap_project.test.name
					}
				`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.mailtrap_project.by_name", "id", "mailtrap_project.test", "id"),
					resource.TestCheckResourceAttrSet("data.mailtrap_project.by_name", "share_links.admin"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectSensitiveValue("data.mailtrap_project.by_name", tfjsonpath.New("share_links")),
					statecheck.ExpectSensitiveValue("mailtrap_project.test", tfjsonpath.New("share_links")),
				},
			},
			{ // setting both filters is rejected
				Config: testProviderConfig(srv.URL) + heredoc.Doc(`
					data "mailtrap_project" "invalid" {
					  id   = 1
					  name = "My Project"
					}
				`),
				ExpectError: regexp.MustCompile(`Exactly one of these attributes must be configured`),
			},
			{ // lookup by id
				Config: testProviderConfig(srv.URL) + heredoc.Doc(`
					resource "mailtrap_project" "test" {
					  name = "My Project"
					}

					data "mailtrap_project" "by_id" {
					  id = mailtrap_project.test.id
					}
				`),
				Check: resource.TestCheckResourceAttr("data.mailtrap_project.by_id", "name", "My Project"),
			},
		},
	})
}
