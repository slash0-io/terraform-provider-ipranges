# ipranges Provider

Data sources for the published IP ranges of third-party services (Stripe, GitHub, Datadog, Okta, …), backed by a versioned public feed built exclusively from each vendor's official publication.

**Available services:** browse the [service catalog](https://github.com/slash0-io/feed/blob/main/CATALOG.md) for every service slug, its purposes (`api`, `webhooks`, `agents`, …), traffic direction, and classification, or enumerate them in Terraform with the `ipranges_services` data source.

## Example Usage

```terraform
provider "ipranges" {}

data "ipranges_egress" "stripe_api" {
  service = "stripe"
  purpose = "api"
}
```

## Schema

### Optional

- `feed_url` (String) Feed base URL (the directory containing `index.json`). Supports `http(s)://` and `file://`. Defaults to the `IPRANGES_FEED_URL` environment variable, then the public feed.
