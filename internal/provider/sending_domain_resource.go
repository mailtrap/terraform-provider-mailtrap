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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	mailtrap "github.com/mailtrap/mailtrap-go"
)

var (
	_ resource.Resource                = &sendingDomainResource{}
	_ resource.ResourceWithConfigure   = &sendingDomainResource{}
	_ resource.ResourceWithImportState = &sendingDomainResource{}
)

// NewSendingDomainResource is the resource factory registered with the provider.
func NewSendingDomainResource() resource.Resource {
	return &sendingDomainResource{}
}

type sendingDomainResource struct {
	client *mailtrap.Client
}

type sendingDomainModel struct {
	ID                         types.Int64  `tfsdk:"id"`
	DomainName                 types.String `tfsdk:"domain_name"`
	OpenTrackingEnabled        types.Bool   `tfsdk:"open_tracking_enabled"`
	ClickTrackingEnabled       types.Bool   `tfsdk:"click_tracking_enabled"`
	AutoUnsubscribeLinkEnabled types.Bool   `tfsdk:"auto_unsubscribe_link_enabled"`
	ComplianceStatus           types.String `tfsdk:"compliance_status"`
	DNSVerified                types.Bool   `tfsdk:"dns_verified"`
	Demo                       types.Bool   `tfsdk:"demo"`
	DNSRecords                 types.List   `tfsdk:"dns_records"`
}

type dnsRecordModel struct {
	Key    types.String `tfsdk:"key"`
	Domain types.String `tfsdk:"domain"`
	Name   types.String `tfsdk:"name"`
	Status types.String `tfsdk:"status"`
	Type   types.String `tfsdk:"type"`
	Value  types.String `tfsdk:"value"`
}

var dnsRecordAttrTypes = map[string]attr.Type{
	"key":    types.StringType,
	"domain": types.StringType,
	"name":   types.StringType,
	"status": types.StringType,
	"type":   types.StringType,
	"value":  types.StringType,
}

func (r *sendingDomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sending_domain"
}

func (r *sendingDomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Mailtrap sending domain used for email authentication.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the sending domain.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"domain_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The domain name (e.g. `example.com`). Changing this forces replacement.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"open_tracking_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether open tracking is enabled.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"click_tracking_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether click tracking is enabled.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"auto_unsubscribe_link_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the automatic unsubscribe link is enabled.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
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

func (r *sendingDomainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *sendingDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sendingDomainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain, _, err := r.client.SendingDomains.Create(ctx, plan.DomainName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error creating sending domain", err.Error())
		return
	}

	// Create only accepts the domain name; apply any configured tracking
	// options with a follow-up update.
	if upd := trackingUpdate(plan); upd != nil {
		domain, _, err = r.client.SendingDomains.Update(ctx, domain.ID, upd)
		if err != nil {
			resp.Diagnostics.AddError("Error setting sending domain tracking options", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(flattenSendingDomain(ctx, domain, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sendingDomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sendingDomainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain, _, err := r.client.SendingDomains.Get(ctx, state.ID.ValueInt64())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading sending domain", err.Error())
		return
	}

	resp.Diagnostics.Append(flattenSendingDomain(ctx, domain, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sendingDomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state sendingDomainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	upd := trackingUpdate(plan)
	if upd == nil {
		upd = &mailtrap.UpdateDomainRequest{}
	}
	domain, _, err := r.client.SendingDomains.Update(ctx, state.ID.ValueInt64(), upd)
	if err != nil {
		resp.Diagnostics.AddError("Error updating sending domain", err.Error())
		return
	}

	resp.Diagnostics.Append(flattenSendingDomain(ctx, domain, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sendingDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sendingDomainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.SendingDomains.Delete(ctx, state.ID.ValueInt64()); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting sending domain", err.Error())
	}
}

func (r *sendingDomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "Expected a numeric sending domain ID.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

// trackingUpdate builds an update payload from the tracking options set in the
// plan, or returns nil when none are configured.
func trackingUpdate(m sendingDomainModel) *mailtrap.UpdateDomainRequest {
	if m.OpenTrackingEnabled.IsNull() && m.ClickTrackingEnabled.IsNull() && m.AutoUnsubscribeLinkEnabled.IsNull() {
		return nil
	}
	upd := &mailtrap.UpdateDomainRequest{}
	if !m.OpenTrackingEnabled.IsNull() {
		upd.OpenTrackingEnabled = mailtrap.Ptr(m.OpenTrackingEnabled.ValueBool())
	}
	if !m.ClickTrackingEnabled.IsNull() {
		upd.ClickTrackingEnabled = mailtrap.Ptr(m.ClickTrackingEnabled.ValueBool())
	}
	if !m.AutoUnsubscribeLinkEnabled.IsNull() {
		upd.AutoUnsubscribeLinkEnabled = mailtrap.Ptr(m.AutoUnsubscribeLinkEnabled.ValueBool())
	}
	return upd
}

func flattenSendingDomain(ctx context.Context, d *mailtrap.SendingDomain, m *sendingDomainModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(d.ID)
	m.DomainName = types.StringValue(d.DomainName)
	m.OpenTrackingEnabled = types.BoolValue(d.OpenTrackingEnabled)
	m.ClickTrackingEnabled = types.BoolValue(d.ClickTrackingEnabled)
	m.AutoUnsubscribeLinkEnabled = types.BoolValue(d.AutoUnsubscribeLinkEnabled)
	m.ComplianceStatus = types.StringValue(d.ComplianceStatus)
	m.DNSVerified = types.BoolValue(d.DNSVerified)
	m.Demo = types.BoolValue(d.Demo)

	records := make([]dnsRecordModel, len(d.DNSRecords))
	for i, rec := range d.DNSRecords {
		records[i] = dnsRecordModel{
			Key:    types.StringValue(rec.Key),
			Domain: types.StringValue(rec.Domain),
			Name:   types.StringValue(rec.Name),
			Status: types.StringValue(rec.Status),
			Type:   types.StringValue(rec.Type),
			Value:  types.StringValue(rec.Value),
		}
	}
	list, listDiags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dnsRecordAttrTypes}, records)
	diags.Append(listDiags...)
	m.DNSRecords = list
	return diags
}
