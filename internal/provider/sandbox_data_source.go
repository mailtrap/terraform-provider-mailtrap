package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	mailtrap "github.com/mailtrap/mailtrap-go"
)

var (
	_ datasource.DataSource                     = &sandboxDataSource{}
	_ datasource.DataSourceWithConfigure        = &sandboxDataSource{}
	_ datasource.DataSourceWithConfigValidators = &sandboxDataSource{}
)

// NewSandboxDataSource is the data source factory registered with the provider.
func NewSandboxDataSource() datasource.DataSource {
	return &sandboxDataSource{}
}

type sandboxDataSource struct {
	client *mailtrap.Client
}

func (d *sandboxDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sandbox"
}

func (d *sandboxDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Mailtrap sandbox (testing inbox) by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the sandbox. Exactly one of `id` and `name` must be set.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Name of the sandbox. Exactly one of `id` and `name` must be set.",
			},
			"project_id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "ID of the project the sandbox belongs to.",
			},
			"email_username": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Username part of the sandbox's email address.",
			},
			"email_username_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the sandbox's email address is enabled.",
			},
			"username": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "SMTP/POP3 username of the sandbox.",
			},
			"password": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "SMTP/POP3 password of the sandbox.",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Status of the sandbox.",
			},
			"domain": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "SMTP domain of the sandbox.",
			},
			"email_domain": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Domain of the sandbox's email address.",
			},
			"pop3_domain": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "POP3 domain of the sandbox.",
			},
			"api_domain": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "API domain of the sandbox.",
			},
			"max_size": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Maximum number of messages the sandbox retains.",
			},
			"max_message_size": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Maximum accepted message size in bytes.",
			},
			"smtp_ports": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.Int64Type,
				MarkdownDescription: "SMTP ports the sandbox listens on.",
			},
			"pop3_ports": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.Int64Type,
				MarkdownDescription: "POP3 ports the sandbox listens on.",
			},
		},
	}
}

func (d *sandboxDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(path.MatchRoot("id"), path.MatchRoot("name")),
	}
}

func (d *sandboxDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *sandboxDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config sandboxModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var sandbox *mailtrap.Sandbox
	if !config.ID.IsNull() {
		var err error
		sandbox, _, err = d.client.Sandboxes.Get(ctx, config.ID.ValueInt64())
		if err != nil {
			resp.Diagnostics.AddError("Error reading sandbox", err.Error())
			return
		}
	} else {
		sandboxes, _, err := d.client.Sandboxes.List(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Error listing sandboxes", err.Error())
			return
		}
		name := config.Name.ValueString()
		for _, s := range sandboxes {
			if s.Name != name {
				continue
			}
			if sandbox != nil {
				resp.Diagnostics.AddError(
					"Multiple sandboxes match",
					fmt.Sprintf("More than one sandbox is named %q. Look the sandbox up by `id` instead.", name),
				)
				return
			}
			sandbox = s
		}
		if sandbox == nil {
			resp.Diagnostics.AddError("Sandbox not found", fmt.Sprintf("No sandbox is named %q.", name))
			return
		}
	}

	resp.Diagnostics.Append(flattenSandbox(ctx, sandbox, &config)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
