# egress Provider

!> **This provider has been renamed to [`slash0-io/ipranges`](https://registry.terraform.io/providers/slash0-io/ipranges/latest).** `slash0-io/egress` is frozen at this version and receives no further updates. The name changed because slightly over half the catalog's purposes are inbound ranges, and 27 services publish inbound ranges only, so a name describing outbound traffic described a minority of what the provider returns.

Migration is four find-and-replaces:

| Old | New |
|---|---|
| `slash0-io/egress` | `slash0-io/ipranges` |
| `egress_ranges` | `ipranges_service` |
| `egress_services` | `ipranges_services` |
| `EGRESS_FEED_URL` | `IPRANGES_FEED_URL` |

Attribute names, the feed, and the data it returns are unchanged.

Data sources for the published IP ranges of third-party services (Stripe, GitHub, Datadog, Okta, …), backed by a versioned public feed built exclusively from each vendor's official publication.

**Available services:** browse the [service catalog](https://github.com/slash0-io/feed/blob/main/CATALOG.md) for every service slug, its purposes (`api`, `webhooks`, `agents`, …), traffic direction, and classification, or enumerate them in Terraform with the `egress_services` data source.

## Example Usage

```terraform
provider "egress" {}

data "egress_ranges" "stripe_api" {
  service = "stripe"
  purpose = "api"
}
```

## Schema

### Optional

- `feed_url` (String) Feed base URL (the directory containing `index.json`). Supports `http(s)://` and `file://`. Defaults to the `EGRESS_FEED_URL` environment variable, then the public feed.
