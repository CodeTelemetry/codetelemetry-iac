package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type graderConfigResource struct{ c *client }
type graderConfigModel struct {
	ID       types.String         `tfsdk:"id"`
	Checks   jsontypes.Normalized `tfsdk:"checks_json"`
	Revision types.String         `tfsdk:"revision"`
	Binding  types.String         `tfsdk:"workspace_binding"`
}
type graderDraft struct {
	Checks            json.RawMessage `json:"checks"`
	RequestedFeatures string          `json:"requestedFeatures"`
}
type graderResponse struct {
	Revision string       `json:"revision"`
	Draft    *graderDraft `json:"draft"`
}

const graderAPI = "/api/validation/policy"

var graderRevisionPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func newGraderConfigResource() resource.Resource { return &graderConfigResource{} }
func (*graderConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_grader_config"
}
func (*graderConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Manages listed check IDs in the workspace Grader draft, preserving unrelated checks and requestedFeatures. Never publishes a policy. Use one state owner per workspace; destroy removes only managed draft checks.", Attributes: map[string]schema.Attribute{
		"id":                schema.StringAttribute{Computed: true},
		"checks_json":       schema.StringAttribute{Required: true, CustomType: jsontypes.NormalizedType{}, Sensitive: true, Description: "JSON array of complete ValidationCheck objects: at most 30 checks / 48 KiB. Only these IDs are managed. The service validates matcher semantics. Protect state and plans."},
		"revision":          schema.StringAttribute{Computed: true, Description: "Opaque server revision of the complete draft; concurrent edits require refresh and a new plan."},
		"workspace_binding": schema.StringAttribute{Computed: true, Description: "Fingerprint of service origin and stable authenticated workspace identity."},
	}}
}
func (r *graderConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (*graderConfigResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data graderConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || data.Checks.IsNull() || data.Checks.IsUnknown() {
		return
	}
	if _, err := parseGraderChecks(data.Checks.ValueString(), true); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("checks_json"), "Invalid Grader checks", err.Error())
	}
}

