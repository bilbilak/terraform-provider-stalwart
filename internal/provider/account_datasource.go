package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &AccountDataSource{}

type AccountDataSource struct{ client *Client }

// AccountDataSourceModel deliberately excludes password/secrets — those are
// write-only on the resource and are never returned by the Stalwart API.
type AccountDataSourceModel struct {
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Quota        types.Int64  `tfsdk:"quota"`
	PrimaryEmail types.String `tfsdk:"primary_email"`
	Aliases      types.List   `tfsdk:"aliases"`
	MemberOf     types.List   `tfsdk:"member_of"`
	Roles        types.List   `tfsdk:"roles"`
}

func NewAccountDataSource() datasource.DataSource { return &AccountDataSource{} }

func (d *AccountDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account"
}

func (d *AccountDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads an existing account from Stalwart Mail Server. " +
			"Credentials are never exposed — use the stalwart_account resource to manage passwords.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Account login name to look up.",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Display name or description for the account.",
			},
			"quota": schema.Int64Attribute{
				Computed:    true,
				Description: "Mailbox quota in bytes. 0 means unlimited.",
			},
			"primary_email": schema.StringAttribute{
				Computed:    true,
				Description: "Primary email address (index 0 in Stalwart's emails array).",
			},
			"aliases": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Additional email addresses for this account.",
			},
			"member_of": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Names of groups this account belongs to.",
			},
			"roles": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Roles assigned to this account.",
			},
		},
	}
}

func (d *AccountDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AccountDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg AccountDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	p, err := d.client.GetPrincipal(ctx, cfg.Name.ValueString())
	if err != nil {
		var nf ErrNotFound
		if errors.As(err, &nf) {
			resp.Diagnostics.AddError("Account not found", fmt.Sprintf("No account named %q exists in Stalwart.", cfg.Name.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Failed to read account", err.Error())
		return
	}

	primary, aliases := splitEmails(p.Emails)
	cfg.Name = types.StringValue(p.Name)
	cfg.Description = types.StringValue(p.Description)
	cfg.Quota = types.Int64Value(p.Quota)
	cfg.PrimaryEmail = types.StringValue(primary)
	cfg.Aliases = stringsToList(aliases)
	cfg.MemberOf = stringsToList(p.MemberOf)
	cfg.Roles = stringsToList(p.Roles)
	// Secrets/password intentionally omitted — never returned by the API.
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
