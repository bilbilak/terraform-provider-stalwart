package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AccountResource{}
var _ resource.ResourceWithImportState = &AccountResource{}

func NewAccountResource() resource.Resource {
	return &AccountResource{}
}

// AccountResource defines the resource implementation.
type AccountResource struct {
	client *StalwartClient
}

// AccountResourceModel describes the resource data model.
type AccountResourceModel struct {
	Id          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Email       types.String `tfsdk:"email"`
	Aliases     types.List   `tfsdk:"aliases"`
	Locale      types.String `tfsdk:"locale"`
	Groups      types.List   `tfsdk:"groups"`
	Roles       types.List   `tfsdk:"roles"`
	Secrets     types.List   `tfsdk:"secrets"`
}

func (r *AccountResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account"
}

func (r *AccountResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Stalwart account resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Account principal ID",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Account name",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Account description",
				Required:            true,
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "Primary email address for the account",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"aliases": schema.ListAttribute{
				MarkdownDescription: "Email aliases for the account",
				Optional:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"locale": schema.StringAttribute{
				MarkdownDescription: "Account locale (defaults to 'en')",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("en"),
			},
			"groups": schema.ListAttribute{
				MarkdownDescription: "Groups this account is a member of",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"roles": schema.ListAttribute{
				MarkdownDescription: "Roles assigned to this account (defaults to ['user'])",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"secrets": schema.ListAttribute{
				MarkdownDescription: "Secrets for the account (at least one password is required)",
				Required:            true,
				Sensitive:           true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *AccountResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*StalwartClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *StalwartClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *AccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AccountResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the emails array (email + aliases)
	emails := []string{data.Email.ValueString()}
	if !data.Aliases.IsNull() && !data.Aliases.IsUnknown() {
		var aliases []string
		resp.Diagnostics.Append(data.Aliases.ElementsAs(ctx, &aliases, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		emails = append(emails, aliases...)
	}

	// Deduplicate emails (keeping first occurrence)
	emails = deduplicateEmails(emails)

	// Get groups array
	var memberOf []string
	if !data.Groups.IsNull() && !data.Groups.IsUnknown() {
		resp.Diagnostics.Append(data.Groups.ElementsAs(ctx, &memberOf, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Get roles array (default to ["user"] if not provided)
	var roles []string
	if !data.Roles.IsNull() && !data.Roles.IsUnknown() {
		resp.Diagnostics.Append(data.Roles.ElementsAs(ctx, &roles, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	if len(roles) == 0 {
		roles = []string{"user"}
	}

	// Get secrets array (required)
	var secrets []string
	resp.Diagnostics.Append(data.Secrets.ElementsAs(ctx, &secrets, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate that at least one secret is provided
	if len(secrets) == 0 {
		resp.Diagnostics.AddError("Validation Error", "At least one secret (password) is required")
		return
	}

	// Create account via API
	id, err := r.client.CreateAccountPrincipal(
		data.Name.ValueString(),
		data.Description.ValueString(),
		emails,
		data.Locale.ValueString(),
		memberOf,
		roles,
		secrets,
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create account, got error: %s", err))
		return
	}

	data.Id = types.Int64Value(id)

	// Write logs using the tflog package
	tflog.Trace(ctx, "created an account resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AccountResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Get account from API
	principal, err := r.client.GetPrincipal(data.Id.ValueInt64())
	if err != nil {
		// If the resource doesn't exist, remove it from state
		if strings.Contains(err.Error(), "principal not found") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read account, got error: %s", err))
		return
	}

	// Update the model with the latest data
	if name, ok := principal["name"].(string); ok {
		data.Name = types.StringValue(name)
	}

	if description, ok := principal["description"].(string); ok {
		data.Description = types.StringValue(description)
	}

	if locale, ok := principal["locale"].(string); ok {
		data.Locale = types.StringValue(locale)
	}

	// Extract emails from the principal data
	// Note: API can return emails as either a string or array
	var emailsList []string
	switch v := principal["emails"].(type) {
	case string:
		// Single email as string
		emailsList = []string{v}
	case []interface{}:
		// Array of emails
		for _, e := range v {
			if emailStr, ok := e.(string); ok {
				emailsList = append(emailsList, emailStr)
			}
		}
	}

	if len(emailsList) > 0 {
		// First email is the primary email
		data.Email = types.StringValue(emailsList[0])

		// Remaining emails are aliases
		if len(emailsList) > 1 {
			aliasesList, diags := types.ListValueFrom(ctx, types.StringType, emailsList[1:])
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			data.Aliases = aliasesList
		} else {
			// Set empty aliases list if only one email
			emptyList, diags := types.ListValueFrom(ctx, types.StringType, []string{})
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			data.Aliases = emptyList
		}
	}

	// Extract groups (memberOf from API) - always set, even if empty
	memberOfSlice := []string{}
	if memberOf, ok := principal["memberOf"].([]interface{}); ok {
		for _, m := range memberOf {
			if memberStr, ok := m.(string); ok {
				memberOfSlice = append(memberOfSlice, memberStr)
			}
		}
	}
	groupsList, diags := types.ListValueFrom(ctx, types.StringType, memberOfSlice)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Groups = groupsList

	// Extract roles - always set, even if empty
	rolesSlice := []string{}
	if roles, ok := principal["roles"].([]interface{}); ok {
		for _, r := range roles {
			if roleStr, ok := r.(string); ok {
				rolesSlice = append(rolesSlice, roleStr)
			}
		}
	}
	rolesList, diags := types.ListValueFrom(ctx, types.StringType, rolesSlice)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Roles = rolesList

	// NOTE: Secrets are hashed by the API and returned as a single string (bcrypt hash)
	// We cannot read them back, so we keep the state value unchanged
	// This means secrets are write-only and won't trigger unnecessary updates

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AccountResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the emails array (email + aliases)
	emails := []string{data.Email.ValueString()}
	if !data.Aliases.IsNull() && !data.Aliases.IsUnknown() {
		var aliases []string
		resp.Diagnostics.Append(data.Aliases.ElementsAs(ctx, &aliases, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		emails = append(emails, aliases...)
	}

	// Deduplicate emails (keeping first occurrence)
	emails = deduplicateEmails(emails)

	// Get groups array
	var memberOf []string
	if !data.Groups.IsNull() && !data.Groups.IsUnknown() {
		resp.Diagnostics.Append(data.Groups.ElementsAs(ctx, &memberOf, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Get roles array (default to ["user"] if not provided)
	var roles []string
	if !data.Roles.IsNull() && !data.Roles.IsUnknown() {
		resp.Diagnostics.Append(data.Roles.ElementsAs(ctx, &roles, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	if len(roles) == 0 {
		roles = []string{"user"}
	}

	// Get secrets array (required)
	var secrets []string
	resp.Diagnostics.Append(data.Secrets.ElementsAs(ctx, &secrets, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate that at least one secret is provided
	if len(secrets) == 0 {
		resp.Diagnostics.AddError("Validation Error", "At least one secret (password) is required")
		return
	}

	// Update account via API
	err := r.client.UpdateAccountPrincipal(
		data.Id.ValueInt64(),
		data.Name.ValueString(),
		data.Description.ValueString(),
		emails,
		data.Locale.ValueString(),
		memberOf,
		roles,
		secrets,
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update account, got error: %s", err))
		return
	}

	// Read back the updated principal to get actual state from API
	principal, err := r.client.GetPrincipal(data.Id.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read account after update, got error: %s", err))
		return
	}

	// Update model with actual data from API
	if name, ok := principal["name"].(string); ok {
		data.Name = types.StringValue(name)
	}
	if description, ok := principal["description"].(string); ok {
		data.Description = types.StringValue(description)
	}
	if locale, ok := principal["locale"].(string); ok {
		data.Locale = types.StringValue(locale)
	}

	// Extract emails - API can return as string or array
	var emailsList []string
	switch v := principal["emails"].(type) {
	case string:
		emailsList = []string{v}
	case []interface{}:
		for _, e := range v {
			if emailStr, ok := e.(string); ok {
				emailsList = append(emailsList, emailStr)
			}
		}
	}

	if len(emailsList) > 0 {
		data.Email = types.StringValue(emailsList[0])
		if len(emailsList) > 1 {
			aliasesList, diags := types.ListValueFrom(ctx, types.StringType, emailsList[1:])
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			data.Aliases = aliasesList
		} else {
			emptyList, diags := types.ListValueFrom(ctx, types.StringType, []string{})
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			data.Aliases = emptyList
		}
	}

	// Extract groups - always set, even if empty
	memberOfSlice := []string{}
	if memberOfData, ok := principal["memberOf"].([]interface{}); ok {
		for _, m := range memberOfData {
			if memberStr, ok := m.(string); ok {
				memberOfSlice = append(memberOfSlice, memberStr)
			}
		}
	}
	groupsList, diags := types.ListValueFrom(ctx, types.StringType, memberOfSlice)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Groups = groupsList

	// Extract roles - always set, even if empty
	rolesSlice := []string{}
	if rolesData, ok := principal["roles"].([]interface{}); ok {
		for _, r := range rolesData {
			if roleStr, ok := r.(string); ok {
				rolesSlice = append(rolesSlice, roleStr)
			}
		}
	}
	rolesList, diags := types.ListValueFrom(ctx, types.StringType, rolesSlice)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Roles = rolesList

	// NOTE: Secrets are hashed and cannot be read back - keep state value unchanged

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AccountResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Delete account via API
	err := r.client.DeletePrincipal(data.Id.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete account, got error: %s", err))
		return
	}
}

func (r *AccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by ID
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse ID as integer: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
