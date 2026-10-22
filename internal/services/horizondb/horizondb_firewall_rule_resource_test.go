package horizondb_test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/firewallrules"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance/check"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

type HorizondbFirewallRuleResource struct{}

func TestAccHorizondbFirewallRule_basic(t *testing.T) {
	resource := HorizondbFirewallRuleResource{}
	resource.preCheck(t)
	data := acceptance.BuildTestData(t, "azurerm_horizondb_firewall_rule", "test")
	data.ResourceTest(t, resource, []acceptance.TestStep{
		{Config: resource.basic(data), Check: check.That(data.ResourceName).ExistsInAzure(resource)},
		data.ImportStep(),
	})
}

func TestAccHorizondbFirewallRule_complete(t *testing.T) {
	resource := HorizondbFirewallRuleResource{}
	resource.preCheck(t)
	data := acceptance.BuildTestData(t, "azurerm_horizondb_firewall_rule", "test")
	data.ResourceTest(t, resource, []acceptance.TestStep{
		{Config: resource.complete(data), Check: check.That(data.ResourceName).ExistsInAzure(resource)},
		data.ImportStep(),
	})
}

func TestAccHorizondbFirewallRule_requiresImport(t *testing.T) {
	resource := HorizondbFirewallRuleResource{}
	resource.preCheck(t)
	data := acceptance.BuildTestData(t, "azurerm_horizondb_firewall_rule", "test")
	data.ResourceTest(t, resource, []acceptance.TestStep{
		{Config: resource.basic(data), Check: check.That(data.ResourceName).ExistsInAzure(resource)},
		data.RequiresImportErrorStep(resource.requiresImport),
	})
}

func TestAccHorizondbFirewallRule_update(t *testing.T) {
	resource := HorizondbFirewallRuleResource{}
	resource.preCheck(t)
	data := acceptance.BuildTestData(t, "azurerm_horizondb_firewall_rule", "test")
	data.ResourceTest(t, resource, []acceptance.TestStep{
		{Config: resource.basic(data), Check: check.That(data.ResourceName).ExistsInAzure(resource)},
		data.ImportStep(),
		{Config: resource.complete(data), Check: check.That(data.ResourceName).ExistsInAzure(resource)},
		data.ImportStep(),
		{Config: resource.basic(data), Check: check.That(data.ResourceName).ExistsInAzure(resource)},
		data.ImportStep(),
	})
}

func TestAccHorizondbFirewallRule_invalidRange(t *testing.T) {
	resource := HorizondbFirewallRuleResource{}
	resource.preCheck(t)
	data := acceptance.BuildTestData(t, "azurerm_horizondb_firewall_rule", "test")
	data.ResourceTest(t, resource, []acceptance.TestStep{
		{
			Config:      resource.config(data, "192.0.2.10", "192.0.2.1", ""),
			ExpectError: regexp.MustCompile("`end_ip_address` must be greater than or equal to `start_ip_address`"),
		},
	})
}

func (HorizondbFirewallRuleResource) preCheck(t *testing.T) {
	t.Helper()
	poolId := os.Getenv("ARM_TEST_HORIZONDB_POOL_ID")
	if poolId == "" {
		t.Skip("ARM_TEST_HORIZONDB_POOL_ID must identify an existing HorizonDB pool")
	}
	if _, err := firewallrules.ParsePoolID(poolId); err != nil {
		t.Fatal(err)
	}
}

func (HorizondbFirewallRuleResource) Exists(ctx context.Context, client *clients.Client, state *pluginsdk.InstanceState) (*bool, error) {
	id, err := firewallrules.ParseFirewallRuleID(state.ID)
	if err != nil {
		return nil, err
	}
	resp, err := client.HorizonDB.FirewallRulesClient.Get(ctx, *id)
	if err != nil {
		return nil, fmt.Errorf("retrieving %s: %+v", id, err)
	}
	return pointer.To(resp.Model != nil), nil
}

func (resource HorizondbFirewallRuleResource) basic(data acceptance.TestData) string {
	return resource.config(data, "192.0.2.1", "192.0.2.1", "")
}

func (resource HorizondbFirewallRuleResource) complete(data acceptance.TestData) string {
	return resource.config(data, "192.0.2.10", "192.0.2.20", "Acceptance test firewall rule")
}

func (HorizondbFirewallRuleResource) config(data acceptance.TestData, start, end, description string) string {
	return fmt.Sprintf(`
provider "azurerm" {
  features {}
}

resource "azurerm_horizondb_firewall_rule" "test" {
  name              = "acctest-rule-%d"
  horizondb_pool_id = %q
  start_ip_address  = %q
  end_ip_address    = %q
  description       = %q
}
`, data.RandomInteger, os.Getenv("ARM_TEST_HORIZONDB_POOL_ID"), start, end, description)
}

func (resource HorizondbFirewallRuleResource) requiresImport(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s

resource "azurerm_horizondb_firewall_rule" "import" {
  name              = azurerm_horizondb_firewall_rule.test.name
  horizondb_pool_id = azurerm_horizondb_firewall_rule.test.horizondb_pool_id
  start_ip_address  = azurerm_horizondb_firewall_rule.test.start_ip_address
  end_ip_address    = azurerm_horizondb_firewall_rule.test.end_ip_address
}
`, resource.basic(data))
}
