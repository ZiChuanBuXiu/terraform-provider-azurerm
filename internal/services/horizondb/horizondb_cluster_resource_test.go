// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb_test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/clusters"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance/check"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

type HorizondbClusterResource struct{}

func TestAccHorizondbCluster_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{Config: r.basic(data), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
	})
}

func TestAccHorizondbCluster_complete(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{Config: r.complete(data), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
	})
}

func TestAccHorizondbCluster_requiresImport(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{Config: r.basic(data), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.RequiresImportErrorStep(r.requiresImport),
	})
}

func TestAccHorizondbCluster_update(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{Config: r.settings(data, 7, false), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
		{Config: r.complete(data), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
		{Config: r.basic(data), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
	})
}

func TestAccHorizondbCluster_authentication(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceSequentialTest(t, r, []acceptance.TestStep{
		{
			Config: r.withAdministratorPassword(data, "P@ssword-123!"),
			Check:  check.That(data.ResourceName).ExistsInAzure(r),
		},
		data.ImportStep("administrator_login", "administrator_password"),
		{
			Config: r.withAdministratorPassword(data, "P@ssword-456!"),
			Check:  check.That(data.ResourceName).ExistsInAzure(r),
		},
	})
}

func TestAccHorizondbCluster_geoRedundantBackup(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: fmt.Sprintf(`
%s
resource "azurerm_horizondb_cluster" "test" {
  name                         = "acctest-horizondb-%d"
  resource_group_name          = azurerm_resource_group.test.name
  location                     = azurerm_resource_group.test.location
  postgresql_version           = "18"
  administrator_login          = "acctestadmin"
  administrator_password       = "P@ssword-123!"
  geo_redundant_backup_enabled = true
}
`, r.template(data), data.RandomInteger),
			Check: check.That(data.ResourceName).ExistsInAzure(r),
		},
		data.ImportStep("administrator_login", "administrator_password"),
	})
}

