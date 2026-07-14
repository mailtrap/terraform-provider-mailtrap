package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"

	mailtrap "github.com/mailtrap/mailtrap-go"
)

var (
	_ datasource.DataSource                     = &projectDataSource{}
	_ datasource.DataSourceWithConfigure        = &projectDataSource{}
	_ datasource.DataSourceWithConfigValidators = &projectDataSource{}
)

// NewProjectDataSource is the data source factory registered with the provider.
func NewProjectDataSource() datasource.DataSource {
	return &projectDataSource{}
}

type projectDataSource struct {
	client *mailtrap.Client
}

func (d *projectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *projectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Mailtrap project by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the project. Exactly one of `id` and `name` must be set.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Name of the project. Exactly one of `id` and `name` must be set.",
			},
			"share_links": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Public share URLs for the project.",
				Attributes: map[string]schema.Attribute{
					"admin":  schema.StringAttribute{Computed: true},
					"viewer": schema.StringAttribute{Computed: true},
				},
			},
		},
	}
}

func (d *projectDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(path.MatchRoot("id"), path.MatchRoot("name")),
	}
}

func (d *projectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *projectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config projectModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var project *mailtrap.Project
	if !config.ID.IsNull() {
		var err error
		project, _, err = d.client.Projects.Get(ctx, config.ID.ValueInt64())
		if err != nil {
			resp.Diagnostics.AddError("Error reading project", err.Error())
			return
		}
	} else {
		projects, _, err := d.client.Projects.List(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Error listing projects", err.Error())
			return
		}
		name := config.Name.ValueString()
		for _, p := range projects {
			if p.Name != name {
				continue
			}
			if project != nil {
				resp.Diagnostics.AddError(
					"Multiple projects match",
					fmt.Sprintf("More than one project is named %q. Look the project up by `id` instead.", name),
				)
				return
			}
			project = p
		}
		if project == nil {
			resp.Diagnostics.AddError("Project not found", fmt.Sprintf("No project is named %q.", name))
			return
		}
	}

	resp.Diagnostics.Append(flattenProject(project, &config)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
