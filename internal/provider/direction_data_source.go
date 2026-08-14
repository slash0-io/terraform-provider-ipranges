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
	_ datasource.DataSource              = (*directionDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*directionDataSource)(nil)
)

// There is one data source per direction, ipranges_egress and ipranges_ingress,
// rather than a single one carrying a direction attribute. Getting the
// direction wrong is otherwise silent: ranges a service connects FROM, dropped
// into a security group egress rule, produce a rule that matches nothing and an
// apply that succeeds. Naming the direction in the type means the mistake is
// visible while reading the config, and it lets Read reject a mismatch at plan
// time instead of shipping a rule that quietly does nothing.
//
// Terraform resolves the provider from the type-name prefix, so these cannot be
// bare "egress"/"ingress": Terraform would look for a provider by that name.
func NewEgressDataSource() datasource.DataSource {
	return &directionDataSource{direction: "egress"}
}

func NewIngressDataSource() datasource.DataSource {
	return &directionDataSource{direction: "ingress"}
}

type directionDataSource struct {
	client    *FeedClient
	direction string // "egress" or "ingress"
}

type directionModel struct {
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

// other names the data source a user reached for by mistake.
func (d *directionDataSource) other() string {
	if d.direction == "egress" {
		return "ingress"
	}
	return "egress"
}

func (d *directionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.direction
}

func (d *directionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var desc string
	if d.direction == "egress" {
		desc = "Ranges your workloads connect OUT to, for security group egress rules. " +
			"Accepts purposes the feed marks egress or both; asking for an ingress purpose is an error."
	} else {
		desc = "Ranges the service connects IN from, such as webhook senders and monitoring probes, " +
			"for security group ingress rules on whatever receives that traffic. " +
			"Accepts purposes the feed marks ingress or both; asking for an egress purpose is an error."
	}
	resp.Schema = schema.Schema{
		Description: desc,
		Attributes: map[string]schema.Attribute{
			"service": schema.StringAttribute{
				Required:    true,
				Description: "Service slug, e.g. \"stripe\". Enumerate with the ipranges_services data source.",
			},
			"purpose": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Purpose key, e.g. \"api\" or \"webhooks\". May be omitted when the service " +
					"publishes exactly one purpose in this direction.",
			},
			"name":           schema.StringAttribute{Computed: true, Description: "Human-readable service name."},
			"classification": schema.StringAttribute{Computed: true, Description: "dedicated | mixed | cdn-shared."},
			"direction": schema.StringAttribute{Computed: true,
				Description: "The feed's direction for this purpose: matches the data source, or \"both\"."},
			"ipv4_cidrs":   schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Sorted IPv4 CIDRs."},
			"ipv6_cidrs":   schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Sorted IPv6 CIDRs."},
			"cidrs":        schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "ipv4_cidrs followed by ipv6_cidrs."},
			"sync_token":   schema.StringAttribute{Computed: true, Description: "Feed sync token at generation time."},
			"generated_at": schema.StringAttribute{Computed: true, Description: "Feed generation timestamp (RFC 3339)."},
		},
	}
}

func (d *directionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *directionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg directionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	slug := cfg.Service.ValueString()
	svc, err := d.client.Service(ctx, slug)
	if err != nil {
		if _, notFound := err.(errNotFound); notFound {
			resp.Diagnostics.AddError(fmt.Sprintf("Unknown service %q", slug), d.suggestServices(ctx))
			return
		}
		resp.Diagnostics.AddError("Reading feed failed", err.Error())
		return
	}

	// Split the catalog's purposes by whether this data source can serve them.
	// "both" counts for either direction.
	var mine, theirs []string
	for k, p := range svc.Purposes {
		if p.Direction == d.direction || p.Direction == "both" {
			mine = append(mine, k)
		} else {
			theirs = append(theirs, k)
		}
	}
	sort.Strings(mine)
	sort.Strings(theirs)

	purpose := cfg.Purpose.ValueString()
	if purpose == "" {
		switch len(mine) {
		case 1:
			purpose = mine[0]
		case 0:
			resp.Diagnostics.AddError(
				fmt.Sprintf("Service %q publishes no %s ranges", slug, d.direction),
				fmt.Sprintf("Its purposes are %s, which are %s. Use the ipranges_%s data source instead.",
					strings.Join(theirs, ", "), d.other(), d.other()),
			)
			return
		default:
			resp.Diagnostics.AddError(
				fmt.Sprintf("Service %q publishes %d %s purposes", slug, len(mine), d.direction),
				"Set purpose to one of: "+strings.Join(mine, ", "),
			)
			return
		}
	}

	p, ok := svc.Purposes[purpose]
	if !ok {
		detail := fmt.Sprintf("Available %s purposes: %s", d.direction, strings.Join(mine, ", "))
		if len(mine) == 0 {
			detail = fmt.Sprintf("This service publishes no %s purposes.", d.direction)
		}
		resp.Diagnostics.AddError(fmt.Sprintf("Service %q has no purpose %q", slug, purpose), detail)
		return
	}
	if p.Direction != d.direction && p.Direction != "both" {
		// The whole reason these are separate types. Allowing it would produce
		// a rule that matches nothing, with a clean apply.
		resp.Diagnostics.AddError(
			fmt.Sprintf("%s/%s is an %s purpose, not %s", slug, purpose, p.Direction, d.direction),
			fmt.Sprintf("These are the addresses %s. Read them with the ipranges_%s data source "+
				"and put them in a security group %s rule. Used as %s, the rule would match no traffic.",
				d.explain(p.Direction), d.other(), d.other(), d.direction),
		)
		return
	}

	state := directionModel{
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

// explain puts the feed's direction into the reader's frame of reference,
// because vendor vocabulary runs the other way: a purpose a vendor calls
// "outbound" is traffic arriving at you.
func (d *directionDataSource) explain(direction string) string {
	if direction == "ingress" {
		return "the service connects to you from"
	}
	return "your workloads connect out to"
}

// suggestServices best-effort lists the catalog for unknown-service errors.
func (d *directionDataSource) suggestServices(ctx context.Context) string {
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
