# ipranges_ingress (Data Source)

Ranges the service connects **in** from, such as webhook senders, monitoring probes and crawlers, for security group ingress rules on whatever receives that traffic. Ranges refresh on every `terraform plan`/`apply`.

Using a purpose the feed marks as `egress` throws an error. Purposes marked `both` are accepted here and by `ipranges_egress`.

Read `direction` from the catalog rather than inferring it from the purpose key. Purpose keys often inherit the vendor's vocabulary, which is written from their side, so a purpose called `outbound` is traffic leaving the vendor, which arrives at you, and belongs here. `anthropic/outbound`, `elastic-cloud/outbound` and `neon/outbound` are all `ingress`.

## Example Usage

```terraform
data "ipranges_ingress" "stripe_webhooks" {
  service = "stripe"
  purpose = "webhooks"
}

resource "aws_security_group_rule" "stripe_webhooks" {
  type              = "ingress"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = data.ipranges_ingress.stripe_webhooks.ipv4_cidrs
  security_group_id = aws_security_group.web.id
}
```

## Schema

### Required

- `service` (String) Service slug, e.g. `stripe`. See the [service catalog](https://github.com/slash0-io/feed/blob/main/CATALOG.md) for all available slugs, or enumerate with `ipranges_services`.

### Optional

- `purpose` (String) Purpose key, e.g. `webhooks`. Each service's purposes are listed in the [catalog](https://github.com/slash0-io/feed/blob/main/CATALOG.md). May be omitted when the service publishes exactly one ingress purpose, even if it publishes others in the opposite direction. Omitting it is fragile: a service gains purposes when a vendor starts publishing more detail, and the plan then fails with the available keys listed. Okta went from one purpose to sixteen this way. Naming the purpose keeps a configuration working across catalog changes, and gives a tighter allowlist than a service-wide set.

### Read-Only

- `ipv4_cidrs` (List of String) Sorted IPv4 CIDRs.
- `ipv6_cidrs` (List of String) Sorted IPv6 CIDRs.
- `cidrs` (List of String) `ipv4_cidrs` followed by `ipv6_cidrs`.
- `direction` (String) The feed's direction for this purpose: `ingress`, or `both` where the same ranges serve either way.
- `classification` (String) `dedicated` | `mixed` | `cdn-shared`.
- `name` (String) Human-readable service name.
- `sync_token` (String) Feed sync token at generation time.
- `generated_at` (String) Feed generation timestamp (RFC 3339).

~> Every CIDR consumes one security-group rule (default quota: 60 per SG, IPv4/IPv6 counted separately). Ranges are losslessly aggregated, so coverage is never widened. Check a purpose's entry counts in the [catalog](https://github.com/slash0-io/feed/blob/main/CATALOG.md) or via `ipranges_services` before wiring large purposes into SGs; 1,000+ entry purposes belong in firewall rule groups, not security groups.
