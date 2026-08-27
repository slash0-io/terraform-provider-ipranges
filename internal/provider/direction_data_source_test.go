package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// readDirection drives a data source the way Terraform does: build a config
// with service and purpose set, everything else null, and call Read.
func readDirection(t *testing.T, d *directionDataSource, service, purpose string) (*datasource.ReadResponse, map[string]tftypes.Value) {
	t.Helper()
	ctx := context.Background()

	var sr datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &sr)
	objType := sr.Schema.Type().TerraformType(ctx).(tftypes.Object)

	vals := map[string]tftypes.Value{}
	for name, typ := range objType.AttributeTypes {
		vals[name] = tftypes.NewValue(typ, nil)
	}
	vals["service"] = tftypes.NewValue(tftypes.String, service)
	if purpose != "" {
		vals["purpose"] = tftypes.NewValue(tftypes.String, purpose)
	}

	resp := &datasource.ReadResponse{
		State: tfsdk.State{Schema: sr.Schema, Raw: tftypes.NewValue(objType, vals)},
	}
	d.Read(ctx, datasource.ReadRequest{
		Config: tfsdk.Config{Schema: sr.Schema, Raw: tftypes.NewValue(objType, vals)},
	}, resp)

	out := map[string]tftypes.Value{}
	if !resp.Diagnostics.HasError() {
		if err := resp.State.Raw.As(&out); err != nil {
			t.Fatalf("decode state: %v", err)
		}
	}
	return resp, out
}

func egressDS(t *testing.T) *directionDataSource {
	return &directionDataSource{client: testClient(t), direction: "egress"}
}

func ingressDS(t *testing.T) *directionDataSource {
	return &directionDataSource{client: testClient(t), direction: "ingress"}
}

func str(t *testing.T, v tftypes.Value) string {
	t.Helper()
	var s string
	if err := v.As(&s); err != nil {
		t.Fatalf("as string: %v", err)
	}
	return s
}

// The reason the data sources are split by direction at all. Ingress ranges in
// a security group egress rule produce a rule matching no traffic and a clean
// apply, so this has to fail at plan time.
func TestEgressRejectsAnIngressPurpose(t *testing.T) {
	resp, _ := readDirection(t, egressDS(t), "stripe", "webhooks")
	if !resp.Diagnostics.HasError() {
		t.Fatal("asking ipranges_egress for an ingress purpose must fail the plan")
	}
	d := resp.Diagnostics.Errors()[0]
	if !strings.Contains(d.Summary(), "ingress purpose") {
		t.Fatalf("summary should name the mismatch, got %q", d.Summary())
	}
	// The error is only useful if it names the data source that would work.
	if !strings.Contains(d.Detail(), "ipranges_ingress") {
		t.Fatalf("detail should point at the right data source, got %q", d.Detail())
	}
}

func TestIngressRejectsAnEgressPurpose(t *testing.T) {
	resp, _ := readDirection(t, ingressDS(t), "stripe", "api")
	if !resp.Diagnostics.HasError() {
		t.Fatal("asking ipranges_ingress for an egress purpose must fail the plan")
	}
	if d := resp.Diagnostics.Errors()[0]; !strings.Contains(d.Detail(), "ipranges_egress") {
		t.Fatalf("detail should point at the right data source, got %q", d.Detail())
	}
}

// Omitting purpose is only safe when there is exactly one candidate in this
// direction. Stripe has one ingress purpose and two egress ones.
func TestPurposeAutoSelectsWhenUnambiguous(t *testing.T) {
	resp, state := readDirection(t, ingressDS(t), "stripe", "")
	if resp.Diagnostics.HasError() {
		t.Fatalf("a single ingress purpose must auto-select: %v", resp.Diagnostics.Errors())
	}
	if got := str(t, state["purpose"]); got != "webhooks" {
		t.Fatalf("purpose = %q, want webhooks", got)
	}
}

