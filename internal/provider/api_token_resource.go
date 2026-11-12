package provider

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
		MarkdownDescription: "Manages a Mailtrap API token. The API has no update endpoint, so every configuration change forces replacement, " +
			"except writing `expires_at` differently as the same time.",
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
				Optional: true,
				Computed: true,
				MarkdownDescription: "When the token expires: an RFC 3339 timestamp in whole seconds (e.g. `2027-06-01T00:00:00Z`), or `" + tokenNeverExpires + "` " +
					"for a token that does not expire. Leave unset for the server default of 1 year from creation. A timestamp must be in the future and at " +
					"most 5 years ahead, or the API rejects it. The API sets the expiry only on create, so changing it to a different time forces " +
					"replacement; writing the same time differently (e.g. with an offset) only updates state. Resetting the token in Mailtrap issues " +
					"a new token outside Terraform and expires this one 12 hours later; if `expires_at` is set, the next plan replaces it.",
				Validators: []validator.String{
					stringvalidator.Any(
						stringvalidator.OneOf(tokenNeverExpires),
						rfc3339Validator{},
					),
				},
				// Unset keeps the prior value. Only a configured, different time
				// replaces; the same instant spelled differently reaches Update.
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplaceIf(
						requiresReplaceIfExpiryChanges,
						"Changing expires_at to a different time replaces the token.",
						"Changing `expires_at` to a different time replaces the token.",
					),
				},
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

	if !plan.ExpiresAt.IsNull() && !plan.ExpiresAt.IsUnknown() {
		if v := plan.ExpiresAt.ValueString(); v == tokenNeverExpires {
			createReq.ExpiresAt = mailtrap.NeverExpires()
		} else {
			createReq.ExpiresAt = mailtrap.ExpiresAt(v)
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

// Update only stores expires_at. The API has no update endpoint, so every
// other attribute requires replacement, and expires_at reaches Update only when
// it names the same instant as state in a different spelling.
func (r *apiTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state apiTokenModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A different instant would need a new token; planning should have
	// replaced it, so fail rather than record an expiry the token lacks.
	if !sameTokenExpiry(state.ExpiresAt.ValueString(), plan.ExpiresAt.ValueString()) {
		resp.Diagnostics.AddError(
			"API token expiry cannot be updated",
			fmt.Sprintf("expires_at is set only when a token is created, so changing it from %q to %q requires replacement. This is a provider bug.",
				state.ExpiresAt.ValueString(), plan.ExpiresAt.ValueString()),
		)
		return
	}

	state.ExpiresAt = plan.ExpiresAt
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
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
	m.ExpiresAt = flattenTokenExpiry(m.ExpiresAt, t.ExpiresAt)

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

// tokenNeverExpires is the expires_at value for a token without an expiry; the
// API reports one as a null expires_at.
const tokenNeverExpires = "never"

// requiresReplaceIfExpiryChanges replaces the token when a configured
// expires_at names a different instant than state, not when it only spells the
// same one differently (an imported token holds the API's spelling). A value
// unknown until apply may be any instant, so it replaces, as RequiresReplace
// does: the API sets the expiry only on create.
func requiresReplaceIfExpiryChanges(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
	if req.ConfigValue.IsUnknown() {
		resp.RequiresReplace = true
		return
	}
	if req.ConfigValue.IsNull() || req.StateValue.IsNull() {
		return
	}
	resp.RequiresReplace = !sameTokenExpiry(req.StateValue.ValueString(), req.PlanValue.ValueString())
}

// flattenTokenExpiry maps the API's expiry (empty for none) onto expires_at,
// keeping the prior value when it names the same instant, so config written
// with an offset, or state from an earlier version, is kept as it is rather
// than rewritten in the API's spelling.
func flattenTokenExpiry(prior types.String, api string) types.String {
	value := api
	if value == "" {
		value = tokenNeverExpires
	}
	if !prior.IsNull() && !prior.IsUnknown() && sameTokenExpiry(prior.ValueString(), value) {
		return prior
	}
	// With no prior value (import, or create without expires_at), store the
	// canonical UTC form, as expires_at is usually written. The API already
	// returns that; this only normalises another spelling, should it change.
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		value = t.UTC().Format(time.RFC3339Nano)
	}
	return types.StringValue(value)
}

func sameTokenExpiry(a, b string) bool {
	if a == b {
		return true
	}
	ta, errA := time.Parse(time.RFC3339, a)
	tb, errB := time.Parse(time.RFC3339, b)
	return errA == nil && errB == nil && ta.Equal(tb)
}

// rfc3339Validator accepts an RFC 3339 timestamp in whole seconds. The API
// returns expiries without fractions, so a fractional one would read back as a
// different instant and plan a replacement on every run.
type rfc3339Validator struct{}

func (rfc3339Validator) Description(context.Context) string {
	return "value must be an RFC 3339 timestamp in whole seconds"
}

func (v rfc3339Validator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v rfc3339Validator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	t, err := time.Parse(time.RFC3339, req.ConfigValue.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid expires_at", v.Description(ctx)+": "+err.Error())
		return
	}
	if t.Nanosecond() != 0 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid expires_at", v.Description(ctx)+": the API keeps whole seconds only")
	}
}
