package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSendingDomainDataSource(t *testing.T) {
	srv := newDomainsMockServer()
	t.Cleanup(srv.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig(srv.URL) + `
resource "mailtrap_sending_domain" "test" {
  domain_name = "example.com"
}

data "mailtrap_sending_domain" "test" {
  id = mailtrap_sending_domain.test.id
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.mailtrap_sending_domain.test", "domain_name", "example.com"),
					resource.TestCheckResourceAttr("data.mailtrap_sending_domain.test", "compliance_status", "pending"),
					resource.TestCheckResourceAttr("data.mailtrap_sending_domain.test", "dns_records.#", "1"),
				),
			},
		},
	})
}
