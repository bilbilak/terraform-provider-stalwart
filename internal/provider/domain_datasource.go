package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &DomainDataSource{}

type DomainDataSource struct{ client *Client }

type DomainDataSourceModel struct {
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

func NewDomainDataSource() datasource.DataSource { return &DomainDataSource{} }

func (d *DomainDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (d *DomainDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads an existing domain registered in Stalwart Mail Server.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Domain name to look up, e.g. example.com.",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Human-readable label for the domain.",
			},
		},
	}
}

func (d *DomainDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected data source configure type", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *DomainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg DomainDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	p, err := d.client.GetPrincipal(ctx, cfg.Name.ValueString())
	if err != nil {
		var nf ErrNotFound
		if errors.As(err, &nf) {
			resp.Diagnostics.AddError("Domain not found", fmt.Sprintf("No domain named %q exists in Stalwart.", cfg.Name.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Failed to read domain", err.Error())
		return
	}

	cfg.Name = types.StringValue(p.Name)
	cfg.Description = types.StringValue(p.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
