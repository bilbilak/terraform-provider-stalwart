package provider

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DomainResource{}
var _ resource.ResourceWithImportState = &DomainResource{}

func NewDomainResource() resource.Resource {
	return &DomainResource{}
}

// DomainResource defines the resource implementation.
type DomainResource struct {
	client *StalwartClient
}

// DomainResourceModel describes the resource data model.
type DomainResourceModel struct {
	Id          types.Int64  `tfsdk:"id"`
	Domain      types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	StsPolicy   types.String `tfsdk:"sts_policy"`
	Dkim        types.Object `tfsdk:"dkim"`
}

// DkimKeyModel describes the DKIM key data model.
type DkimKeyModel struct {
	Selector  types.String `tfsdk:"selector"`
	PublicKey types.String `tfsdk:"public_key"`
}

func (r *DomainResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *DomainResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Stalwart domain resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Domain principal ID",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Domain name",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Domain description",
				Required:            true,
			},
			"sts_policy": schema.StringAttribute{
				MarkdownDescription: "MTA-STS policy ID extracted from DNS records",
				Computed:            true,
			},
			"dkim": schema.SingleNestedAttribute{
				MarkdownDescription: "DKIM key information",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"rsa": schema.SingleNestedAttribute{
						MarkdownDescription: "RSA DKIM key",
						Computed:            true,
						Attributes: map[string]schema.Attribute{
							"selector": schema.StringAttribute{
								MarkdownDescription: "DKIM selector",
								Computed:            true,
							},
							"public_key": schema.StringAttribute{
								MarkdownDescription: "RSA public key",
								Computed:            true,
							},
						},
					},
					"ed25519": schema.SingleNestedAttribute{
						MarkdownDescription: "Ed25519 DKIM key",
						Computed:            true,
						Attributes: map[string]schema.Attribute{
							"selector": schema.StringAttribute{
								MarkdownDescription: "DKIM selector",
								Computed:            true,
							},
							"public_key": schema.StringAttribute{
								MarkdownDescription: "Ed25519 public key",
								Computed:            true,
							},
						},
					},
				},
			},
		},
	}
}

