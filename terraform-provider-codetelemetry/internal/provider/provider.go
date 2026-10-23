package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type telemetryProvider struct{ version string }
type providerModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Token    types.String `tfsdk:"token"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &telemetryProvider{version} }
}
func (p *telemetryProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "codetelemetry"
	resp.Version = p.version
}
func (*telemetryProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Manage Code Telemetry workspace configuration through its authenticated API.", Attributes: map[string]schema.Attribute{
		"endpoint": schema.StringAttribute{Optional: true, Description: "Service origin. Falls back to CODETELEMETRY_ENDPOINT. HTTPS required except on loopback."},
		"token":    schema.StringAttribute{Optional: true, Sensitive: true, Description: "Cognito ID token, or CODETELEMETRY_TOKEN. Ingest keys are not accepted."},
	}}
}
func (*telemetryProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.Endpoint.IsUnknown() || data.Token.IsUnknown() {
		resp.Diagnostics.AddError("Unknown provider configuration", "Endpoint and token must be known before configuring the provider.")
		return
	}
	endpoint, token := data.Endpoint.ValueString(), data.Token.ValueString()
	if data.Endpoint.IsNull() {
		endpoint = os.Getenv("CODETELEMETRY_ENDPOINT")
	}
	if data.Token.IsNull() {
		token = os.Getenv("CODETELEMETRY_TOKEN")
	}
	c, err := newClient(endpoint, token)
	if err != nil {
		resp.Diagnostics.AddError("Invalid provider configuration", err.Error())
		return
	}
	c.binding, err = workspaceBinding(c.endpoint, token)
	if err != nil {
		resp.Diagnostics.AddError("Invalid provider identity", err.Error())
		return
	}
	resp.ResourceData = c
}
func (*telemetryProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{newAlarmRulesResource, newEnrichmentResource}
}
func (*telemetryProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }
