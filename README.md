# terraform-provider-egress

> **This provider has been renamed to [`slash0-io/ipranges`](https://registry.terraform.io/providers/slash0-io/ipranges/latest).** `slash0-io/egress` is frozen at this version and receives no further updates. The name changed because slightly over half the catalog's purposes are inbound ranges, and 27 services publish inbound ranges only, so a name describing outbound traffic described a minority of what the provider returns.

Migration is four find-and-replaces:

| Old | New |
|---|---|
| `slash0-io/egress` | `slash0-io/ipranges` |
| `egress_ranges` | `ipranges_service` |
| `egress_services` | `ipranges_services` |
| `EGRESS_FEED_URL` | `IPRANGES_FEED_URL` |

Attribute names, the feed, and the data it returns are unchanged.

Terraform data sources for the published IP ranges of third-party services (Stripe, GitHub, Datadog, Okta, Cloudflare, and 40+ more) so `0.0.0.0/0` never has to appear in your egress rules again.

```hcl
terraform {
  required_providers {
    egress = {
      source  = "slash0-io/egress"
      version = "~> 0.1"
    }
  }
}

data "egress_ranges" "stripe_api" {
  service = "stripe"
  purpose = "api"
}

resource "aws_security_group_rule" "stripe_egress" {
  type              = "egress"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = data.egress_ranges.stripe_api.ipv4_cidrs
  security_group_id = aws_security_group.app.id
  description       = "Stripe API"
}
```

## How it works

The provider reads a **versioned public feed** ([slash0-io/feed](https://github.com/slash0-io/feed)) rebuilt continuously from each vendor's *official* publication, never from third-party aggregators. Every service document carries a provenance chain: the upstream URL, retrieval timestamp, and the SHA-256 of the upstream body it was derived from.

The catalog records two things most sources don't model:

- **`direction`**: `egress` ranges are what your workloads connect *to* (SG egress rules); `ingress` ranges are what the service connects *from* (webhook/agent sources, for SG ingress rules). A third of published ranges are the latter, and putting them in egress rules is a silent no-op.
- **`classification`**: `dedicated` (vendor-owned space, safe to pin), `mixed`, or `cdn-shared` (pinning would allowlist an entire CDN). The feed also publishes a **non-publishers list**: services whose vendors state that IP pinning is unsupported, with their recommended alternative.

## Data sources

| Data source | Purpose |
|---|---|
| `egress_ranges` | Current ranges for one service purpose (`stripe`/`api`, `github`/`hooks`, `datadog`/`agents`, …) |
| `egress_services` | The full catalog: slugs, purposes, directions, classifications |

**→ [Browse the service catalog](https://github.com/slash0-io/feed/blob/main/CATALOG.md)** for every available slug and purpose, or visit the [feed's landing page](https://feed.slash0.io/).

Provider configuration: `feed_url` (optional) defaults to the `EGRESS_FEED_URL` environment variable, then the public feed. `file://` URLs are supported for air-gapped or vendored feeds.

## Security-group quota reality

Every CIDR in a rule consumes one security-group rule. The default quota is **60 per SG**, IPv4 and IPv6 counted separately, and `rules × SGs-per-ENI ≤ 1000` caps how far increases go. The [catalog](https://github.com/slash0-io/feed/blob/main/CATALOG.md), the [feed landing page](https://feed.slash0.io/), and `egress_services` (`ipv4_count`/`ipv6_count`) publish each purpose's entry count so you know the cost *before* wiring it in.

Ranges are **losslessly aggregated**: published coverage is preserved exactly, never widened. There is deliberately no supernet "summarization" option, because overshoot space inside shared cloud ranges is rentable by anyone with a credit card, which would turn an allowlist into an attack surface.

Patterns by scale:

- **≤ 60 entries** (most SaaS purposes): a single SG works by default.
- **60–1,000** (e.g. `stripe/api` at ~130): request a rules-per-SG quota increase, or split across up to 5 SGs on the same ENI: `chunklist(data.egress_ranges.stripe_api.ipv4_cidrs, 60)`.
- **1,000+** (`aws/all`, `azure/all`, `github/actions`): not security-group material at all. These purposes exist for AWS Network Firewall rule groups, route tables, proxies, and audit tooling. Prefer the service-specific purposes (`aws/s3`, `azure/storage`) where they fit.

## Staying current

Terraform data sources refresh only at `plan`/`apply` time. If you apply infrequently, pair the provider with scheduled applies, or stop managing the drift yourself: the hosted tier keeps AWS-native managed prefix lists continuously updated and shared into your account via AWS RAM, with staged rollouts and change notifications. It's in development with design partners: [**request early access**](https://github.com/slash0-io/terraform-provider-egress/issues/new?template=early-access.yml).

## Development

```sh
go build -o bin/terraform-provider-egress .
go test ./...
```

To run [examples/basic](examples/basic) against a local feed: build a feed with the [generator](https://github.com/slash0-io/feed), point a `dev_overrides` CLI config at `bin/`, and set `EGRESS_FEED_URL=file:///path/to/dist/v1`.

The feed schema in `internal/feedschema` is a vendored copy of the canonical definition in [slash0-io/feed](https://github.com/slash0-io/feed); schema v1 is frozen (additive changes only).

## License

[MPL-2.0](LICENSE)
