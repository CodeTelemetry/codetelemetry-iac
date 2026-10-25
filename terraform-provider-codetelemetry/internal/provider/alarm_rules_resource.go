package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type alarmRulesResource struct{ c *client }
type alarmRulesModel struct {
	ID       types.String         `tfsdk:"id"`
	Rules    jsontypes.Normalized `tfsdk:"rules_json"`
	Revision types.Int64          `tfsdk:"revision"`
	Binding  types.String         `tfsdk:"workspace_binding"`
}
type rulesResponse struct {
	Rules    json.RawMessage `json:"rules"`
	Revision *int64          `json:"rev"`
}

func newAlarmRulesResource() resource.Resource { return &alarmRulesResource{} }
func (*alarmRulesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alarm_rules"
}
func (*alarmRulesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Owns the entire authenticated workspace alarm rule list. Use exactly one resource per workspace. Destroy clears all rules.", Attributes: map[string]schema.Attribute{
		"id":                schema.StringAttribute{Computed: true},
		"workspace_binding": schema.StringAttribute{Computed: true, Description: "Fingerprint of service origin and stable workspace identity; prevents accidental credential or endpoint rebinding."},
		"rules_json":        schema.StringAttribute{Required: true, CustomType: jsontypes.NormalizedType{}, Sensitive: true, Description: "JSON array of complete AlertRule objects (maximum 60 rules / 300000 bytes). Include all defaulted fields; see examples. Sensitive because webhook URLs can contain secrets."},
		"revision":          schema.Int64Attribute{Computed: true, Description: "Server revision used to reject concurrent edits."},
	}}
}
func (r *alarmRulesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected Code Telemetry API client.")
		return
	}
	r.c = c
}
func (*alarmRulesResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data alarmRulesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || data.Rules.IsUnknown() || data.Rules.IsNull() {
		return
	}
	if err := validateRules(data.Rules.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("rules_json"), "Invalid alarm rules", err.Error())
	}
}

func validateRules(raw string) error {
	if len(raw) > 300000 {
		return fmt.Errorf("rules_json exceeds 300000 bytes")
	}
	var rules []map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &rules) != nil || rules == nil || len(rules) > 60 {
		return fmt.Errorf("rules_json must be an array of at most 60 rule objects")
	}
	// Require complete objects so API-default insertion never produces an inconsistent
	// Terraform result. API still owns TQL compilation, privacy, and delivery validation.
	required := []string{"id", "name", "enabled", "type", "params", "channels", "cooldownMinutes", "severity", "evaluateEveryMinutes", "notifyOnRecovery", "origin"}
	for _, rule := range rules {
		if err := validateRuleFields(rule); err != nil {
			return err
		}
		if rule == nil {
			return fmt.Errorf("each rule must be an object")
		}
		for _, key := range required {
			if value, ok := rule[key]; !ok || string(value) == "null" {
				return fmt.Errorf("each rule must explicitly include %s; see the complete example", key)
			}
		}
	}
	return nil
}

