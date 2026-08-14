# ipranges_services (Data Source)

The feed catalog: every service with published, pinnable IP ranges.

## Example Usage

```terraform
data "ipranges_services" "catalog" {}

output "catalog" {
  value = [for s in data.ipranges_services.catalog.services : "${s.slug} (${s.classification})"]
}
```

## Schema

### Read-Only

- `services` (List of Object)
  - `slug` (String)
  - `name` (String)
  - `category` (String)
  - `classification` (String) `dedicated` | `mixed` | `cdn-shared`
  - `purposes` (List of Object)
    - `key` (String)
    - `direction` (String) `egress` | `ingress` | `both`
    - `ipv4_count` (Number) IPv4 CIDR count, the SG-rule quota cost of this purpose
    - `ipv6_count` (Number) IPv6 CIDR count (IPv4/IPv6 SG quotas are separate)
- `sync_token` (String)
- `generated_at` (String)
