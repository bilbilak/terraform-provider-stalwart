package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &DNSRecordsDataSource{}

type DNSRecordsDataSource struct{ client *Client }

type DNSRecordsDataSourceModel struct {
	Domain  types.String `tfsdk:"domain"`
	Records types.List   `tfsdk:"records"`
}

func NewDNSRecordsDataSource() datasource.DataSource { return &DNSRecordsDataSource{} }

func (d *DNSRecordsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_records"
}

func (d *DNSRecordsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the DNS records Stalwart requires for a given domain (MX, SPF, DKIM, DMARC, etc.).",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				Required:    true,
				Description: "Domain to retrieve DNS records for.",
			},
			"records": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of DNS records to publish.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type":    schema.StringAttribute{Computed: true, Description: "DNS record type (MX, TXT, CNAME, etc.)."},
						"name":    schema.StringAttribute{Computed: true, Description: "Record name / host."},
						"content": schema.StringAttribute{Computed: true, Description: "Record value / content."},
					},
				},
			},
		},
	}
}

func (d *DNSRecordsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DNSRecordsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg DNSRecordsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	records, err := d.client.GetDNSRecords(ctx, cfg.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch DNS records", err.Error())
		return
	}

	vals := make([]attr.Value, len(records))
	for i, r := range records {
		vals[i] = types.ObjectValueMust(
			dnsRecordObjectType.AttrTypes,
			map[string]attr.Value{
				"type":    types.StringValue(r.Type),
				"name":    types.StringValue(r.Name),
				"content": types.StringValue(r.Content),
			},
		)
	}
	cfg.Records = types.ListValueMust(dnsRecordObjectType, vals)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
