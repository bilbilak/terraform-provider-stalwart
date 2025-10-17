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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &GroupResource{}
var _ resource.ResourceWithImportState = &GroupResource{}

func NewGroupResource() resource.Resource {
	return &GroupResource{}
}

// GroupResource defines the resource implementation.
type GroupResource struct {
	client *StalwartClient
}

// GroupResourceModel describes the resource data model.
type GroupResourceModel struct {
	Id          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Email       types.String `tfsdk:"email"`
	Aliases     types.List   `tfsdk:"aliases"`
}

func (r *GroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *GroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Stalwart group resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Group principal ID",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Group name",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Group description",
				Required:            true,
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "Primary email address for the group",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"aliases": schema.ListAttribute{
				MarkdownDescription: "Email aliases for the group",
				Optional:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *GroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *GroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GroupResourceModel

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

	// Create group via API
	id, err := r.client.CreateGroupPrincipal(data.Name.ValueString(), data.Description.ValueString(), emails)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create group, got error: %s", err))
		return
	}

	data.Id = types.Int64Value(id)

	// Write logs using the tflog package
	tflog.Trace(ctx, "created a group resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GroupResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Get group from API
	principal, err := r.client.GetPrincipal(data.Id.ValueInt64())
	if err != nil {
		// If the resource doesn't exist, remove it from state
		if strings.Contains(err.Error(), "principal not found") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read group, got error: %s", err))
		return
	}

	// Update the model with the latest data
	if name, ok := principal["name"].(string); ok {
		data.Name = types.StringValue(name)
	}

	if description, ok := principal["description"].(string); ok {
		data.Description = types.StringValue(description)
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

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data GroupResourceModel

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

	// Update group via API
	err := r.client.UpdateGroupPrincipal(data.Id.ValueInt64(), data.Name.ValueString(), data.Description.ValueString(), emails)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update group, got error: %s", err))
		return
	}

	// Read back the updated principal to get actual state from API
	principal, err := r.client.GetPrincipal(data.Id.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read group after update, got error: %s", err))
		return
	}

	// Update model with actual data from API
	if name, ok := principal["name"].(string); ok {
		data.Name = types.StringValue(name)
	}
	if description, ok := principal["description"].(string); ok {
		data.Description = types.StringValue(description)
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

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GroupResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Delete group via API
	err := r.client.DeletePrincipal(data.Id.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete group, got error: %s", err))
		return
	}
}

func (r *GroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by ID
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse ID as integer: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

// deduplicateEmails removes duplicate emails while preserving order (keeps first occurrence)
func deduplicateEmails(emails []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(emails))

	for _, email := range emails {
		if !seen[email] {
			seen[email] = true
			result = append(result, email)
		}
	}

	return result
}
