# ipranges_egress (Data Source)

Ranges your workloads connect **out** to, for security group egress rules. Ranges refresh on every `terraform plan`/`apply`.

Using a purpose the feed marks as `ingress` throws an error. Purposes marked `both` are accepted here and by `ipranges_ingress`.

Read `direction` from the catalog rather than inferring it from the purpose key. Purpose keys often inherit the vendor's vocabulary, which is written from their side. `sentry/ingest` is named for what Sentry does with the traffic, while the connection is your workload reaching out, so it belongs here.

## Example Usage

```terraform
data "ipranges_egress" "stripe_api" {
  service = "stripe"
  purpose = "api"
}

resource "aws_security_group_rule" "stripe_api" {
  type              = "egress"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = data.ipranges_egress.stripe_api.ipv4_cidrs
  security_group_id = aws_security_group.app.id
}
```

## Schema

### Required

- `service` (String) Service slug, e.g. `stripe`. See the [service catalog](https://github.com/slash0-io/feed/blob/main/CATALOG.md) for all available slugs, or enumerate with `ipranges_services`.

### Optional

- `purpose` (String) Purpose key, e.g. `api`. Each service's purposes are listed in the [catalog](https://github.com/slash0-io/feed/blob/main/CATALOG.md). May be omitted when the service publishes exactly one egress purpose, even if it publishes others in the opposite direction. Omitting it is fragile: a service gains purposes when a vendor starts publishing more detail, and the plan then fails with the available keys listed. Okta went from one purpose to sixteen this way. Naming the purpose keeps a configuration working across catalog changes, and gives a tighter allowlist than a service-wide set.

### Read-Only

- `ipv4_cidrs` (List of String) Sorted IPv4 CIDRs.
- `ipv6_cidrs` (List of String) Sorted IPv6 CIDRs.
- `cidrs` (List of String) `ipv4_cidrs` followed by `ipv6_cidrs`.
- `direction` (String) The feed's direction for this purpose: `egress`, or `both` where the same ranges serve either way.
- `classification` (String) `dedicated` | `mixed` | `cdn-shared`.
- `name` (String) Human-readable service name.
- `sync_token` (String) Feed sync token at generation time.
- `generated_at` (String) Feed generation timestamp (RFC 3339).

~> Every CIDR consumes one security-group rule (default quota: 60 per SG, IPv4/IPv6 counted separately). Ranges are losslessly aggregated, so coverage is never widened. Check a purpose's entry counts in the [catalog](https://github.com/slash0-io/feed/blob/main/CATALOG.md) or via `ipranges_services` before wiring large purposes into SGs; 1,000+ entry purposes belong in firewall rule groups, not security groups.
