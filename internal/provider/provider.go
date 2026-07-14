// Package provider implements the Mailtrap Terraform provider.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	mailtrap "github.com/mailtrap/mailtrap-go"
)

// envAPIToken is read when the api_token attribute is not set in configuration.
const envAPIToken = "MAILTRAP_API_TOKEN"

var _ provider.Provider = &mailtrapProvider{}

type mailtrapProvider struct {
	version string
}

// New returns the provider factory used by the plugin server and acceptance tests.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &mailtrapProvider{version: version}
	}
}

type mailtrapProviderModel struct {
	APIToken types.String `tfsdk:"api_token"`
	BaseURL  types.String `tfsdk:"base_url"`
}

func (p *mailtrapProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "mailtrap"
	resp.Version = p.version
}

func (p *mailtrapProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage Mailtrap resources through the Mailtrap API.",
		Attributes: map[string]schema.Attribute{
			"api_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Mailtrap API token. May also be set via the `" + envAPIToken + "` environment variable.",
			},
			"base_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Override the general-host API base URL. Intended for testing.",
			},
		},
	}
}

func (p *mailtrapProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg mailtrapProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token := os.Getenv(envAPIToken)
	if !cfg.APIToken.IsNull() && cfg.APIToken.ValueString() != "" {
		token = cfg.APIToken.ValueString()
	}
	if token == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Missing Mailtrap API token",
			"Set the provider `api_token` attribute or the "+envAPIToken+" environment variable.",
		)
		return
	}

	var opts []mailtrap.Option
	if !cfg.BaseURL.IsNull() && cfg.BaseURL.ValueString() != "" {
		opts = append(opts, mailtrap.WithBaseURL(mailtrap.HostGeneral, cfg.BaseURL.ValueString()))
	}

	client, err := mailtrap.NewClient(token, opts...)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Mailtrap client", err.Error())
		return
	}

	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *mailtrapProvider) Resources(_ context.Context) []func() resource.Resource {
	// No resources yet.
	return nil
}

func (p *mailtrapProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