// Check transport shape without implementing the private evaluator. The service
// remains authoritative for recursive matchers, scopes, and scoring semantics.
func parseGraderChecks(raw string, input bool) ([]map[string]json.RawMessage, error) {
	var checks []map[string]json.RawMessage
	limit := 60
	if input {
		limit = 30
	}
	if len(raw) > 48*1024 || json.Unmarshal([]byte(raw), &checks) != nil || checks == nil || len(checks) > limit {
		return nil, fmt.Errorf("checks_json must be an array of at most %d checks within 48 KiB", limit)
	}
	ids := map[string]bool{}
	for _, check := range checks {
		var id string
		if check == nil || json.Unmarshal(check["id"], &id) != nil || id == "" || len(id) > 128 || ids[id] {
			return nil, fmt.Errorf("checks require distinct nonempty IDs of at most 128 bytes")
		}
		ids[id] = true
		if !input {
			continue
		} // Preserve compatibility fields in unowned remote checks.
		if err := knownFields(check, "id name category description weight satisfactionMode minimumOccurrences aggregationMode enabled repositoryId environment beforePush matcherRule detectorRule applicabilityRule evidenceRequirement createdFromActivityId components linkage"); err != nil {
			return nil, fmt.Errorf("unsupported Grader check field")
		}
		for key, allowed := range map[string][]string{
			"category":         {"unit_test", "integration_test", "end_to_end_test", "coverage", "environment_validation", "security_validation", "static_analysis", "custom"},
			"satisfactionMode": {"observed", "successful", "minimum_occurrences", "composite"},
			"aggregationMode":  {"once", "best", "count"},
		} {
			var v string
			if json.Unmarshal(check[key], &v) != nil || !slices.Contains(allowed, v) {
				return nil, fmt.Errorf("invalid Grader %s", key)
			}
		}
		var name string
		if json.Unmarshal(check["name"], &name) != nil || strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > 120 {
			return nil, fmt.Errorf("checks require a name of at most 120 characters")
		}
		var enabled bool
		if string(check["enabled"]) == "null" || json.Unmarshal(check["enabled"], &enabled) != nil {
			return nil, fmt.Errorf("checks require explicit enabled")
		}
		for key, bounds := range map[string][2]float64{"weight": {0, 1000}, "minimumOccurrences": {1, 100}} {
			var v float64
			if string(check[key]) == "null" || json.Unmarshal(check[key], &v) != nil || v < bounds[0] || v > bounds[1] {
				return nil, fmt.Errorf("invalid Grader %s", key)
			}
		}
		var mode string
		_ = json.Unmarshal(check["satisfactionMode"], &mode)
		if mode == "composite" {
			var components []json.RawMessage
			if json.Unmarshal(check["components"], &components) != nil || len(components) < 1 || len(components) > 8 {
				return nil, fmt.Errorf("composite checks require 1–8 components")
			}
		}
	}
	return checks, nil
}
func graderCheckID(check map[string]json.RawMessage) string {
	var id string
	_ = json.Unmarshal(check["id"], &id)
	return id
}
func sameGraderCheck(a, b map[string]json.RawMessage) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	var av, bv any
	_ = json.Unmarshal(x, &av)
	_ = json.Unmarshal(y, &bv)
	return reflect.DeepEqual(av, bv)
}
func (r *graderConfigResource) read(ctx context.Context) (graderResponse, []map[string]json.RawMessage, error) {
	var remote graderResponse
	if r.c.ingest {
		all := make([]map[string]json.RawMessage, 0)
		offset := 0
		for pages := 0; pages < 6; pages++ {
			var page struct {
				Config struct {
					Revision string       `json:"revision"`
					Draft    *graderDraft `json:"draft"`
					Next     *int         `json:"nextOffset"`
					Total    int          `json:"totalChecks"`
				} `json:"config"`
			}
			if err := r.c.request(ctx, "POST", "/api/agent/grader", map[string]int{"offset": offset, "limit": 10}, &page); err != nil {
				return remote, nil, err
			}
			if page.Config.Draft == nil || (remote.Revision != "" && remote.Revision != page.Config.Revision) {
				return remote, nil, fmt.Errorf("Grader changed during read; refresh and plan again")
			}
			checks, err := parseGraderChecks(string(page.Config.Draft.Checks), false)
			if err != nil {
				return remote, nil, err
			}
			all = append(all, checks...)
			remote = graderResponse{Revision: page.Config.Revision, Draft: page.Config.Draft}
			if len(all) > 60 {
				return remote, nil, fmt.Errorf("Grader exceeds 60 checks")
			}
			if page.Config.Next == nil {
				if len(all) != page.Config.Total {
					return remote, nil, fmt.Errorf("incomplete Grader response")
				}
				break
			}
			if *page.Config.Next != offset+len(checks) || *page.Config.Next <= offset || pages == 5 {
				return remote, nil, fmt.Errorf("invalid Grader pagination")
			}
			offset = *page.Config.Next
		}
		raw, _ := json.Marshal(all)
		remote.Draft.Checks = raw
	} else if err := r.c.request(ctx, "GET", graderAPI, nil, &remote); err != nil {
		return remote, nil, err
	}
	if remote.Draft == nil || !graderRevisionPattern.MatchString(remote.Revision) {
		return remote, nil, fmt.Errorf("incomplete Grader response: draft and revision required")
	}
	checks, err := parseGraderChecks(string(remote.Draft.Checks), false)
	return remote, checks, err
}
func selectGraderChecks(remote, owned []map[string]json.RawMessage) []map[string]json.RawMessage {
	result := make([]map[string]json.RawMessage, 0, len(owned))
	// Configured order is stable even if the UI reorders unrelated checks.
	for _, own := range owned {
		for _, check := range remote {
			if graderCheckID(own) == graderCheckID(check) {
				result = append(result, check)
				break
			}
		}
	}
	return result
}
func (r *graderConfigResource) set(data *graderConfigModel, checks []map[string]json.RawMessage, revision string) {
	raw, _ := json.Marshal(checks)
	data.Checks = jsontypes.NewNormalizedValue(string(raw))
	data.ID = types.StringValue("workspace")
	data.Revision = types.StringValue(revision)
	data.Binding = types.StringValue(r.c.binding)
}
func (r *graderConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data graderConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.checkWorkspace(data.Binding); err != nil {
		resp.Diagnostics.AddError("Workspace mismatch", err.Error())
		return
	}
	remote, checks, err := r.read(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read Grader draft", err.Error())
		return
	}
	// Import has no owned IDs yet and deliberately adopts the entire visible draft.
	if !data.Checks.IsNull() {
		owned, err := parseGraderChecks(data.Checks.ValueString(), false)
		if err != nil {
			resp.Diagnostics.AddError("Invalid saved Grader checks", err.Error())
			return
		}
		checks = selectGraderChecks(checks, owned)
	}
	if len(checks) > 30 {
		resp.Diagnostics.AddError("Too many managed checks", "Select at most 30 checks before importing this draft.")
		return
	}
	r.set(&data, checks, remote.Revision)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *graderConfigResource) save(ctx context.Context, data *graderConfigModel, prior *graderConfigModel) error {
	if data.Checks.IsNull() || data.Checks.IsUnknown() {
		return fmt.Errorf("checks_json must be known before apply")
	}
	desired, err := parseGraderChecks(data.Checks.ValueString(), true)
	if err != nil {
		return err
	}
	var owned []map[string]json.RawMessage
	if prior != nil {
		if err := r.c.checkWorkspace(prior.Binding); err != nil {
			return err
		}
		if !graderRevisionPattern.MatchString(prior.Revision.ValueString()) {
			return fmt.Errorf("missing Grader revision; refresh and plan again")
		}
		owned, err = parseGraderChecks(prior.Checks.ValueString(), false)
		if err != nil {
			return err
		}
	}
	remote, current, err := r.read(ctx)
	if err != nil {
		return err
	}
	if prior != nil && prior.Revision.ValueString() != remote.Revision {
		return fmt.Errorf("Grader configuration changed; refresh and plan again")
	}
	wanted := map[string]map[string]json.RawMessage{}
	for _, check := range desired {
		wanted[graderCheckID(check)] = check
	}
	managed := map[string]bool{}
	for _, check := range owned {
		managed[graderCheckID(check)] = true
	}
	next := make([]map[string]json.RawMessage, 0, len(current)+len(desired))
	for _, check := range current {
		id := graderCheckID(check)
		if replacement, ok := wanted[id]; ok {
			// Initial adoption is safe only when the check already matches exactly.
			// Updates cannot silently adopt an ID that wasn't previously owned either.
			if !managed[id] && !sameGraderCheck(check, replacement) {
				return fmt.Errorf("Grader check ID already exists with different content; import and review before adoption")
			}
			next = append(next, replacement)
			delete(wanted, id)
		} else if !managed[id] {
			next = append(next, check)
		}
	}
	for _, check := range desired {
		if _, ok := wanted[graderCheckID(check)]; ok {
			next = append(next, check)
		}
	}
	raw, _ := json.Marshal(next)
	if _, err := parseGraderChecks(string(raw), false); err != nil {
		return err
	}
	currentJSON, _ := json.Marshal(current)
	nextJSON, _ := json.Marshal(next)
	var currentValue, nextValue any
	_ = json.Unmarshal(currentJSON, &currentValue)
	_ = json.Unmarshal(nextJSON, &nextValue)
	if !reflect.DeepEqual(currentValue, nextValue) {
		var saved graderResponse
		body := struct {
			Checks   json.RawMessage `json:"checks"`
			Revision string          `json:"expectedRevision"`
		}{raw, remote.Revision}
		if r.c.ingest {
			removes := []string{}
			desiredIDs := map[string]bool{}
			for _, check := range desired {
				desiredIDs[graderCheckID(check)] = true
			}
			for id := range managed {
				if !desiredIDs[id] {
					removes = append(removes, id)
				}
			}

			features := remote.Draft.RequestedFeatures
			if features == "" {
				features = "Manage the selected Terraform Grader checks."
			}
			if err := r.c.request(ctx, "POST", "/api/agent/grader/configure", map[string]any{"expectedRevision": remote.Revision, "features": features, "upsertChecks": desired, "removeCheckIds": removes}, &saved); err != nil {
				return err
			}
		} else {
			if err := r.c.request(ctx, "PUT", graderAPI+"/draft", body, &saved); err != nil {
				return err
			}
			if saved.Draft == nil {
				return fmt.Errorf("Grader save unconfirmed; refresh before retrying")
			}
		}
		// The PUT contract returns the draft but no revision. Read back once; never
		// retry a write. A concurrent edit must not be recorded as a successful apply.
		remote, current, err = r.read(ctx)
		if err != nil {
			return fmt.Errorf("Grader save unconfirmed; refresh before retrying")
		}
	} else {
		current = next
	}
	for _, check := range current {
		id := graderCheckID(check)
		if managed[id] {
			found := false
			for _, want := range desired {
				if graderCheckID(want) == id {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Grader changed during removal; refresh and plan again")
			}
		}
	}
	actual := selectGraderChecks(current, desired)
	if len(actual) != len(desired) {
		return fmt.Errorf("Grader changed during save; refresh and plan again")
	}
	for i := range desired {
		if !sameGraderCheck(actual[i], desired[i]) {
			return fmt.Errorf("Grader changed during save; refresh and plan again")
		}
	}
	r.set(data, actual, remote.Revision)
	return nil
}
func (r *graderConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data graderConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.save(ctx, &data, nil); err != nil {
		resp.Diagnostics.AddError("Cannot create Grader config", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *graderConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, prior graderConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.save(ctx, &data, &prior); err != nil {
		resp.Diagnostics.AddError("Cannot update Grader config", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *graderConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var prior graderConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data := graderConfigModel{Checks: jsontypes.NewNormalizedValue("[]")}
	if err := r.save(ctx, &data, &prior); err != nil {
		resp.Diagnostics.AddError("Cannot remove managed Grader checks", err.Error())
	}
}
func (r *graderConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != "workspace" {
		resp.Diagnostics.AddError("Invalid import ID", "Use workspace; the token selects the workspace.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	if r.c != nil {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_binding"), r.c.binding)...)
	}
}
