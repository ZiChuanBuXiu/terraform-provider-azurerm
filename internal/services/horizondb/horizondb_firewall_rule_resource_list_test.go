package horizondb_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/firewallrules"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/provider/framework"
)

func TestAccHorizondbFirewallRule_listByPoolID(t *testing.T) {
	(HorizondbFirewallRuleResource{}).preCheck(t)
	data := acceptance.BuildTestData(t, "azurerm_horizondb_firewall_rule", "list")
	poolId, err := firewallrules.ParsePoolID(os.Getenv("ARM_TEST_HORIZONDB_POOL_ID"))
	if err != nil {
		t.Fatal(err)
	}
	checks := []querycheck.QueryResultCheck{
		querycheck.ExpectLengthAtLeast("azurerm_horizondb_firewall_rule.list", 3),
	}
	for index := range 3 {
		checks = append(checks, querycheck.ExpectIdentity("azurerm_horizondb_firewall_rule.list", map[string]knownvalue.Check{
			"name":                knownvalue.StringExact(fmt.Sprintf("acctest-rule-%d-%d", data.RandomInteger, index)),
			"subscription_id":     knownvalue.StringExact(poolId.SubscriptionId),
			"resource_group_name": knownvalue.StringExact(poolId.ResourceGroupName),
			"cluster_name":        knownvalue.StringExact(poolId.ClusterName),
			"pool_name":           knownvalue.StringExact(poolId.PoolName),
		}))
	}
	resource.Test(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_14_0)},
		ProtoV5ProviderFactories: framework.ProtoV5ProviderFactoriesInit(context.Background(), "azurerm"),
		Steps: []resource.TestStep{
			{Config: fmt.Sprintf(`
provider "azurerm" {
  features {}
}

resource "azurerm_horizondb_firewall_rule" "test" {
  count             = 3
  name              = "acctest-rule-%d-${count.index}"
  horizondb_pool_id = %q
  start_ip_address  = "192.0.2.${count.index}"
  end_ip_address    = "192.0.2.${count.index}"
}
`, data.RandomInteger, os.Getenv("ARM_TEST_HORIZONDB_POOL_ID"))},
			{
				Query: true,
				Config: fmt.Sprintf(`
list "azurerm_horizondb_firewall_rule" "list" {
  provider = azurerm
  config {
    horizondb_pool_id = %q
  }
}
`, os.Getenv("ARM_TEST_HORIZONDB_POOL_ID")),
				QueryResultChecks: checks,
			},
		},
	})
}