// Two candidates must not be guessed between: picking one silently would be
// the same class of quiet mistake the split exists to prevent.
func TestPurposeAmbiguityIsAnError(t *testing.T) {
	resp, _ := readDirection(t, egressDS(t), "stripe", "")
	if !resp.Diagnostics.HasError() {
		t.Fatal("two egress purposes must not be silently disambiguated")
	}
	d := resp.Diagnostics.Errors()[0]
	for _, want := range []string{"api", "terminal"} {
		if !strings.Contains(d.Detail(), want) {
			t.Fatalf("detail should list the choices, got %q", d.Detail())
		}
	}
}

// A purpose the feed marks "both" is legitimate in either direction.
func TestBothDirectionPurposeServesEither(t *testing.T) {
	base := writeFeed(t, map[string]string{
		"index.json": indexJSON,
		"services/stripe.json": `{"schemaVersion":1,"slug":"stripe","name":"Stripe",
		 "classification":"dedicated","generatedAt":"2026-08-27T00:00:00Z","syncToken":"1787","sources":[],
		 "purposes":{"all":{"direction":"both","ipv4":["192.0.2.0/24"],"ipv6":[]}}}`,
	})
	for _, dir := range []string{"egress", "ingress"} {
		d := &directionDataSource{client: NewFeedClient(base, "test"), direction: dir}
		resp, state := readDirection(t, d, "stripe", "all")
		if resp.Diagnostics.HasError() {
			t.Fatalf("%s: a both-direction purpose must be accepted: %v", dir, resp.Diagnostics.Errors())
		}
		if got := str(t, state["direction"]); got != "both" {
			t.Fatalf("%s: direction = %q, want both", dir, got)
		}
	}
}

// cidrs is documented as ipv4 followed by ipv6, which practitioners splat
// straight into a rule.
func TestCIDRsConcatenatesFamiliesInOrder(t *testing.T) {
	resp, state := readDirection(t, ingressDS(t), "stripe", "webhooks")
	if resp.Diagnostics.HasError() {
		t.Fatalf("read: %v", resp.Diagnostics.Errors())
	}
	var all []tftypes.Value
	if err := state["cidrs"].As(&all); err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(all))
	for i, v := range all {
		got[i] = str(t, v)
	}
	want := []string{"198.51.100.0/24", "2001:db8::/32"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("cidrs = %v, want %v (ipv4 then ipv6)", got, want)
	}
}

// A service with no purposes in this direction should say so and name the
// other data source, not report a missing purpose.
func TestNoPurposesInThisDirectionExplainsWhy(t *testing.T) {
	base := writeFeed(t, map[string]string{
		"index.json": indexJSON,
		"services/stripe.json": `{"schemaVersion":1,"slug":"stripe","name":"Stripe",
		 "classification":"dedicated","generatedAt":"2026-08-27T00:00:00Z","syncToken":"1787","sources":[],
		 "purposes":{"webhooks":{"direction":"ingress","ipv4":["198.51.100.0/24"],"ipv6":[]}}}`,
	})
	d := &directionDataSource{client: NewFeedClient(base, "test"), direction: "egress"}
	resp, _ := readDirection(t, d, "stripe", "")
	if !resp.Diagnostics.HasError() {
		t.Fatal("a service with no egress purposes must error")
	}
	if det := resp.Diagnostics.Errors()[0].Detail(); !strings.Contains(det, "ipranges_ingress") {
		t.Fatalf("detail should redirect to the other data source, got %q", det)
	}
}

// The type name is what a practitioner writes in a config, and changing it
// silently breaks every module using it.
func TestTypeNamesAreStable(t *testing.T) {
	for _, tc := range []struct {
		ds   datasource.DataSource
		want string
	}{
		{NewEgressDataSource(), "ipranges_egress"},
		{NewIngressDataSource(), "ipranges_ingress"},
	} {
		var resp datasource.MetadataResponse
		tc.ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "ipranges"}, &resp)
		if resp.TypeName != tc.want {
			t.Fatalf("type name = %q, want %q", resp.TypeName, tc.want)
		}
	}
}
