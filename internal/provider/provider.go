// Package provider implements the Mailtrap Terraform provider.
package provider

import (
	"context"
	"net/http"
	"os"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	mailtrap "github.com/mailtrap/mailtrap-go"
	"golang.org/x/net/http/httpguts"
)

// envAPIToken is read when the api_token attribute is not set in configuration.
const envAPIToken = "MAILTRAP_API_TOKEN"

// reservedHeaders cannot be set through `headers`. The client sets the first
// four itself, and overriding them could break authentication or request and
// response encoding. Go's HTTP client ignores a Host header entry and uses
// the request's Host field instead, so it would be silently dropped.
var reservedHeaders = []string{"Authorization", "User-Agent", "Content-Type", "Accept", "Host"}

var (
	_ provider.Provider                   = &mailtrapProvider{}
	_ provider.ProviderWithValidateConfig = &mailtrapProvider{}
)

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
	Headers  types.Map    `tfsdk:"headers"`
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
			"headers": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Sensitive:   true,
				MarkdownDescription: "Extra HTTP headers sent with every API request, for example credentials for a gateway " +
					"or proxy in front of the API. Values must be known during plan. `Authorization`, `User-Agent`, " +
					"`Content-Type`, `Accept` and `Host` cannot be set.",
			},
		},
	}
}

func (p *mailtrapProvider) ValidateConfig(ctx context.Context, req provider.ValidateConfigRequest, resp *provider.ValidateConfigResponse) {
	var headers types.Map
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("headers"), &headers)...)
	if resp.Diagnostics.HasError() || headers.IsNull() || headers.IsUnknown() {
		return
	}

	names := make([]string, 0, len(headers.Elements()))
	for name := range headers.Elements() {
		names = append(names, name)
	}
	// Sorted so duplicate diagnostics name the same key on every run.
	sort.Strings(names)

	seen := map[string]string{}
	for _, name := range names {
		attr := path.Root("headers").AtMapKey(name)
		if !httpguts.ValidHeaderFieldName(name) {
			resp.Diagnostics.AddAttributeError(attr, "Invalid header name",
				"\""+name+"\" is not a valid HTTP header name.")
			continue
		}
		canonical := http.CanonicalHeaderKey(name)
		for _, reserved := range reservedHeaders {
			if canonical == reserved {
				resp.Diagnostics.AddAttributeError(attr, "Reserved header",
					"The "+reserved+" header cannot be set through `headers`.")
			}
		}
		if prev, ok := seen[canonical]; ok {
			resp.Diagnostics.AddAttributeError(attr, "Duplicate header name",
				"\""+name+"\" and \""+prev+"\" are the same header; HTTP header names are case-insensitive.")
			continue
		}
		seen[canonical] = name
	}
}

// configHeaders converts the `headers` attribute to a plain map. Every value
// must be known: data sources and refreshes call the API during plan, and
// sending them without the extra headers would fail at a gateway.
func configHeaders(ctx context.Context, headers types.Map) (map[string]string, diag.Diagnostics) {
	if headers.IsNull() {
		return nil, nil
	}
	if headers.IsUnknown() {
		return nil, unknownHeadersDiag()
	}

	values := map[string]types.String{}
	if diags := headers.ElementsAs(ctx, &values, false); diags.HasError() {
		return nil, diags
	}

	out := make(map[string]string, len(values))
	for name, v := range values {
		if v.IsUnknown() {
			return nil, unknownHeadersDiag()
		}
		out[name] = v.ValueString()
	}
	return out, nil
}

func unknownHeadersDiag() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddAttributeError(path.Root("headers"), "Unknown header values",
		"`headers` must be known during plan. Use values that do not depend on resources created in the same apply.")
	return diags
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
	headers, diags := configHeaders(ctx, cfg.Headers)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(headers) > 0 {
		opts = append(opts, mailtrap.WithHTTPClient(&http.Client{
			Transport: &headersTransport{headers: headers},
		}))
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
	return []func() resource.Resource{
		NewAPITokenResource,
		NewContactFieldResource,
		NewContactListResource,
		NewEmailTemplateResource,
		NewProjectResource,
		NewSandboxResource,
		NewDomainResource,
		NewWebhookResource,
	}
}

func (p *mailtrapProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewAccountDataSource,
		NewProjectDataSource,
		NewSandboxDataSource,
		NewDomainDataSource,
	}
}
