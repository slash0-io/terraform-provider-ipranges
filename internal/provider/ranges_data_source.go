package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*rangesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*rangesDataSource)(nil)
)

func NewRangesDataSource() datasource.DataSource { return &rangesDataSource{} }

type rangesDataSource struct{ client *FeedClient }

type rangesModel struct {
	Service        types.String `tfsdk:"service"`
	Purpose        types.String `tfsdk:"purpose"`
	Name           types.String `tfsdk:"name"`
	Classification types.String `tfsdk:"classification"`
	Direction      types.String `tfsdk:"direction"`
	IPv4CIDRs      types.List   `tfsdk:"ipv4_cidrs"`
	IPv6CIDRs      types.List   `tfsdk:"ipv6_cidrs"`
	CIDRs          types.List   `tfsdk:"cidrs"`
	SyncToken      types.String `tfsdk:"sync_token"`
	GeneratedAt    types.String `tfsdk:"generated_at"`
}

// The Terraform name is <provider>_service while the Go identifiers here stay
// ranges*: internally "ranges" and "services" is a clearer pair to read than
// "service" and "services", which differ by one character at every call site.
func (d *rangesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service"
}

func (d *rangesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Current published IP ranges for one service purpose, e.g. Stripe's API endpoints " +
			"or GitHub's webhook sources. Ranges refresh on every terraform plan/apply.",
		Attributes: map[string]schema.Attribute{
			"service": schema.StringAttribute{
				Required:    true,
				Description: "Service slug, e.g. \"stripe\". Enumerate with the ipranges_services data source.",
			},
			"purpose": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Purpose key, e.g. \"api\" or \"webhooks\". May be omitted only when the " +
					"service publishes exactly one purpose.",
			},
			"name":           schema.StringAttribute{Computed: true, Description: "Human-readable service name."},
			"classification": schema.StringAttribute{Computed: true, Description: "dedicated | mixed | cdn-shared."},
			"direction": schema.StringAttribute{Computed: true,
				Description: "egress = ranges you connect to (SG egress rules); ingress = ranges the service connects from (SG ingress rules)."},
			"ipv4_cidrs":   schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Sorted IPv4 CIDRs."},
			"ipv6_cidrs":   schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Sorted IPv6 CIDRs."},
			"cidrs":        schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "ipv4_cidrs followed by ipv6_cidrs."},
			"sync_token":   schema.StringAttribute{Computed: true, Description: "Feed sync token at generation time."},
			"generated_at": schema.StringAttribute{Computed: true, Description: "Feed generation timestamp (RFC 3339)."},
		},
	}
}

func (d *rangesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*FeedClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *rangesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg rangesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	slug := cfg.Service.ValueString()
	svc, err := d.client.Service(ctx, slug)
	if err != nil {
		if _, notFound := err.(errNotFound); notFound {
			resp.Diagnostics.AddError(
				fmt.Sprintf("Unknown service %q", slug),
				d.suggestServices(ctx),
			)
			return
		}
		resp.Diagnostics.AddError("Reading feed failed", err.Error())
		return
	}

	keys := make([]string, 0, len(svc.Purposes))
	for k := range svc.Purposes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	purpose := cfg.Purpose.ValueString()
	if purpose == "" {
		if len(keys) != 1 {
			resp.Diagnostics.AddError(
				fmt.Sprintf("Service %q publishes %d purposes", slug, len(keys)),
				"Set purpose to one of: "+strings.Join(keys, ", "),
			)
			return
		}
		purpose = keys[0]
	}
	p, ok := svc.Purposes[purpose]
	if !ok {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Service %q has no purpose %q", slug, purpose),
			"Available purposes: "+strings.Join(keys, ", "),
		)
		return
	}

	state := rangesModel{
		Service:        types.StringValue(slug),
		Purpose:        types.StringValue(purpose),
		Name:           types.StringValue(svc.Name),
		Classification: types.StringValue(svc.Classification),
		Direction:      types.StringValue(p.Direction),
		SyncToken:      types.StringValue(svc.SyncToken),
		GeneratedAt:    types.StringValue(svc.GeneratedAt),
	}
	var diags = &resp.Diagnostics
	state.IPv4CIDRs = mustStringList(ctx, diags, p.IPv4)
	state.IPv6CIDRs = mustStringList(ctx, diags, p.IPv6)
	state.CIDRs = mustStringList(ctx, diags, append(append([]string{}, p.IPv4...), p.IPv6...))
	if diags.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// suggestServices best-effort lists the catalog for unknown-service errors.
func (d *rangesDataSource) suggestServices(ctx context.Context) string {
	idx, err := d.client.Index(ctx)
	if err != nil {
		return "Use the ipranges_services data source to list the catalog."
	}
	slugs := make([]string, 0, len(idx.Services))
	for _, s := range idx.Services {
		slugs = append(slugs, s.Slug)
	}
	return "Available services: " + strings.Join(slugs, ", ")
}
