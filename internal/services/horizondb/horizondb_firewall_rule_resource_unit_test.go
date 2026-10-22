package horizondb

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/firewallrules"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
)

func TestValidateHorizondbFirewallRange(t *testing.T) {
	tests := []struct {
		name  string
		start string
		end   string
		valid bool
	}{
		{name: "equal", start: "192.0.2.1", end: "192.0.2.1", valid: true},
		{name: "range", start: "192.0.2.9", end: "192.0.2.10", valid: true},
		{name: "allIPv4", start: "0.0.0.0", end: "255.255.255.255", valid: true},
		{name: "reversed", start: "192.0.2.10", end: "192.0.2.9"},
		{name: "invalidStart", start: "invalid", end: "192.0.2.1"},
		{name: "invalidEnd", start: "192.0.2.1", end: "invalid"},
		{name: "ipv6Start", start: "::1", end: "192.0.2.1"},
		{name: "ipv6End", start: "192.0.2.1", end: "::1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateHorizondbFirewallRange(test.start, test.end)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%t, got error %v", test.valid, err)
			}
		})
	}
}

func TestExpandHorizondbFirewallRuleClearsDescription(t *testing.T) {
	model := expandHorizondbFirewallRule(HorizondbFirewallRuleResourceModel{
		StartIpAddress: "192.0.2.1", EndIpAddress: "192.0.2.10",
	})
	payload, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Properties map[string]interface{} `json:"properties"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if value, exists := decoded.Properties["description"]; !exists || value != "" {
		t.Fatalf("description removal must send an explicit empty string: %s", payload)
	}
	if _, exists := decoded.Properties["provisioningState"]; exists {
		t.Fatal("request includes read-only provisioning state")
	}
}

func TestHorizondbFirewallRuleFlatten(t *testing.T) {
	resource := HorizondbFirewallRuleResource{}
	metadata := sdk.NewResourceMetaData(nil, resource)
	id := firewallrules.NewFirewallRuleID("00000000-0000-0000-0000-000000000000", "example-rg", "example-cluster", "example-pool", "example-rule")
	metadata.ResourceData.SetId(id.ID())
	model := firewallrules.FirewallRule{
		Properties: &firewallrules.FirewallRuleProperties{
			StartIPAddress: "192.0.2.1", EndIPAddress: "192.0.2.10", Description: pointer.To("example"),
		},
	}
	if err := resource.flatten(metadata, &id, &model); err != nil {
		t.Fatal(err)
	}
	poolId := firewallrules.NewPoolID(id.SubscriptionId, id.ResourceGroupName, id.ClusterName, id.PoolName).ID()
	for key, expected := range map[string]string{
		"name": "example-rule", "horizondb_pool_id": poolId,
		"start_ip_address": "192.0.2.1", "end_ip_address": "192.0.2.10", "description": "example",
	} {
		if actual := metadata.ResourceData.Get(key); actual != expected {
			t.Fatalf("%s: expected %q, got %v", key, expected, actual)
		}
	}
	model.Properties.Description = nil
	if err := resource.flatten(metadata, &id, &model); err != nil {
		t.Fatal(err)
	}
	if metadata.ResourceData.Get("description") != "" {
		t.Fatal("removed API description remained in state")
	}
	if err := resource.flatten(metadata, &id, nil); err == nil {
		t.Fatal("expected an error for an empty API response")
	}
}

func TestHorizondbSchemas(t *testing.T) {
	resource := sdk.WrappedResource(HorizondbFirewallRuleResource{})
	if err := resource.InternalValidate(nil, true); err != nil {
		t.Fatal(err)
	}
	if resource.Identity == nil {
		t.Fatal("resource identity is missing")
	}
	for _, source := range (Registration{}).DataSources() {
		wrapper := sdk.NewDataSourceWrapper(source)
		dataSource, err := wrapper.DataSource()
		if err != nil {
			t.Fatal(err)
		}
		if err := dataSource.InternalValidate(nil, false); err != nil {
			t.Fatalf("%s: %v", source.ResourceType(), err)
		}
	}
	_, errors := resource.Schema["name"].ValidateFunc(strings.Repeat("a", 129), "name")
	if len(errors) == 0 {
		t.Fatal("firewall name exceeding the API limit was accepted")
	}
}
