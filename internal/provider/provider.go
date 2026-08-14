package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// DefaultFeedURL is the public feed. Overridable via the feed_url provider
// attribute or the IPRANGES_FEED_URL environment variable (attribute wins).
const DefaultFeedURL = "https://feed.slash0.io/v1"

var _ provider.Provider = (*iprangesProvider)(nil)

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &iprangesProvider{version: version} }
}

type iprangesProvider struct{ version string }

type providerModel struct {
	FeedURL types.String `tfsdk:"feed_url"`
}

func (p *iprangesProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "ipranges"
	resp.Version = p.version
}

func (p *iprangesProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Data sources for third-party service IP ranges (Stripe, GitHub, Datadog, ...), " +
			"backed by a versioned public feed built exclusively from each vendor's official publication.",
		Attributes: map[string]schema.Attribute{
			"feed_url": schema.StringAttribute{
				Optional: true,
				Description: "Feed base URL (the directory containing index.json). Supports http(s):// and file://. " +
					"Defaults to the IPRANGES_FEED_URL environment variable, then the public feed.",
			},
		},
	}
}

func (p *iprangesProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	feedURL := DefaultFeedURL
	if v := os.Getenv("IPRANGES_FEED_URL"); v != "" {
		feedURL = v
	}
	if !cfg.FeedURL.IsNull() && cfg.FeedURL.ValueString() != "" {
		feedURL = cfg.FeedURL.ValueString()
	}
	client := NewFeedClient(feedURL, "terraform-provider-ipranges/"+p.version)
	resp.DataSourceData = client
}

func (p *iprangesProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewRangesDataSource, NewServicesDataSource}
}

func (p *iprangesProvider) Resources(context.Context) []func() resource.Resource {
	return nil
}