// The service strips unknown keys and trims selected strings. Reject those
// lossy inputs during planning, before they can mutate remote configuration.
func validateRuleFields(rule map[string]json.RawMessage) error {
	if err := knownFields(rule, "id name enabled type params channels cooldownMinutes severity evaluateEveryMinutes notifyOnRecovery origin tql scope"); err != nil {
		return err
	}
	if err := trimmedField(rule, "name"); err != nil {
		return err
	}
	for key, allowed := range map[string]string{
		"params": "windowHours thresholdUsd thresholdPct minCalls",
		"origin": "kind recommendationId version",
		"tql":    "query column comparator threshold minMatched",
	} {
		raw, ok := rule[key]
		if !ok {
			continue
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(raw, &object) != nil || object == nil {
			return fmt.Errorf("rule %s must be an object", key)
		}
		if err := knownFields(object, allowed); err != nil {
			return err
		}
		if key == "origin" {
			var kind string
			_ = json.Unmarshal(object["kind"], &kind)
			if kind == "custom" && len(object) != 1 {
				return fmt.Errorf("custom origin accepts only kind")
			}
		}
		if key == "tql" {
			for _, field := range []string{"query", "column"} {
				if err := trimmedField(object, field); err != nil {
					return err
				}
			}
		}
	}
	for key, allowed := range map[string]string{"channels": "type url to", "scope": "field values"} {
		raw, ok := rule[key]
		if !ok {
			continue
		}
		var objects []map[string]json.RawMessage
		if json.Unmarshal(raw, &objects) != nil || objects == nil {
			return fmt.Errorf("rule %s must be an array", key)
		}
		for _, object := range objects {
			if object == nil {
				return fmt.Errorf("rule %s entries must be objects", key)
			}
			if err := knownFields(object, allowed); err != nil {
				return err
			}
			if key == "channels" {
				var kind string
				_ = json.Unmarshal(object["type"], &kind)
				fields := map[string]string{"in_app": "type", "email": "type to", "webhook": "type url"}[kind]
				if fields != "" {
					if err := knownFields(object, fields); err != nil {
						return err
					}
				}
			} else {
				if err := trimmedField(object, "field"); err != nil {
					return err
				}
				var values []string
				if json.Unmarshal(object["values"], &values) != nil {
					return fmt.Errorf("scope values must be strings")
				}
				for _, value := range values {
					if strings.TrimFunc(value, func(r rune) bool { return unicode.IsSpace(r) || r == '\ufeff' }) != value {
						return fmt.Errorf("scope values must not have leading or trailing whitespace")
					}
				}
			}
		}
	}
	return nil
}
func knownFields(object map[string]json.RawMessage, allowed string) error {
	for key := range object {
		if !slices.Contains(strings.Fields(allowed), key) {
			return fmt.Errorf("rules contain an unsupported field; use the documented AlertRule schema")
		}
	}
	return nil
}
func trimmedField(object map[string]json.RawMessage, key string) error {
	raw, ok := object[key]
	if !ok {
		return nil
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return fmt.Errorf("%s must be a string", key)
	}
	if strings.TrimFunc(value, func(r rune) bool { return unicode.IsSpace(r) || r == '\ufeff' }) != value {
		return fmt.Errorf("%s must not have leading or trailing whitespace", key)
	}
	return nil
}

func (r *alarmRulesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior alarmRulesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.checkWorkspace(prior.Binding); err != nil {
		resp.Diagnostics.AddError("Workspace mismatch", err.Error())
		return
	}
	if prior.Binding.IsNull() && !prior.Rules.IsNull() {
		resp.Diagnostics.AddWarning("Legacy workspace identity", "This state has no saved workspace fingerprint. Its first successful refresh binds the current endpoint and token identity. Verify the original workspace credentials before applying changes.")
	}
	var remote rulesResponse
	if err := r.c.request(ctx, "GET", "/api/alert-rules", nil, &remote); err != nil {
		resp.Diagnostics.AddError("Cannot read alarm rules", err.Error())
		return
	}
	if err := validRulesResponse(remote); err != nil {
		resp.Diagnostics.AddError("Invalid alarm response", err.Error())
		return
	}
	data := alarmRulesModel{types.StringValue("workspace"), jsontypes.NewNormalizedValue(string(remote.Rules)), types.Int64Value(*remote.Revision), types.StringValue(r.c.binding)}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func validRulesResponse(remote rulesResponse) error {
	if remote.Revision == nil || *remote.Revision < 0 {
		return fmt.Errorf("response is missing a valid revision")
	}
	var rules []json.RawMessage
	if json.Unmarshal(remote.Rules, &rules) != nil || rules == nil || len(rules) > 60 || len(remote.Rules) > 300000 {
		return fmt.Errorf("invalid or oversized rules response")
	}
	return nil
}
func (r *alarmRulesResource) save(ctx context.Context, data *alarmRulesModel, revision int64) error {
	// Config validation can see unknown values. Enforce bounds and lossless
	// normalization again once apply resolves them, before marshaling or writing.
	if err := validateRules(data.Rules.ValueString()); err != nil {
		return err
	}
	var remote rulesResponse
	body := struct {
		Rules    json.RawMessage `json:"rules"`
		Revision int64           `json:"rev"`
	}{json.RawMessage(data.Rules.ValueString()), revision}
	if err := r.c.request(ctx, "PUT", "/api/alert-rules", body, &remote); err != nil {
		return err
	}
	if err := validRulesResponse(remote); err != nil {
		return err
	}
	data.Binding = types.StringValue(r.c.binding)
	data.ID = types.StringValue("workspace")
	data.Revision = types.Int64Value(*remote.Revision)
	data.Rules = jsontypes.NewNormalizedValue(string(remote.Rules))
	return nil
}
func (r *alarmRulesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data alarmRulesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := validateRules(data.Rules.ValueString()); err != nil {
		resp.Diagnostics.AddError("Invalid alarm rules", err.Error())
		return
	}
	var current rulesResponse
	if err := r.c.request(ctx, "GET", "/api/alert-rules", nil, &current); err != nil {
		resp.Diagnostics.AddError("Cannot inspect alarm rules", err.Error())
		return
	}
	if err := validRulesResponse(current); err != nil {
		resp.Diagnostics.AddError("Invalid alarm response", err.Error())
		return
	}
	var existing []json.RawMessage
	_ = json.Unmarshal(current.Rules, &existing)
	if len(existing) != 0 {
		resp.Diagnostics.AddError("Alarm rules already exist", "Import the workspace before replacing existing rules: terraform import codetelemetry_alarm_rules.main workspace")
		return
	}
	if err := r.save(ctx, &data, *current.Revision); err != nil {
		resp.Diagnostics.AddError("Cannot create alarm rules", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *alarmRulesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, prior alarmRulesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.checkWorkspace(prior.Binding); err != nil {
		resp.Diagnostics.AddError("Workspace mismatch", err.Error())
		return
	}
	if prior.Revision.IsNull() || prior.Revision.IsUnknown() {
		resp.Diagnostics.AddError("Missing alarm revision", "Refresh before applying.")
		return
	}
	if err := r.save(ctx, &data, prior.Revision.ValueInt64()); err != nil {
		resp.Diagnostics.AddError("Cannot update alarm rules", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *alarmRulesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data alarmRulesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.checkWorkspace(data.Binding); err != nil {
		resp.Diagnostics.AddError("Workspace mismatch", err.Error())
		return
	}
	if data.Revision.IsNull() || data.Revision.IsUnknown() {
		resp.Diagnostics.AddError("Missing alarm revision", "Refresh before destroying.")
		return
	}
	data.Rules = jsontypes.NewNormalizedValue("[]")
	if err := r.save(ctx, &data, data.Revision.ValueInt64()); err != nil {
		resp.Diagnostics.AddError("Cannot clear alarm rules", err.Error())
	}
}
func (*alarmRulesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != "workspace" {
		resp.Diagnostics.AddError("Invalid import ID", "Use workspace; the token selects the authenticated workspace.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// Import has no previous configured representation. Retain equivalent imported
// JSON and computed fields together, preventing formatting-only API writes.
func (r *alarmRulesResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var prior, planned alarmRulesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planned)...)
	if resp.Diagnostics.HasError() || prior.Rules.IsNull() || prior.Rules.IsUnknown() || planned.Rules.IsNull() || planned.Rules.IsUnknown() {
		return
	}
	if len(prior.Rules.ValueString()) > 300000 || len(planned.Rules.ValueString()) > 300000 {
		return
	}
	same, diags := prior.Rules.StringSemanticEquals(ctx, planned.Rules)
	resp.Diagnostics.Append(diags...)
	if same {
		resp.Diagnostics.Append(resp.Plan.Set(ctx, &prior)...)
	}
}