func (r *DomainResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DomainResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create domain via API
	id, err := r.client.CreatePrincipal(data.Domain.ValueString(), data.Description.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create domain, got error: %s", err))
		return
	}

	data.Id = types.Int64Value(id)

	// Create DKIM keys for the domain
	err = r.client.CreateDKIMKey(data.Domain.ValueString(), "Rsa")
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create RSA DKIM key, got error: %s", err))
		return
	}

	err = r.client.CreateDKIMKey(data.Domain.ValueString(), "Ed25519")
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create Ed25519 DKIM key, got error: %s", err))
		return
	}

	// Fetch DNS records for the newly created domain
	err = r.fetchDNSRecords(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch DNS records, got error: %s", err))
		return
	}

	// Write logs using the tflog package
	tflog.Trace(ctx, "created a domain resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DomainResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Get domain from API
	principal, err := r.client.GetPrincipal(data.Id.ValueInt64())
	if err != nil {
		// If the resource doesn't exist, remove it from state
		if strings.Contains(err.Error(), "principal not found") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read domain, got error: %s", err))
		return
	}

	// Update the model with the latest data
	if name, ok := principal["name"].(string); ok {
		data.Domain = types.StringValue(name)
	}

	if description, ok := principal["description"].(string); ok {
		data.Description = types.StringValue(description)
	}

	// Fetch DNS records for the domain
	err = r.fetchDNSRecords(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch DNS records, got error: %s", err))
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DomainResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Update domain via API
	err := r.client.UpdatePrincipal(data.Id.ValueInt64(), data.Domain.ValueString(), data.Description.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update domain, got error: %s", err))
		return
	}

	// Fetch DNS records to refresh computed fields (DKIM keys, STS policy)
	err = r.fetchDNSRecords(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch DNS records, got error: %s", err))
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DomainResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Delete domain via API
	err := r.client.DeletePrincipal(data.Id.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete domain, got error: %s", err))
		return
	}
}

func (r *DomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by ID
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse ID as integer: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

// fetchDNSRecords fetches DNS records for a domain and populates the model
func (r *DomainResource) fetchDNSRecords(ctx context.Context, data *DomainResourceModel) error {
	records, err := r.client.GetDNSRecords(data.Domain.ValueString())
	if err != nil {
		return err
	}

	// Parse records
	stsPolicy := ""
	var dkimRsaSelector string
	var dkimRsaPublicKey string
	var dkimEd25519Selector string
	var dkimEd25519PublicKey string
	foundRsa := false
	foundEd25519 := false

	for _, record := range records {
		recordType, _ := record["type"].(string)
		name, _ := record["name"].(string)
		content, _ := record["content"].(string)

		if recordType == "TXT" {
			// Check for MTA-STS record
			if strings.Contains(name, "_mta-sts.") && strings.Contains(content, "v=STSv1") {
				stsPolicy = extractSTSID(content)
			}

			// Check for DKIM records
			if strings.Contains(name, "._domainkey.") && strings.Contains(content, "v=DKIM1") {
				selector := extractDKIMSelector(name, data.Domain.ValueString())
				publicKey := extractDKIMPublicKey(content)
				keyType := extractDKIMKeyType(content)

				if keyType == "rsa" {
					dkimRsaSelector = selector
					dkimRsaPublicKey = publicKey
					foundRsa = true
				} else if keyType == "ed25519" {
					dkimEd25519Selector = selector
					dkimEd25519PublicKey = publicKey
					foundEd25519 = true
				}
			}
		}
	}

	data.StsPolicy = types.StringValue(stsPolicy)

	// Define the nested DKIM key object type
	dkimKeyAttrTypes := map[string]attr.Type{
		"selector":   types.StringType,
		"public_key": types.StringType,
	}

	// Create RSA object
	var rsaObj attr.Value
	if foundRsa {
		rsaObj, _ = types.ObjectValue(
			dkimKeyAttrTypes,
			map[string]attr.Value{
				"selector":   types.StringValue(dkimRsaSelector),
				"public_key": types.StringValue(dkimRsaPublicKey),
			},
		)
	} else {
		rsaObj = types.ObjectNull(dkimKeyAttrTypes)
	}

	// Create Ed25519 object
	var ed25519Obj attr.Value
	if foundEd25519 {
		ed25519Obj, _ = types.ObjectValue(
			dkimKeyAttrTypes,
			map[string]attr.Value{
				"selector":   types.StringValue(dkimEd25519Selector),
				"public_key": types.StringValue(dkimEd25519PublicKey),
			},
		)
	} else {
		ed25519Obj = types.ObjectNull(dkimKeyAttrTypes)
	}

	// Create the parent DKIM object containing both keys
	dkimObj, diags := types.ObjectValue(
		map[string]attr.Type{
			"rsa":     types.ObjectType{AttrTypes: dkimKeyAttrTypes},
			"ed25519": types.ObjectType{AttrTypes: dkimKeyAttrTypes},
		},
		map[string]attr.Value{
			"rsa":     rsaObj,
			"ed25519": ed25519Obj,
		},
	)
	if diags.HasError() {
		return fmt.Errorf("failed to create dkim object")
	}

	data.Dkim = dkimObj

	return nil
}

// extractSTSID extracts the ID from MTA-STS TXT record content
// Example: "v=STSv1; id=8463389085187991628" -> "8463389085187991628"
func extractSTSID(content string) string {
	re := regexp.MustCompile(`id=([0-9]+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractDKIMSelector extracts the selector from DKIM record name
// Example: "202509e._domainkey.aoking.nl." -> "202509e"
func extractDKIMSelector(name, domain string) string {
	// Remove the domain and ._domainkey. suffix
	selector := strings.TrimSuffix(name, ".")
	selector = strings.TrimSuffix(selector, "."+domain)
	selector = strings.TrimSuffix(selector, "._domainkey")
	return selector
}

// extractDKIMPublicKey extracts the public key from DKIM record content
// Example: "v=DKIM1; k=rsa; h=sha256; p=MIIBIj..." -> "MIIBIj..."
func extractDKIMPublicKey(content string) string {
	re := regexp.MustCompile(`p=([A-Za-z0-9+/=]+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractDKIMKeyType extracts the key type from DKIM record content
// Example: "v=DKIM1; k=rsa; h=sha256; p=..." -> "rsa"
func extractDKIMKeyType(content string) string {
	re := regexp.MustCompile(`k=([a-z0-9]+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