func TestAccHorizondbCluster_restore(t *testing.T) {
	r := HorizondbClusterResource{}
	r.preCheck(t, "ARM_TEST_HORIZONDB_SOURCE_CLUSTER_ID", "ARM_TEST_HORIZONDB_RESTORE_TIME")
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	data.ResourceTest(t, r, []acceptance.TestStep{
		{Config: r.restore(data, ""), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
	})
}

func TestAccHorizondbCluster_restoreCrossRegion(t *testing.T) {
	r := HorizondbClusterResource{}
	r.preCheck(t, "ARM_TEST_HORIZONDB_SOURCE_CLUSTER_ID", "ARM_TEST_HORIZONDB_RESTORE_TIME", "ARM_TEST_HORIZONDB_SOURCE_LOCATION")
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	data.ResourceTest(t, r, []acceptance.TestStep{
		{Config: r.restore(data, os.Getenv("ARM_TEST_HORIZONDB_SOURCE_LOCATION")), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		// The API does not return the requested timestamp for cross-region restores.
		data.ImportStep("administrator_login", "administrator_password", "restore.0.point_in_time_utc"),
	})
}

func TestAccHorizondbCluster_customerManagedKey(t *testing.T) {
	r := HorizondbClusterResource{}
	r.preCheck(t, "ARM_TEST_HORIZONDB_KEY_URI", "ARM_TEST_HORIZONDB_IDENTITY_ID", "ARM_TEST_HORIZONDB_ROTATED_KEY_URI", "ARM_TEST_HORIZONDB_ROTATED_IDENTITY_ID")
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	data.ResourceTest(t, r, []acceptance.TestStep{
		{Config: r.customerManagedKey(data, false), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
		{Config: r.customerManagedKey(data, true), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
		{Config: r.customerManagedKey(data, false), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
	})
}

func TestAccHorizondbCluster_versionRequired(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{{
		Config: fmt.Sprintf(`
%s
resource "azurerm_horizondb_cluster" "test" {
  name                = "acctest-horizondb-%d"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
}
`, r.template(data), data.RandomInteger),
		ExpectError: regexp.MustCompile("`postgresql_version` is required"),
	}})
}

func TestAccHorizondbCluster_createModeValidation(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: fmt.Sprintf(`
%s
resource "azurerm_horizondb_cluster" "test" {
  name                = "acctest-horizondb-%d"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
  create_mode         = "PointInTimeRestore"
}
`, r.template(data), data.RandomInteger),
			ExpectError: regexp.MustCompile("`restore` is required when `create_mode` is `PointInTimeRestore`"),
		},
		{
			Config: fmt.Sprintf(`
%s
resource "azurerm_horizondb_cluster" "test" {
  name                = "acctest-horizondb-%d"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
  postgresql_version  = "18"

  restore {
    source_horizondb_cluster_id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/example-rg/providers/Microsoft.HorizonDb/clusters/source"
    point_in_time_utc           = "2026-10-01T00:00:00Z"
  }
}
`, r.template(data), data.RandomInteger),
			ExpectError: regexp.MustCompile("`restore` can only be specified when `create_mode` is `PointInTimeRestore`"),
		},
	})
}

func TestAccHorizondbCluster_pool(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceSequentialTest(t, r, []acceptance.TestStep{
		{Config: r.withPool(data, "primary", 1, "Disabled", 4), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
		{Config: r.withPool(data, "primary", 2, "ZoneRedundant", 8), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
		{Config: r.basic(data), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
		{Config: r.withPool(data, "replacement", 1, "Disabled", 4), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
	})
}

func TestAccHorizondbCluster_poolRenameRejected(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceSequentialTest(t, r, []acceptance.TestStep{
		{Config: r.withPool(data, "primary", 1, "Disabled", 4), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		{Config: r.withPool(data, "renamed", 1, "Disabled", 4), ExpectError: regexp.MustCompile("changing `pool.name` in place is not supported")},
	})
}

func TestAccHorizondbCluster_poolValidation(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config:      r.withPool(data, "primary", 1, "ZoneRedundant", 4),
			ExpectError: regexp.MustCompile("`pool.high_availability_mode` must be `Disabled`"),
		},
		{
			Config:      r.withPool(data, fmt.Sprintf("acctest-horizondb-%d", data.RandomInteger), 1, "Disabled", 4),
			ExpectError: regexp.MustCompile("`pool.name` must differ"),
		},
	})
}

func (HorizondbClusterResource) preCheck(t *testing.T, variables ...string) {
	t.Helper()
	for _, variable := range variables {
		if os.Getenv(variable) == "" {
			t.Skipf("%s must be specified for this HorizonDB scenario", variable)
		}
	}
}

func (HorizondbClusterResource) Exists(ctx context.Context, client *clients.Client, state *pluginsdk.InstanceState) (*bool, error) {
	id, err := clusters.ParseClusterID(state.ID)
	if err != nil {
		return nil, err
	}
	resp, err := client.HorizonDB.ClustersClient.Get(ctx, *id)
	if err != nil {
		return nil, fmt.Errorf("retrieving %s: %+v", id, err)
	}
	return pointer.To(resp.Model != nil), nil
}

func (HorizondbClusterResource) template(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurerm" {
  features {}
}

resource "azurerm_resource_group" "test" {
  name     = "acctestRG-horizondb-%d"
  location = "%s"
}
`, data.RandomInteger, data.Locations.Primary)
}

func (r HorizondbClusterResource) basic(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s
resource "azurerm_horizondb_cluster" "test" {
  name                = "acctest-horizondb-%d"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
  postgresql_version  = "18"
  administrator_login = "acctestadmin"
  administrator_password = "P@ssword-123!"
}
`, r.template(data), data.RandomInteger)
}

func (r HorizondbClusterResource) withAdministratorPassword(data acceptance.TestData, password string) string {
	return fmt.Sprintf(`
%s
resource "azurerm_horizondb_cluster" "test" {
  name                   = "acctest-horizondb-%d"
  resource_group_name    = azurerm_resource_group.test.name
  location               = azurerm_resource_group.test.location
  postgresql_version     = "18"
  administrator_login    = "acctestadmin"
  administrator_password = %q
}
`, r.template(data), data.RandomInteger, password)
}

func (r HorizondbClusterResource) withPool(data acceptance.TestData, name string, nodes int, mode string, cores int) string {
	return fmt.Sprintf(`
%s
resource "azurerm_horizondb_cluster" "test" {
  name                = "acctest-horizondb-%d"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
  postgresql_version  = "18"
  administrator_login = "acctestadmin"
  administrator_password = "P@ssword-123!"

  pool {
    name                        = %q
    node_count                  = %d
    high_availability_mode      = %q

    compute {
      processor  = "AMD"
      generation = 1
      vcores     = %d
    }
  }

  tags = {
    cores = "%d"
  }
}
`, r.template(data), data.RandomInteger, name, nodes, mode, cores, cores)
}

func (r HorizondbClusterResource) complete(data acceptance.TestData) string {
	return r.settings(data, 14, true)
}

func (r HorizondbClusterResource) settings(data acceptance.TestData, retention int, publicNetworkAccessEnabled bool) string {
	return fmt.Sprintf(`
%s
resource "azurerm_horizondb_cluster" "test" {
  name                          = "acctest-horizondb-%d"
  resource_group_name           = azurerm_resource_group.test.name
  location                      = azurerm_resource_group.test.location
  postgresql_version            = "18"
  administrator_login           = "acctestadmin"
  administrator_password        = "P@ssword-123!"
  backup_retention_days         = %d
  public_network_access_enabled = %t
  tags = {
    environment = "acceptance"
    retention   = "%d"
  }
}
`, r.template(data), data.RandomInteger, retention, publicNetworkAccessEnabled, retention)
}

func (r HorizondbClusterResource) requiresImport(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s
resource "azurerm_horizondb_cluster" "import" {
  name                = azurerm_horizondb_cluster.test.name
  resource_group_name = azurerm_horizondb_cluster.test.resource_group_name
  location            = azurerm_horizondb_cluster.test.location
  postgresql_version  = azurerm_horizondb_cluster.test.postgresql_version
  administrator_login = "acctestadmin"
  administrator_password = "P@ssword-123!"
}
`, r.basic(data))
}

func (r HorizondbClusterResource) restore(data acceptance.TestData, sourceLocation string) string {
	sourceLocationConfig := ""
	if sourceLocation != "" {
		sourceLocationConfig = fmt.Sprintf("source_location = %q", sourceLocation)
	}
	return fmt.Sprintf(`
%s
resource "azurerm_horizondb_cluster" "test" {
  name                = "acctest-horizondb-%d"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
  create_mode         = "PointInTimeRestore"
  administrator_login = "acctestadmin"
  administrator_password = "P@ssword-123!"

  restore {
    source_horizondb_cluster_id = %q
    point_in_time_utc           = %q
    %s
  }
}
`, r.template(data), data.RandomInteger, os.Getenv("ARM_TEST_HORIZONDB_SOURCE_CLUSTER_ID"), os.Getenv("ARM_TEST_HORIZONDB_RESTORE_TIME"), sourceLocationConfig)
}

func (r HorizondbClusterResource) customerManagedKey(data acceptance.TestData, rotated bool) string {
	key, identityId, retention := os.Getenv("ARM_TEST_HORIZONDB_KEY_URI"), os.Getenv("ARM_TEST_HORIZONDB_IDENTITY_ID"), 7
	if rotated {
		key, identityId, retention = os.Getenv("ARM_TEST_HORIZONDB_ROTATED_KEY_URI"), os.Getenv("ARM_TEST_HORIZONDB_ROTATED_IDENTITY_ID"), 14
	}
	return fmt.Sprintf(`
%s
resource "azurerm_horizondb_cluster" "test" {
  name                          = "acctest-horizondb-%d"
  resource_group_name           = azurerm_resource_group.test.name
  location                      = azurerm_resource_group.test.location
  postgresql_version            = "18"
  administrator_login           = "acctestadmin"
  administrator_password        = "P@ssword-123!"
  backup_retention_days         = %d
  public_network_access_enabled = %t

  identity {
    type         = "UserAssigned"
    identity_ids = [%q, %q]
  }

  customer_managed_key {
    key_vault_key_id          = %q
    user_assigned_identity_id = %q
  }

  tags = {
    rotated = "%t"
  }
}
`, r.template(data), data.RandomInteger, retention, rotated,
		os.Getenv("ARM_TEST_HORIZONDB_IDENTITY_ID"), os.Getenv("ARM_TEST_HORIZONDB_ROTATED_IDENTITY_ID"),
		key, identityId, rotated)
}
