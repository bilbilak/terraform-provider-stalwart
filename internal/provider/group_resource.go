package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &GroupResource{}

type GroupResource struct{ client *Client }

type GroupResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	Type                types.String `tfsdk:"type"`
	PrimaryEmail        types.String `tfsdk:"primary_email"`
	Aliases             types.List   `tfsdk:"aliases"`
	EnabledPermissions  types.List   `tfsdk:"enabled_permissions"`
	DisabledPermissions types.List   `tfsdk:"disabled_permissions"`
	ExternalMembers     types.List   `tfsdk:"external_members"`
}

func NewGroupResource() resource.Resource { return &GroupResource{} }

func (r *GroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *GroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	emptyList := listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{}))

	resp.Schema = schema.Schema{
		Description: "Manages a mail group (mailing list) in Stalwart Mail Server. " +
			"Users join the group via their own member_of field, not the other way around. " +
			"Mail sent to primary_email or any alias is delivered to all members.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Resource identifier. Always equal to name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Internal name for this group (e.g. info-domain1). Must be unique across all principals.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Human-readable description.",
			},
			"type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Principal type: 'list' (mailing list, default) or 'group' (IMAP shared folder group).",
				Default:     stringdefault.StaticString("list"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"primary_email": schema.StringAttribute{
				Required:    true,
				Description: "Primary address for this group (e.g. info@domain1.com). Stored as index 0 in Stalwart.",
			},
			"aliases": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "Additional addresses that also deliver to this group (e.g. [\"hello@domain1.com\"]).",
				Default:     emptyList,
			},
			"enabled_permissions": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "Permissions explicitly granted to this group (e.g. [\"email-send\", \"email-receive\"]).",
				Default:     listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
			},
			"disabled_permissions": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "Permissions explicitly denied for this group.",
				Default:     emptyList,
			},
			"external_members": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "External email addresses that also receive mail sent to this group.",
				Default:     emptyList,
			},
		},
	}
}

func (r *GroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *GroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan GroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.CreatePrincipal(ctx, &Principal{
		Type:                plan.Type.ValueString(),
		Name:                plan.Name.ValueString(),
		Description:         plan.Description.ValueString(),
		Emails:              combineEmails(plan.PrimaryEmail.ValueString(), listToStrings(ctx, plan.Aliases)),
		EnabledPermissions:  listToStrings(ctx, plan.EnabledPermissions),
		DisabledPermissions: listToStrings(ctx, plan.DisabledPermissions),
		ExternalMembers:     listToStrings(ctx, plan.ExternalMembers),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create group", err.Error())
		return
	}

	plan.ID = plan.Name
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *GroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state GroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	p, err := r.client.GetPrincipal(ctx, state.Name.ValueString())
	if err != nil {
		var nf ErrNotFound
		if errors.As(err, &nf) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read group", err.Error())
		return
	}

	primary, aliases := splitEmails(p.Emails)
	state.Name = types.StringValue(p.Name)
	state.Description = types.StringValue(p.Description)
	state.Type = types.StringValue(p.Type)
	state.PrimaryEmail = types.StringValue(primary)
	state.Aliases = stringsToList(aliases)
	state.EnabledPermissions = stringsToList(p.EnabledPermissions)
	state.DisabledPermissions = stringsToList(p.DisabledPermissions)
	state.ExternalMembers = stringsToList(p.ExternalMembers)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *GroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state GroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	oldEmails := combineEmails(state.PrimaryEmail.ValueString(), listToStrings(ctx, state.Aliases))
	newEmails := combineEmails(plan.PrimaryEmail.ValueString(), listToStrings(ctx, plan.Aliases))

	ops := []PatchOp{
		{Action: "set", Field: "description", Value: plan.Description.ValueString()},
	}
	ops = append(ops, orderedEmailOps(oldEmails, newEmails)...)
	ops = append(ops, diffStringSlice("enabledPermissions", listToStrings(ctx, state.EnabledPermissions), listToStrings(ctx, plan.EnabledPermissions))...)
	ops = append(ops, diffStringSlice("disabledPermissions", listToStrings(ctx, state.DisabledPermissions), listToStrings(ctx, plan.DisabledPermissions))...)
	ops = append(ops, diffStringSlice("externalMembers", listToStrings(ctx, state.ExternalMembers), listToStrings(ctx, plan.ExternalMembers))...)

	if err := r.client.UpdatePrincipal(ctx, state.Name.ValueString(), ops); err != nil {
		resp.Diagnostics.AddError("Failed to update group", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *GroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state GroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeletePrincipal(ctx, state.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete group", err.Error())
	}
}
