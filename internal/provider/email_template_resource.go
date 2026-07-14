package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	mailtrap "github.com/mailtrap/mailtrap-go"
)

var (
	_ resource.Resource                = &emailTemplateResource{}
	_ resource.ResourceWithConfigure   = &emailTemplateResource{}
	_ resource.ResourceWithImportState = &emailTemplateResource{}
)

// NewEmailTemplateResource is the resource factory registered with the provider.
func NewEmailTemplateResource() resource.Resource {
	return &emailTemplateResource{}
}

type emailTemplateResource struct {
	client *mailtrap.Client
}

type emailTemplateModel struct {
	ID        types.Int64  `tfsdk:"id"`
	UUID      types.String `tfsdk:"uuid"`
	Name      types.String `tfsdk:"name"`
	Category  types.String `tfsdk:"category"`
	Subject   types.String `tfsdk:"subject"`
	BodyText  types.String `tfsdk:"body_text"`
	BodyHTML  types.String `tfsdk:"body_html"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func (r *emailTemplateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_email_template"
}

func (r *emailTemplateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a reusable Mailtrap email template.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the template.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"uuid": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "UUID of the template.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the template.",
			},
			"category": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Category of the template.",
			},
			"subject": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Subject line of emails sent with the template.",
			},
			"body_text": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Plain-text body. The API cannot clear a body once set: removing this attribute keeps the last value on the server.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"body_html": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "HTML body. The API cannot clear a body once set: removing this attribute keeps the last value on the server.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last update timestamp.",
			},
		},
	}
}

func (r *emailTemplateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *emailTemplateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan emailTemplateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	template, _, err := r.client.EmailTemplates.Create(ctx, emailTemplateRequest(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating email template", err.Error())
		return
	}

	flattenEmailTemplate(template, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *emailTemplateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state emailTemplateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	template, _, err := r.client.EmailTemplates.Get(ctx, state.ID.ValueInt64())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading email template", err.Error())
		return
	}

	flattenEmailTemplate(template, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *emailTemplateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state emailTemplateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	template, _, err := r.client.EmailTemplates.Update(ctx, state.ID.ValueInt64(), emailTemplateRequest(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating email template", err.Error())
		return
	}

	flattenEmailTemplate(template, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *emailTemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state emailTemplateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.EmailTemplates.Delete(ctx, state.ID.ValueInt64()); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting email template", err.Error())
	}
}

func (r *emailTemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "Expected a numeric email template ID.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func emailTemplateRequest(m emailTemplateModel) *mailtrap.EmailTemplateRequest {
	req := &mailtrap.EmailTemplateRequest{
		Name:     m.Name.ValueString(),
		Category: m.Category.ValueString(),
		Subject:  m.Subject.ValueString(),
	}
	if !m.BodyText.IsNull() && !m.BodyText.IsUnknown() {
		req.BodyText = m.BodyText.ValueString()
	}
	if !m.BodyHTML.IsNull() && !m.BodyHTML.IsUnknown() {
		req.BodyHTML = m.BodyHTML.ValueString()
	}
	return req
}

func flattenEmailTemplate(t *mailtrap.EmailTemplate, m *emailTemplateModel) {
	m.ID = types.Int64Value(t.ID)
	m.UUID = types.StringValue(t.UUID)
	m.Name = types.StringValue(t.Name)
	m.Category = types.StringValue(t.Category)
	m.Subject = types.StringValue(t.Subject)
	m.BodyText = emptyAsNull(t.BodyText)
	m.BodyHTML = emptyAsNull(t.BodyHTML)
	m.CreatedAt = types.StringValue(t.CreatedAt)
	m.UpdatedAt = types.StringValue(t.UpdatedAt)
}

// emptyAsNull maps the API's empty string (body never set) to a null value so
// unset optional bodies stay null in state.
func emptyAsNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
