// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/pools"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/provider/framework"
)

func TestAccHorizondbPoolParameterGroupAssociation_list(t *testing.T) {
	(HorizondbPoolParameterGroupAssociationResource{}).preCheck(t)
	if os.Getenv("ARM_TEST_HORIZONDB_POOL_ID_ALT") == "" {
		t.Skip("ARM_TEST_HORIZONDB_POOL_ID_ALT must identify a second dedicated PostgreSQL 18 pool in another cluster in ARM_TEST_LOCATION")
	}
	data := acceptance.BuildTestData(t, "azurerm_horizondb_pool_parameter_group_association", "test")
	ids := []string{os.Getenv("ARM_TEST_HORIZONDB_POOL_ID"), os.Getenv("ARM_TEST_HORIZONDB_POOL_ID_ALT")}
	steps := []resource.TestStep{{Config: fmt.Sprintf(`
%s
resource "azurerm_horizondb_parameter_group" "test" {
  count               = 2
  name                = "acctest-parameters-%d-${count.index}"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
  postgresql_version  = "18"
}
resource "azurerm_horizondb_pool_parameter_group_association" "test" {
  count                        = 2
  horizondb_pool_id            = [%q, %q][count.index]
  horizondb_parameter_group_id = azurerm_horizondb_parameter_group.test[count.index].id
  apply_method                 = "PendingReboot"
}
`, (HorizondbParameterGroupResource{}).template(data), data.RandomInteger, ids[0], ids[1])}}
	for _, rawId := range ids {
		id, err := pools.ParsePoolID(rawId)
		if err != nil {
			t.Fatal(err)
		}
		identity := map[string]knownvalue.Check{
			"name": knownvalue.StringExact(id.PoolName), "cluster_name": knownvalue.StringExact(id.ClusterName),
			"resource_group_name": knownvalue.StringExact(id.ResourceGroupName), "subscription_id": knownvalue.StringExact(id.SubscriptionId),
		}
		steps = append(steps, resource.TestStep{
			Query: true,
			Config: fmt.Sprintf(`
list "azurerm_horizondb_pool_parameter_group_association" "list" {
  provider         = azurerm
  include_resource = true
  config {
    horizondb_cluster_id = %q
  }
}
`, pools.NewClusterID(id.SubscriptionId, id.ResourceGroupName, id.ClusterName).ID()),
			QueryResultChecks: []querycheck.QueryResultCheck{
				querycheck.ExpectLength("azurerm_horizondb_pool_parameter_group_association.list", 1),
				querycheck.ExpectIdentity("azurerm_horizondb_pool_parameter_group_association.list", identity),
				querycheck.ExpectResourceKnownValues("azurerm_horizondb_pool_parameter_group_association.list", queryfilter.ByResourceIdentity(identity),
					[]querycheck.KnownValueCheck{{Path: tfjsonpath.New("apply_method"), KnownValue: knownvalue.StringExact("PendingReboot")}}),
			},
		})
	}
	resource.Test(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_14_0)},
		ProtoV5ProviderFactories: framework.ProtoV5ProviderFactoriesInit(context.Background(), "azurerm"),
		Steps:                    steps,
	})
}
