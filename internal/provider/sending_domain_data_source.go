package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	mailtrap "github.com/mailtrap/mailtrap-go"
)

var (
	_ datasource.DataSource              = &sendingDomainDataSource{}
	_ datasource.DataSourceWithConfigure = &sendingDomainDataSource{}
)

// NewSendingDomainDataSource is the data source factory registered with the provider.
func NewSendingDomainDataSource() datasource.DataSource {
	return &sendingDomainDataSource{}
}

type sendingDomainDataSource struct {
	client *mailtrap.Client
}

func (d *sendingDomainDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sending_domain"
}

func (d *sendingDomainDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Mailtrap sending domain, including the DNS records required to authenticate it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Numeric identifier of the sending domain.",
			},
			"domain_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The domain name.",
			},
			"open_tracking_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether open tracking is enabled.",
			},
			"click_tracking_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether click tracking is enabled.",
			},
			"auto_unsubscribe_link_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the automatic unsubscribe link is enabled.",
			},
			"compliance_status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Compliance review status of the domain.",
			},
			"dns_verified": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the domain's DNS records have been verified.",
			},
			"demo": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this is a demo domain.",
			},
			"dns_records": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "DNS records to publish to authenticate the domain.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key":    schema.StringAttribute{Computed: true},
						"domain": schema.StringAttribute{Computed: true},
						"name":   schema.StringAttribute{Computed: true},
						"status": schema.StringAttribute{Computed: true},
						"type":   schema.StringAttribute{Computed: true},
						"value":  schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *sendingDomainDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*mailtrap.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("Expected *mailtrap.Client, got %T. This is a provider bug.", req.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *sendingDomainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config sendingDomainModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain, _, err := d.client.SendingDomains.Get(ctx, config.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Error reading sending domain", err.Error())
		return
	}

	resp.Diagnostics.Append(flattenSendingDomain(ctx, domain, &config)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
