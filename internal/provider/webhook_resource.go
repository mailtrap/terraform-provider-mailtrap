package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	mailtrap "github.com/mailtrap/mailtrap-go"
)

var (
	_ resource.Resource                = &webhookResource{}
	_ resource.ResourceWithConfigure   = &webhookResource{}
	_ resource.ResourceWithImportState = &webhookResource{}
)

// NewWebhookResource is the resource factory registered with the provider.
func NewWebhookResource() resource.Resource {
	return &webhookResource{}
}

type webhookResource struct {
	client *mailtrap.Client
}

type webhookModel struct {
	ID             types.Int64  `tfsdk:"id"`
	URL            types.String `tfsdk:"url"`
	WebhookType    types.String `tfsdk:"webhook_type"`
	Active         types.Bool   `tfsdk:"active"`
	PayloadFormat  types.String `tfsdk:"payload_format"`
	SendingStream  types.String `tfsdk:"sending_stream"`
	DomainID       types.Int64  `tfsdk:"domain_id"`
	InboundInboxID types.Int64  `tfsdk:"inbound_inbox_id"`
	EventTypes     types.List   `tfsdk:"event_types"`
	SigningSecret  types.String `tfsdk:"signing_secret"`
}

func (r *webhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *webhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Mailtrap webhook that delivers event notifications to a URL.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the webhook.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"url": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "URL event payloads are delivered to.",
			},
			"webhook_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Kind of webhook. Changing this forces replacement.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{stringvalidator.OneOf(
					mailtrap.WebhookTypeEmailSending,
					mailtrap.WebhookTypeCampaigns,
					mailtrap.WebhookTypeAuditLog,
					mailtrap.WebhookTypeInboundReceiving,
				)},
			},
			"active": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the webhook is active. Defaults to `true`.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"payload_format": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Encoding of delivered payloads.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators: []validator.String{stringvalidator.OneOf(
					mailtrap.PayloadFormatJSON,
					mailtrap.PayloadFormatJSONLines,
				)},
			},
			"sending_stream": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Sending stream the webhook applies to (`email_sending` webhooks only). Changing this forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{stringvalidator.OneOf("transactional", "bulk")},
			},
			"domain_id": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Domain the webhook is scoped to. Changing this forces replacement.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"inbound_inbox_id": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Inbound inbox the webhook is scoped to. Changing this forces replacement.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"event_types": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Event types the webhook subscribes to (`email_sending` and `campaigns` webhooks).",
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				Validators: []validator.List{listvalidator.ValueStringsAre(stringvalidator.OneOf(
					mailtrap.WebhookEventDelivery,
					mailtrap.WebhookEventSoftBounce,
					mailtrap.WebhookEventBounce,
					mailtrap.WebhookEventSuspension,
					mailtrap.WebhookEventUnsubscribe,
					mailtrap.WebhookEventOpen,
					mailtrap.WebhookEventSpamComplaint,
					mailtrap.WebhookEventClick,
					mailtrap.WebhookEventReject,
				))},
			},
			"signing_secret": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "HMAC SHA-256 secret for verifying payload signatures. Returned only on creation; unavailable for imported webhooks.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *webhookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *webhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan webhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &mailtrap.CreateWebhookRequest{
		URL:         plan.URL.ValueString(),
		WebhookType: plan.WebhookType.ValueString(),
	}
	if !plan.Active.IsNull() && !plan.Active.IsUnknown() {
		createReq.Active = mailtrap.Ptr(plan.Active.ValueBool())
	}
	if !plan.PayloadFormat.IsNull() && !plan.PayloadFormat.IsUnknown() {
		createReq.PayloadFormat = plan.PayloadFormat.ValueString()
	}
	if !plan.SendingStream.IsNull() && !plan.SendingStream.IsUnknown() {
		createReq.SendingStream = plan.SendingStream.ValueString()
	}
	if !plan.DomainID.IsNull() {
		createReq.DomainID = mailtrap.Ptr(plan.DomainID.ValueInt64())
	}
	if !plan.InboundInboxID.IsNull() {
		createReq.InboundInboxID = mailtrap.Ptr(plan.InboundInboxID.ValueInt64())
	}
	if !plan.EventTypes.IsNull() && !plan.EventTypes.IsUnknown() {
		resp.Diagnostics.Append(plan.EventTypes.ElementsAs(ctx, &createReq.EventTypes, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	webhook, _, err := r.client.Webhooks.Create(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating webhook", err.Error())
		return
	}

	resp.Diagnostics.Append(flattenWebhook(ctx, webhook, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *webhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state webhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	webhook, _, err := r.client.Webhooks.Get(ctx, state.ID.ValueInt64())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading webhook", err.Error())
		return
	}

	resp.Diagnostics.Append(flattenWebhook(ctx, webhook, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *webhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state webhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	upd := &mailtrap.UpdateWebhookRequest{URL: plan.URL.ValueString()}
	if !plan.Active.IsNull() && !plan.Active.IsUnknown() {
		upd.Active = mailtrap.Ptr(plan.Active.ValueBool())
	}
	if !plan.PayloadFormat.IsNull() && !plan.PayloadFormat.IsUnknown() {
		upd.PayloadFormat = plan.PayloadFormat.ValueString()
	}
	if !plan.EventTypes.IsNull() && !plan.EventTypes.IsUnknown() {
		resp.Diagnostics.Append(plan.EventTypes.ElementsAs(ctx, &upd.EventTypes, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	webhook, _, err := r.client.Webhooks.Update(ctx, state.ID.ValueInt64(), upd)
	if err != nil {
		resp.Diagnostics.AddError("Error updating webhook", err.Error())
		return
	}

	// The signing secret is only returned on creation; carry it over.
	plan.SigningSecret = state.SigningSecret
	resp.Diagnostics.Append(flattenWebhook(ctx, webhook, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *webhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state webhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, _, err := r.client.Webhooks.Delete(ctx, state.ID.ValueInt64()); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting webhook", err.Error())
	}
}

func (r *webhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "Expected a numeric webhook ID.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func flattenWebhook(ctx context.Context, w *mailtrap.Webhook, m *webhookModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(w.ID)
	m.URL = types.StringValue(w.URL)
	m.WebhookType = types.StringValue(w.WebhookType)
	m.Active = types.BoolValue(w.Active)
	m.PayloadFormat = types.StringValue(w.PayloadFormat)

	if w.SendingStream != "" {
		m.SendingStream = types.StringValue(w.SendingStream)
	} else {
		m.SendingStream = types.StringNull()
	}
	if w.DomainID != nil {
		m.DomainID = types.Int64Value(*w.DomainID)
	} else {
		m.DomainID = types.Int64Null()
	}
	if w.InboundInboxID != nil {
		m.InboundInboxID = types.Int64Value(*w.InboundInboxID)
	} else {
		m.InboundInboxID = types.Int64Null()
	}

	if w.EventTypes != nil {
		list, listDiags := types.ListValueFrom(ctx, types.StringType, w.EventTypes)
		diags.Append(listDiags...)
		m.EventTypes = list
	} else {
		m.EventTypes = types.ListNull(types.StringType)
	}

	// The API returns the signing secret only on creation; on every other call
	// keep the value already in state (null for imported webhooks).
	if w.SigningSecret != "" {
		m.SigningSecret = types.StringValue(w.SigningSecret)
	} else if m.SigningSecret.IsUnknown() {
		m.SigningSecret = types.StringNull()
	}
	return diags
}
