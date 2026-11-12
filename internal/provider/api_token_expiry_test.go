package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The expiry helpers are pure, so these run in plain `go test`, without the
// TF_ACC acceptance tests that CI skips.

func TestSameTokenExpiry(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"2027-06-01T00:00:00Z", "2027-06-01T00:00:00Z", true},
		{"2027-06-01T00:00:00Z", "2027-06-01T02:00:00+02:00", true},
		{"2027-06-01T00:00:00Z", "2027-06-01T00:00:00.000Z", true},
		{"never", "never", true},
		{"never", "2027-06-01T00:00:00Z", false},
		{"2027-06-01T00:00:00Z", "2028-01-01T00:00:00Z", false},
		{"2027-06-01T00:00:00Z", "next year", false},
	} {
		if got := sameTokenExpiry(tc.a, tc.b); got != tc.want {
			t.Errorf("sameTokenExpiry(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestFlattenTokenExpiry(t *testing.T) {
	for _, tc := range []struct {
		name  string
		prior types.String
		api   string
		want  string
	}{
		{"prior kept for the same instant", types.StringValue("2027-06-01T02:00:00+02:00"), "2027-06-01T00:00:00Z", "2027-06-01T02:00:00+02:00"},
		{"null prior: canonical UTC", types.StringNull(), "2027-06-01T02:00:00+02:00", "2027-06-01T00:00:00Z"},
		{"unknown prior: canonical UTC", types.StringUnknown(), "2027-06-01T00:00:00.000Z", "2027-06-01T00:00:00Z"},
		{"empty API value: never", types.StringNull(), "", tokenNeverExpires},
		{"never kept", types.StringValue(tokenNeverExpires), "", tokenNeverExpires},
		{"different instant: API value", types.StringValue("2027-06-01T00:00:00Z"), "2027-06-01T12:00:00Z", "2027-06-01T12:00:00Z"},
		{"expiry set where prior was never", types.StringValue(tokenNeverExpires), "2027-06-01T12:00:00Z", "2027-06-01T12:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := flattenTokenExpiry(tc.prior, tc.api); got.ValueString() != tc.want {
				t.Errorf("flattenTokenExpiry(%s, %q) = %q, want %q", tc.prior, tc.api, got.ValueString(), tc.want)
			}
		})
	}
}

func TestRFC3339Validator(t *testing.T) {
	for _, tc := range []struct {
		name    string
		value   types.String
		wantErr bool
	}{
		{"UTC", types.StringValue("2027-06-01T00:00:00Z"), false},
		{"offset", types.StringValue("2027-06-01T02:00:00+02:00"), false},
		{"zero fraction", types.StringValue("2027-06-01T00:00:00.000Z"), false},
		{"fraction", types.StringValue("2027-06-01T00:00:00.5Z"), true},
		{"not a timestamp", types.StringValue("next year"), true},
		{"date only", types.StringValue("2027-06-01"), true},
		{"null", types.StringNull(), false},
		{"unknown", types.StringUnknown(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := validator.StringRequest{Path: path.Root("expires_at"), ConfigValue: tc.value}
			resp := &validator.StringResponse{}
			rfc3339Validator{}.ValidateString(context.Background(), req, resp)
			if got := resp.Diagnostics.HasError(); got != tc.wantErr {
				t.Errorf("error = %v, want %v: %v", got, tc.wantErr, resp.Diagnostics)
			}
		})
	}
}

func TestRequiresReplaceIfExpiryChanges(t *testing.T) {
	for _, tc := range []struct {
		name          string
		config, state types.String
		want          bool
	}{
		{"unknown until apply", types.StringUnknown(), types.StringValue("2027-06-01T00:00:00Z"), true},
		{"removed from config", types.StringNull(), types.StringValue("2027-06-01T00:00:00Z"), false},
		{"no prior state", types.StringValue("2027-06-01T00:00:00Z"), types.StringNull(), false},
		{"same instant, other spelling", types.StringValue("2027-06-01T02:00:00+02:00"), types.StringValue("2027-06-01T00:00:00Z"), false},
		{"different instant", types.StringValue("2028-01-01T00:00:00Z"), types.StringValue("2027-06-01T00:00:00Z"), true},
		{"never to a date", types.StringValue("2027-06-01T00:00:00Z"), types.StringValue(tokenNeverExpires), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A configured value is planned as is; UseStateForUnknown has
			// already copied state into the plan when config is null.
			plan := tc.config
			if tc.config.IsNull() {
				plan = tc.state
			}
			req := planmodifier.StringRequest{ConfigValue: tc.config, StateValue: tc.state, PlanValue: plan}
			resp := &stringplanmodifier.RequiresReplaceIfFuncResponse{}
			requiresReplaceIfExpiryChanges(context.Background(), req, resp)
			if resp.RequiresReplace != tc.want {
				t.Errorf("RequiresReplace = %v, want %v", resp.RequiresReplace, tc.want)
			}
		})
	}
}
