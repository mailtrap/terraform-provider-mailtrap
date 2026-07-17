package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	mailtrap "github.com/mailtrap/mailtrap-go"
)

var (
	_ resource.Resource                = &sandboxResource{}
	_ resource.ResourceWithConfigure   = &sandboxResource{}
	_ resource.ResourceWithImportState = &sandboxResource{}
)

// NewSandboxResource is the resource factory registered with the provider.
func NewSandboxResource() resource.Resource {
	return &sandboxResource{}
}

type sandboxResource struct {
	client *mailtrap.Client
}

// sandboxModel exposes the sandbox's credentials and connection configuration.
// Volatile message counters (emails_count, last_message_sent_at, ...) are
// deliberately omitted so refreshes don't produce perpetual drift.
type sandboxModel struct {
	ID                   types.Int64  `tfsdk:"id"`
	ProjectID            types.Int64  `tfsdk:"project_id"`
	Name                 types.String `tfsdk:"name"`
	EmailUsername        types.String `tfsdk:"email_username"`
	EmailUsernameEnabled types.Bool   `tfsdk:"email_username_enabled"`
	Username             types.String `tfsdk:"username"`
	Password             types.String `tfsdk:"password"`
	Status               types.String `tfsdk:"status"`
	Domain               types.String `tfsdk:"domain"`
	EmailDomain          types.String `tfsdk:"email_domain"`
	POP3Domain           types.String `tfsdk:"pop3_domain"`
	APIDomain            types.String `tfsdk:"api_domain"`
	MaxSize              types.Int64  `tfsdk:"max_size"`
	MaxMessageSize       types.Int64  `tfsdk:"max_message_size"`
	SMTPPorts            types.List   `tfsdk:"smtp_ports"`
	POP3Ports            types.List   `tfsdk:"pop3_ports"`
}

func (r *sandboxResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sandbox"
}

func (r *sandboxResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Mailtrap sandbox (testing inbox) inside a project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the sandbox.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"project_id": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "ID of the project the sandbox belongs to. Changing this forces replacement.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the sandbox.",
			},
			"email_username": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Username part of the sandbox's email address.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"email_username_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the sandbox's email address is enabled.",
			},
			"username": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "SMTP/POP3 username of the sandbox.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"password": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "SMTP/POP3 password of the sandbox.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Status of the sandbox.",
			},
			"domain": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "SMTP domain of the sandbox.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"email_domain": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Domain of the sandbox's email address.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"pop3_domain": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "POP3 domain of the sandbox.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"api_domain": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "API domain of the sandbox.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"max_size": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Maximum number of messages the sandbox retains.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"max_message_size": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Maximum accepted message size in bytes.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"smtp_ports": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.Int64Type,
				MarkdownDescription: "SMTP ports the sandbox listens on.",
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"pop3_ports": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.Int64Type,
				MarkdownDescription: "POP3 ports the sandbox listens on.",
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *sandboxResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.client = client
}

func (r *sandboxResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sandboxModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Remember the configured email username before flattenSandbox overwrites
	// the plan with the create response.
	emailUsername := plan.EmailUsername

	sandbox, _, err := r.client.Sandboxes.Create(ctx, plan.ProjectID.ValueInt64(), plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error creating sandbox", err.Error())
		return
	}

	// Persist the created sandbox right away so a failure in the follow-up
	// update below doesn't leave it orphaned outside of state.
	resp.Diagnostics.Append(flattenSandbox(ctx, sandbox, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create only accepts the name; apply a configured email username with a
	// follow-up update.
	if !emailUsername.IsNull() && !emailUsername.IsUnknown() {
		sandbox, _, err = r.client.Sandboxes.Update(ctx, sandbox.ID, &mailtrap.SandboxUpdateRequest{
			EmailUsername: emailUsername.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Error setting sandbox email username", err.Error())
			return
		}
		resp.Diagnostics.Append(flattenSandbox(ctx, sandbox, &plan)...)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}

func (r *sandboxResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sandboxModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sandbox, _, err := r.client.Sandboxes.Get(ctx, state.ID.ValueInt64())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading sandbox", err.Error())
		return
	}

	resp.Diagnostics.Append(flattenSandbox(ctx, sandbox, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sandboxResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, config sandboxModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	upd := &mailtrap.SandboxUpdateRequest{Name: plan.Name.ValueString()}
	if !config.EmailUsername.IsNull() && !plan.EmailUsername.Equal(state.EmailUsername) {
		upd.EmailUsername = plan.EmailUsername.ValueString()
	}
	sandbox, _, err := r.client.Sandboxes.Update(ctx, state.ID.ValueInt64(), upd)
	if err != nil {
		resp.Diagnostics.AddError("Error updating sandbox", err.Error())
		return
	}

	resp.Diagnostics.Append(flattenSandbox(ctx, sandbox, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sandboxResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sandboxModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, _, err := r.client.Sandboxes.Delete(ctx, state.ID.ValueInt64()); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting sandbox", err.Error())
	}
}

func (r *sandboxResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "Expected a numeric sandbox ID.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func flattenSandbox(ctx context.Context, s *mailtrap.Sandbox, m *sandboxModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(s.ID)
	m.ProjectID = types.Int64Value(s.ProjectID)
	m.Name = types.StringValue(s.Name)
	m.EmailUsername = types.StringValue(s.EmailUsername)
	m.EmailUsernameEnabled = types.BoolValue(s.EmailUsernameEnabled)
	m.Username = types.StringValue(s.Username)
	m.Password = types.StringValue(s.Password)
	m.Status = types.StringValue(s.Status)
	m.Domain = types.StringValue(s.Domain)
	m.EmailDomain = types.StringValue(s.EmailDomain)
	m.POP3Domain = types.StringValue(s.POP3Domain)
	m.APIDomain = types.StringValue(s.APIDomain)
	m.MaxSize = types.Int64Value(s.MaxSize)
	m.MaxMessageSize = types.Int64Value(s.MaxMessageSize)

	m.SMTPPorts = flattenPorts(ctx, s.SMTPPorts, &diags)
	m.POP3Ports = flattenPorts(ctx, s.POP3Ports, &diags)
	return diags
}

func flattenPorts(ctx context.Context, ports []int, diags *diag.Diagnostics) types.List {
	vals := make([]int64, len(ports))
	for i, p := range ports {
		vals[i] = int64(p)
	}
	list, listDiags := types.ListValueFrom(ctx, types.Int64Type, vals)
	diags.Append(listDiags...)
	return list
}
