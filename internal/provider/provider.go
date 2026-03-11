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

var _ provider.Provider = &StalwartProvider{}

type StalwartProvider struct{ version string }

type StalwartProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Token    types.String `tfsdk:"token"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &StalwartProvider{version: version} }
}

func (p *StalwartProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "stalwart"
	resp.Version = p.version
}

func (p *StalwartProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages Stalwart Mail Server resources via its management API.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Required:    true,
				Description: "Stalwart management API base URL, e.g. https://mail.example.com.",
			},
			"token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Bearer token for API auth. Overrides username/password. Falls back to STALWART_TOKEN env var.",
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Description: "Admin username for HTTP Basic auth.",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Admin password for HTTP Basic auth.",
			},
		},
	}
}

func (p *StalwartProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg StalwartProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := cfg.Endpoint.ValueString()
	var client *Client

	switch {
	case !cfg.Token.IsNull() && cfg.Token.ValueString() != "":
		client = NewClientBearer(endpoint, cfg.Token.ValueString())
	case !cfg.Username.IsNull() && !cfg.Password.IsNull():
		client = NewClientBasic(endpoint, cfg.Username.ValueString(), cfg.Password.ValueString())
	default:
		if token := os.Getenv("STALWART_TOKEN"); token != "" {
			client = NewClientBearer(endpoint, token)
		} else {
			resp.Diagnostics.AddError(
				"Missing authentication",
				"Provide either 'token' or 'username'+'password', or set STALWART_TOKEN.",
			)
			return
		}
	}

	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *StalwartProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDomainResource,
		NewAccountResource,
		NewGroupResource,
		NewDKIMResource,
	}
}

func (p *StalwartProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDNSRecordsDataSource,
		NewDomainDataSource,
		NewAccountDataSource,
		NewGroupDataSource,
	}
}
