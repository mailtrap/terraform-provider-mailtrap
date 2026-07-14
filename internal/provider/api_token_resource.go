package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	_ resource.Resource                = &apiTokenResource{}
	_ resource.ResourceWithConfigure   = &apiTokenResource{}
	_ resource.ResourceWithImportState = &apiTokenResource{}
)

// NewAPITokenResource is the resource factory registered with the provider.
func NewAPITokenResource() resource.Resource {
	return &apiTokenResource{}
}

type apiTokenResource struct {
	client *mailtrap.Client
}

type apiTokenModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Resources   types.List   `tfsdk:"resources"`
	Token       types.String `tfsdk:"token"`
	Last4Digits types.String `tfsdk:"last_4_digits"`
	CreatedBy   types.String `tfsdk:"created_by"`
	ExpiresAt   types.String `tfsdk:"expires_at"`
}

type apiTokenPermissionModel struct {
	ResourceType types.String `tfsdk:"resource_type"`
	ResourceID   types.Int64  `tfsdk:"resource_id"`
	AccessLevel  types.Int64  `tfsdk:"access_level"`
}

var apiTokenPermissionAttrTypes = map[string]attr.Type{
	"resource_type": types.StringType,
	"resource_id":   types.Int64Type,
	"access_level":  types.Int64Type,
}

func (r *apiTokenResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_token"
}

func (r *apiTokenResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Mailtrap API token. The API has no update endpoint, so every configuration change forces replacement.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the API token.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the API token. Changing this forces replacement.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"resources": schema.ListNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Permissions granted to the token. Changing this forces replacement. Drift in permissions made outside Terraform is not detected.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"resource_type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Kind of resource the permission applies to.",
						},
						"resource_id": schema.Int64Attribute{
							Required:            true,
							MarkdownDescription: "ID of the resource the permission applies to.",
						},
						"access_level": schema.Int64Attribute{
							Required:            true,
							MarkdownDescription: "Access level granted on the resource.",
						},
					},
				},
			},
			"token": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Full token value. Returned only on creation; unavailable for imported tokens.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"last_4_digits": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last four characters of the token value.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "User or token that created this token.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"expires_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC 3339 expiry timestamp, or null if the token does not expire.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *apiTokenResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *apiTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiTokenModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &mailtrap.CreateAPITokenRequest{Name: plan.Name.ValueString()}
	if !plan.Resources.IsNull() && !plan.Resources.IsUnknown() {
		var perms []apiTokenPermissionModel
		resp.Diagnostics.Append(plan.Resources.ElementsAs(ctx, &perms, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, p := range perms {
			createReq.Resources = append(createReq.Resources, &mailtrap.APITokenPermission{
				ResourceType: p.ResourceType.ValueString(),
				ResourceID:   p.ResourceID.ValueInt64(),
				AccessLevel:  int(p.AccessLevel.ValueInt64()),
			})
		}
	}

	token, _, err := r.client.APITokens.Create(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating API token", err.Error())
		return
	}

	resp.Diagnostics.Append(flattenAPIToken(ctx, token, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *apiTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiTokenModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token, _, err := r.client.APITokens.Get(ctx, state.ID.ValueInt64())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading API token", err.Error())
		return
	}

	resp.Diagnostics.Append(flattenAPIToken(ctx, token, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never reached: every configurable attribute requires replacement
// because the upstream API has no update endpoint.
func (r *apiTokenResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"API tokens cannot be updated",
		"The Mailtrap API does not support updating API tokens; all changes require replacement. This is a provider bug.",
	)
}

func (r *apiTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiTokenModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.APITokens.Delete(ctx, state.ID.ValueInt64()); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting API token", err.Error())
	}
}

func (r *apiTokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "Expected a numeric API token ID.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func flattenAPIToken(ctx context.Context, t *mailtrap.APIToken, m *apiTokenModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(t.ID)
	m.Name = types.StringValue(t.Name)
	m.Last4Digits = types.StringValue(t.Last4Digits)
	m.CreatedBy = types.StringValue(t.CreatedBy)
	m.ExpiresAt = emptyAsNull(t.ExpiresAt)

	// The full token value is only returned on creation; on every other call
	// keep the value already in state (null for imported tokens).
	if t.Token != "" {
		m.Token = types.StringValue(t.Token)
	} else if m.Token.IsUnknown() {
		m.Token = types.StringNull()
	}

	// Keep configured permissions as-is so API-side normalization (ordering,
	// implicit grants) doesn't force a replacement; only fill them from the
	// API when state has none, i.e. after import.
	if m.Resources.IsNull() || m.Resources.IsUnknown() {
		perms := make([]apiTokenPermissionModel, len(t.Resources))
		for i, p := range t.Resources {
			perms[i] = apiTokenPermissionModel{
				ResourceType: types.StringValue(p.ResourceType),
				ResourceID:   types.Int64Value(p.ResourceID),
				AccessLevel:  types.Int64Value(int64(p.AccessLevel)),
			}
		}
		list, listDiags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: apiTokenPermissionAttrTypes}, perms)
		diags.Append(listDiags...)
		m.Resources = list
	}
	return diags
}
