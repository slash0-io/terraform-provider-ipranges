terraform {
  required_providers {
    ipranges = {
      source = "slash0-io/ipranges"
    }
  }
}

# feed_url is resolved from (in order): this block, IPRANGES_FEED_URL, the
# public feed default.
provider "ipranges" {}

# Stripe's API endpoints: the ranges your workloads connect out to.
data "ipranges_egress" "stripe_api" {
  service = "stripe"
  purpose = "api"
}

# GitHub webhook sources. Direction is "ingress", so these belong in ingress
# rules on whatever receives your webhooks, not in egress rules.
data "ipranges_ingress" "github_hooks" {
  service = "github"
  purpose = "hooks"
}

data "ipranges_services" "catalog" {}

output "stripe_api_ipv4" {
  value = data.ipranges_egress.stripe_api.ipv4_cidrs
}

output "stripe_direction" {
  value = data.ipranges_egress.stripe_api.direction
}

output "github_hooks_cidrs" {
  value = data.ipranges_ingress.github_hooks.cidrs
}

output "catalog_size" {
  value = length(data.ipranges_services.catalog.services)
}

# The real-world shape: an egress security group rule fed by the data source.
# (Commented out so this example plans without AWS credentials.)
#
# resource "aws_security_group_rule" "stripe_egress" {
#   type              = "egress"
#   from_port         = 443
#   to_port           = 443
#   protocol          = "tcp"
#   cidr_blocks       = data.ipranges_egress.stripe_api.ipv4_cidrs
#   security_group_id = aws_security_group.app.id
#   description       = "Stripe API (managed by the slash0 feed ${data.ipranges_egress.stripe_api.sync_token})"
# }
