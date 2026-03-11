package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &AccountResource{}

type AccountResource struct{ client *Client }

type AccountResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Password     types.String `tfsdk:"password"`
	Quota        types.Int64  `tfsdk:"quota"`
	PrimaryEmail types.String `tfsdk:"primary_email"`
	Aliases      types.List   `tfsdk:"aliases"`
	MemberOf     types.List   `tfsdk:"member_of"`
	Roles        types.List   `tfsdk:"roles"`
}

func NewAccountResource() resource.Resource { return &AccountResource{} }

func (r *AccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account"
}

func (r *AccountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an individual account in Stalwart Mail Server.",
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
				Description: "Account login name (e.g. sam).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Display name or description for this account.",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Account password. Stalwart stores it hashed. Changing this value updates the password.",
			},
			"quota": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Mailbox quota in bytes. 0 means unlimited.",
				Default:     int64default.StaticInt64(0),
			},
			"primary_email": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Primary email address for this account (e.g. sam@domain1.com). Stored as index 0 in Stalwart.",
			},
			"aliases": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "Additional email addresses (aliases) for this account. Each is stored after the primary in Stalwart's emails array.",
				Default:     listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
			},
			"member_of": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "Names of groups (stalwart_group.name) this user belongs to. Groups must be created before the user. This is the recommended way to assign group membership in Stalwart.",
				Default:     listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
			},
			"roles": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "Roles assigned to this user (e.g. [\"user\"]).",
				Default: listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{
					types.StringValue("user"),
				})),
			},
		},
	}
}

func (r *AccountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AccountResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	p := &Principal{
		Type:        "individual",
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Quota:       plan.Quota.ValueInt64(),
		Emails:      combineEmails(plan.PrimaryEmail.ValueString(), listToStrings(ctx, plan.Aliases)),
		MemberOf:    listToStrings(ctx, plan.MemberOf),
		Roles:       listToStrings(ctx, plan.Roles),
	}
	if !plan.Password.IsNull() && plan.Password.ValueString() != "" {
		p.Secrets = []string{plan.Password.ValueString()}
	}

	_, err := r.client.CreatePrincipal(ctx, p)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create account", err.Error())
		return
	}

	plan.ID = plan.Name
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AccountResourceModel
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
		resp.Diagnostics.AddError("Failed to read account", err.Error())
		return
	}

	primary, aliases := splitEmails(p.Emails)
	state.Name = types.StringValue(p.Name)
	state.Description = types.StringValue(p.Description)
	state.Quota = types.Int64Value(p.Quota)
	state.PrimaryEmail = types.StringValue(primary)
	state.Aliases = stringsToList(aliases)
	state.MemberOf = stringsToList(p.MemberOf)
	state.Roles = stringsToList(p.Roles)
	// password is write-only; keep whatever is already in state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state AccountResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	oldEmails := combineEmails(state.PrimaryEmail.ValueString(), listToStrings(ctx, state.Aliases))
	newEmails := combineEmails(plan.PrimaryEmail.ValueString(), listToStrings(ctx, plan.Aliases))

	ops := []PatchOp{
		{Action: "set", Field: "description", Value: plan.Description.ValueString()},
		{Action: "set", Field: "quota", Value: plan.Quota.ValueInt64()},
	}
	if !plan.Password.IsNull() && plan.Password != state.Password {
		ops = append(ops, PatchOp{Action: "set", Field: "secrets", Value: []string{plan.Password.ValueString()}})
	}
	ops = append(ops, orderedEmailOps(oldEmails, newEmails)...)
	ops = append(ops, diffStringSlice("memberOf", listToStrings(ctx, state.MemberOf), listToStrings(ctx, plan.MemberOf))...)
	ops = append(ops, diffStringSlice("roles", listToStrings(ctx, state.Roles), listToStrings(ctx, plan.Roles))...)

	if err := r.client.UpdatePrincipal(ctx, state.Name.ValueString(), ops); err != nil {
		resp.Diagnostics.AddError("Failed to update account", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeletePrincipal(ctx, state.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete account", err.Error())
	}
}

// combineEmails builds the ordered emails slice Stalwart expects:
// primary at index 0, aliases following.
func combineEmails(primary string, aliases []string) []string {
	if primary == "" {
		return aliases
	}
	out := make([]string, 0, 1+len(aliases))
	out = append(out, primary)
	out = append(out, aliases...)
	return out
}

// splitEmails decomposes Stalwart's emails array back into primary + aliases.
func splitEmails(emails []string) (primary string, aliases []string) {
	if len(emails) == 0 {
		return "", nil
	}
	return emails[0], emails[1:]
}

// listToStrings converts a types.List of strings to []string.
func listToStrings(ctx context.Context, l types.List) []string {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var out []string
	l.ElementsAs(ctx, &out, false)
	return out
}

// stringsToList converts []string to a types.List of strings.
func stringsToList(ss []string) types.List {
	vals := make([]attr.Value, len(ss))
	for i, s := range ss {
		vals[i] = types.StringValue(s)
	}
	return types.ListValueMust(types.StringType, vals)
}
