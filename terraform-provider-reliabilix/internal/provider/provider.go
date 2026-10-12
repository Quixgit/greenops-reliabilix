// Package provider is the Terraform provider: one provider block, three resources and one data source.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/quixgit/greenops-reliabilix/terraform-provider-reliabilix/internal/client"
)

type reliabilixProvider struct{ version string }

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &reliabilixProvider{version: version} }
}

type providerModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
}

func (p *reliabilixProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "reliabilix"
	resp.Version = p.version
}

func (p *reliabilixProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages Reliabilix projects, their data-residency and threshold policies, and budgets.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{Optional: true, Description: "Address of the API, like https://api.example.com. Defaults to the RELIABILIX_ENDPOINT environment variable."},
			"api_key":  schema.StringAttribute{Optional: true, Sensitive: true, Description: "An API key of the access type Terraform (role iac), from Settings > API keys. Defaults to the RELIABILIX_API_KEY environment variable."},
		},
	}
}

func (p *reliabilixProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	endpoint, key := os.Getenv("RELIABILIX_ENDPOINT"), os.Getenv("RELIABILIX_API_KEY")
	if !cfg.Endpoint.IsNull() && !cfg.Endpoint.IsUnknown() {
		endpoint = cfg.Endpoint.ValueString()
	}
	if !cfg.APIKey.IsNull() && !cfg.APIKey.IsUnknown() {
		key = cfg.APIKey.ValueString()
	}
	c, err := client.New(endpoint, key)
	if err != nil {
		resp.Diagnostics.AddError("The provider is not configured", err.Error()+". Set endpoint and api_key, or RELIABILIX_ENDPOINT and RELIABILIX_API_KEY.")
		return
	}
	resp.ResourceData, resp.DataSourceData = c, c
}

func (p *reliabilixProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{newProjectResource, newPolicyResource, newBudgetResource}
}

func (p *reliabilixProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{newProjectDataSource}
}

// clientFrom fetches the configured client in Configure of a resource or data source; nil before the provider is configured.
func clientFrom(data any, diags interface{ AddError(string, string) }) *client.Client {
	if data == nil {
		return nil
	}
	c, ok := data.(*client.Client)
	if !ok {
		diags.AddError("Unexpected provider data", "the provider passed something other than its client")
	}
	return c
}
