package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure StalwartProvider satisfies various provider interfaces.
var _ provider.Provider = &StalwartProvider{}

// StalwartProvider defines the provider implementation.
type StalwartProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// StalwartProviderModel describes the provider data model.
type StalwartProviderModel struct {
	ServerHostname types.String `tfsdk:"server_hostname"`
	ApiKey         types.String `tfsdk:"api_key"`
}

func (p *StalwartProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "stalwart"
	resp.Version = p.version
}

func (p *StalwartProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"server_hostname": schema.StringAttribute{
				MarkdownDescription: "Stalwart server hostname (e.g., mail.example.com)",
				Optional:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "API key for authentication with Stalwart server",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *StalwartProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data StalwartProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Configuration values are now available.
	// If practitioners pass configuration values, they take priority over environment variables.

	serverHostname := os.Getenv("STALWART_SERVER_HOSTNAME")
	apiKey := os.Getenv("STALWART_API_KEY")

	if !data.ServerHostname.IsNull() {
		serverHostname = data.ServerHostname.ValueString()
	}

	if !data.ApiKey.IsNull() {
		apiKey = data.ApiKey.ValueString()
	}

	// If any of the expected configurations are missing, return
	// errors with provider-specific guidance.

	if serverHostname == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("server_hostname"),
			"Missing Stalwart Server Hostname",
			"The provider cannot create the Stalwart API client as there is a missing or empty value for the Stalwart server hostname. "+
				"Set the server_hostname value in the configuration or use the STALWART_SERVER_HOSTNAME environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing Stalwart API Key",
			"The provider cannot create the Stalwart API client as there is a missing or empty value for the Stalwart API key. "+
				"Set the api_key value in the configuration or use the STALWART_API_KEY environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Create a new Stalwart client using the configuration values
	client := &StalwartClient{
		ServerHostname: serverHostname,
		ApiKey:         apiKey,
	}

	// Make the Stalwart client available during DataSource and Resource
	// type Configure methods.
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *StalwartProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDomainResource,
		NewGroupResource,
		NewAccountResource,
	}
}

func (p *StalwartProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &StalwartProvider{
			version: version,
		}
	}
}
