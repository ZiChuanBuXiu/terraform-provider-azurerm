// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package dataprotection_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/provider/framework"
)

func TestAccDataProtectionBackupInstanceKubernetesCluster_listByVaultID(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_data_protection_backup_instance_kubernetes_cluster", "test")
	r := DataProtectionBackupInstanceKubernetesClusterResource{}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0),
		},
		ProtoV5ProviderFactories: framework.ProtoV5ProviderFactoriesInit(context.Background(), "azurerm"),
		Steps: []resource.TestStep{
			{
				Config: r.listConfig(data),
			},
			{
				Query:  true,
				Config: r.listQuery(),
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectLength("azurerm_data_protection_backup_instance_kubernetes_cluster.list", 1),
				},
			},
		},
	})
}

func (r DataProtectionBackupInstanceKubernetesClusterResource) listConfig(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s

resource "azurerm_elastic_san" "list" {
  name                = "acctestlist%[2]d"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
  base_size_in_tib    = 1

  sku {
    name = "Premium_LRS"
  }
}

resource "azurerm_elastic_san_volume_group" "list" {
  name           = "acctestlistvg%[2]d"
  elastic_san_id = azurerm_elastic_san.list.id
}

resource "azurerm_elastic_san_volume" "list" {
  name            = "vol1"
  volume_group_id = azurerm_elastic_san_volume_group.list.id
  size_in_gib     = 1
}

resource "azurerm_data_protection_backup_policy_elastic_san_volume_group" "list" {
  name                            = "acctest-dbp-esan-list-%[3]d"
  vault_id                        = azurerm_data_protection_backup_vault.test.id
  backup_repeating_time_intervals = ["R/2024-02-08T13:00:00+00:00/P1D"]

  default_retention_rule {
    life_cycle {
      duration        = "P7D"
      data_store_type = "OperationalStore"
    }
  }
}

resource "azurerm_role_assignment" "list_snapshot_exporter" {
  scope                = azurerm_elastic_san.list.id
  role_definition_name = "Elastic SAN Snapshot Exporter"
  principal_id         = azurerm_data_protection_backup_vault.test.identity[0].principal_id
}

resource "azurerm_data_protection_backup_instance_elastic_san_volume_group" "non_target" {
  name                         = "acctest-dbi-esan-list-%[3]d"
  location                     = azurerm_resource_group.test.location
  vault_id                     = azurerm_data_protection_backup_vault.test.id
  backup_policy_id             = azurerm_data_protection_backup_policy_elastic_san_volume_group.list.id
  elastic_san_volume_group_id  = azurerm_elastic_san_volume_group.list.id
  snapshot_resource_group_name = azurerm_resource_group.snap.name
  volume_name                  = azurerm_elastic_san_volume.list.name

  depends_on = [
    azurerm_role_assignment.list_snapshot_exporter,
    azurerm_role_assignment.test_vault_msi_snapshot_contributor_on_snap_rg,
  ]
}
`, r.basic(data), data.RandomIntOfLength(8), data.RandomInteger)
}

func (DataProtectionBackupInstanceKubernetesClusterResource) listQuery() string {
	return `
list "azurerm_data_protection_backup_instance_kubernetes_cluster" "list" {
  provider = azurerm
  config {
    vault_id = azurerm_data_protection_backup_vault.test.id
  }
}
`
}
