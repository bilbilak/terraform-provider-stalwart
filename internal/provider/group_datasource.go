package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &GroupDataSource{}

type GroupDataSource struct{ client *Client }

type GroupDataSourceModel struct {
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	Type                types.String `tfsdk:"type"`
	PrimaryEmail        types.String `tfsdk:"primary_email"`
	Aliases             types.List   `tfsdk:"aliases"`
	EnabledPermissions  types.List   `tfsdk:"enabled_permissions"`
	DisabledPermissions types.List   `tfsdk:"disabled_permissions"`
	ExternalMembers     types.List   `tfsdk:"external_members"`
}

func NewGroupDataSource() datasource.DataSource { return &GroupDataSource{} }

func (d *GroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (d *GroupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads an existing mail group from Stalwart Mail Server.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Internal group name to look up.",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Human-readable description.",
			},
			"type": schema.StringAttribute{
				Computed:    true,
				Description: "Principal type: list or group.",
			},
			"primary_email": schema.StringAttribute{
				Computed:    true,
				Description: "Primary delivery address (index 0 in Stalwart's emails array).",
			},
			"aliases": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Additional delivery addresses for this group.",
			},
			"enabled_permissions": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Permissions explicitly granted to this group.",
			},
			"disabled_permissions": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Permissions explicitly denied for this group.",
			},
			"external_members": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "External email addresses that also receive mail sent to this group.",
			},
		},
	}
}

func (d *GroupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *GroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg GroupDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	p, err := d.client.GetPrincipal(ctx, cfg.Name.ValueString())
	if err != nil {
		var nf ErrNotFound
		if errors.As(err, &nf) {
			resp.Diagnostics.AddError("Group not found", fmt.Sprintf("No group named %q exists in Stalwart.", cfg.Name.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Failed to read group", err.Error())
		return
	}

	primary, aliases := splitEmails(p.Emails)
	cfg.Name = types.StringValue(p.Name)
	cfg.Description = types.StringValue(p.Description)
	cfg.Type = types.StringValue(p.Type)
	cfg.PrimaryEmail = types.StringValue(primary)
	cfg.Aliases = stringsToList(aliases)
	cfg.EnabledPermissions = stringsToList(p.EnabledPermissions)
	cfg.DisabledPermissions = stringsToList(p.DisabledPermissions)
	cfg.ExternalMembers = stringsToList(p.ExternalMembers)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
