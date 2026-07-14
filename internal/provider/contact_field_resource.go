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
	_ resource.Resource                = &contactFieldResource{}
	_ resource.ResourceWithConfigure   = &contactFieldResource{}
	_ resource.ResourceWithImportState = &contactFieldResource{}
)

// NewContactFieldResource is the resource factory registered with the provider.
func NewContactFieldResource() resource.Resource {
	return &contactFieldResource{}
}

type contactFieldResource struct {
	client *mailtrap.Client
}

type contactFieldModel struct {
	ID       types.Int64  `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	DataType types.String `tfsdk:"data_type"`
	MergeTag types.String `tfsdk:"merge_tag"`
}

func (r *contactFieldResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_contact_field"
}

func (r *contactFieldResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a custom contact field that can be set on Mailtrap contacts.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the contact field.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the contact field.",
			},
			"data_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Value type of the field. Changing this forces replacement.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{stringvalidator.OneOf(
					mailtrap.ContactFieldTypeText,
					mailtrap.ContactFieldTypeInteger,
					mailtrap.ContactFieldTypeFloat,
					mailtrap.ContactFieldTypeBoolean,
					mailtrap.ContactFieldTypeDate,
				)},
			},
			"merge_tag": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Merge tag used to personalize campaigns with the field's per-contact value.",
			},
		},
	}
}

func (r *contactFieldResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *contactFieldResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan contactFieldModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	field, _, err := r.client.ContactFields.Create(ctx, &mailtrap.CreateContactFieldRequest{
		Name:     plan.Name.ValueString(),
		DataType: plan.DataType.ValueString(),
		MergeTag: plan.MergeTag.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating contact field", err.Error())
		return
	}

	flattenContactField(field, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *contactFieldResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state contactFieldModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	field, _, err := r.client.ContactFields.Get(ctx, state.ID.ValueInt64())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading contact field", err.Error())
		return
	}

	flattenContactField(field, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *contactFieldResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state contactFieldModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	field, _, err := r.client.ContactFields.Update(ctx, state.ID.ValueInt64(), &mailtrap.UpdateContactFieldRequest{
		Name:     plan.Name.ValueString(),
		MergeTag: plan.MergeTag.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating contact field", err.Error())
		return
	}

	flattenContactField(field, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *contactFieldResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state contactFieldModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.ContactFields.Delete(ctx, state.ID.ValueInt64()); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting contact field", err.Error())
	}
}

func (r *contactFieldResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "Expected a numeric contact field ID.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func flattenContactField(f *mailtrap.ContactField, m *contactFieldModel) {
	m.ID = types.Int64Value(f.ID)
	m.Name = types.StringValue(f.Name)
	m.DataType = types.StringValue(f.DataType)
	m.MergeTag = types.StringValue(f.MergeTag)
}
