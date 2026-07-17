package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	mailtrap "github.com/mailtrap/mailtrap-go"
)

var (
	_ datasource.DataSource              = &accountDataSource{}
	_ datasource.DataSourceWithConfigure = &accountDataSource{}
)

// NewAccountDataSource is the data source factory registered with the provider.
func NewAccountDataSource() datasource.DataSource {
	return &accountDataSource{}
}

type accountDataSource struct {
	client *mailtrap.Client
}

type accountModel struct {
	ID           types.Int64  `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	AccessLevels types.List   `tfsdk:"access_levels"`
}

func (d *accountDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account"
}

func (d *accountDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Mailtrap account the API token has access to. " +
			"Without filters, the token must have access to exactly one account.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the account.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Name of the account.",
			},
			"access_levels": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.Int64Type,
				MarkdownDescription: "The token's access levels for the account.",
			},
		},
	}
}

func (d *accountDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *accountDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config accountModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	accounts, _, err := d.client.Accounts.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing accounts", err.Error())
		return
	}

	var account *mailtrap.Account
	for _, a := range accounts {
		if !config.ID.IsNull() && a.ID != config.ID.ValueInt64() {
			continue
		}
		if !config.Name.IsNull() && a.Name != config.Name.ValueString() {
			continue
		}
		if account != nil {
			resp.Diagnostics.AddError(
				"Multiple accounts match",
				"More than one account matches the given filters. Set `id` or `name` to disambiguate.",
			)
			return
		}
		account = a
	}
	if account == nil {
		resp.Diagnostics.AddError("Account not found", "No account matches the given filters.")
		return
	}

	resp.Diagnostics.Append(flattenAccount(ctx, account, &config)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func flattenAccount(ctx context.Context, a *mailtrap.Account, m *accountModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(a.ID)
	m.Name = types.StringValue(a.Name)

	levels := make([]int64, len(a.AccessLevels))
	for i, l := range a.AccessLevels {
		levels[i] = int64(l)
	}
	list, listDiags := types.ListValueFrom(ctx, types.Int64Type, levels)
	diags.Append(listDiags...)
	m.AccessLevels = list
	return diags
}
