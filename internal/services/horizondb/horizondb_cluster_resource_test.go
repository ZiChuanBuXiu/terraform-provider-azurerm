// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/clusters"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
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
		{Config: r.resetSettings(data), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
	})
}

func TestAccHorizondbCluster_authentication(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.withAdministratorPassword(data, "P@ssword-123!"),
			Check:  check.That(data.ResourceName).ExistsInAzure(r),
		},
		data.ImportStep("administrator_login", "administrator_password"),
		{
			Config: r.withAdministratorPassword(data, "P@ssword-456!"),
			Check:  check.That(data.ResourceName).ExistsInAzure(r),
		},
		data.ImportStep("administrator_login", "administrator_password"),
	})
}

func TestAccHorizondbCluster_geoRedundantBackup(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.geoRedundantBackup(data),
			Check:  check.That(data.ResourceName).ExistsInAzure(r),
		},
		data.ImportStep("administrator_login", "administrator_password"),
	})
}

func TestAccHorizondbCluster_restore(t *testing.T) {
	r := HorizondbClusterResource{}
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	restoreVariables := config.Variables{}
	// Azure can omit creation-time restore metadata from GET responses.
	restoreImport := data.ImportStepFor("azurerm_horizondb_cluster.restored", "administrator_login", "administrator_password", "restore")
	restoreImport.ConfigVariables = restoreVariables
	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.basic(data),
			Check:  check.That(data.ResourceName).ExistsInAzure(r),
		},
		data.ImportStep("administrator_login", "administrator_password"),
		{
			PreConfig: func() {
				restoreVariables["point_in_time_utc"] = config.StringVariable(time.Now().UTC().Format(time.RFC3339))
				time.Sleep(time.Minute)
			},
			Config:          r.restore(data, false),
			ConfigVariables: restoreVariables,
			Check:           check.That("azurerm_horizondb_cluster.restored").ExistsInAzure(r),
		},
		restoreImport,
	})
}

func TestAccHorizondbCluster_restoreCrossRegion(t *testing.T) {
	r := HorizondbClusterResource{}
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	if data.Locations.Primary == data.Locations.Secondary {
		t.Skip("cross-region restore requires distinct primary and secondary locations with HorizonDB support")
	}
	restoreVariables := config.Variables{}
	// Azure can omit creation-time restore metadata from GET responses.
	restoreImport := data.ImportStepFor("azurerm_horizondb_cluster.restored", "administrator_login", "administrator_password", "restore")
	restoreImport.ConfigVariables = restoreVariables
	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.geoRedundantBackup(data),
			Check:  check.That(data.ResourceName).ExistsInAzure(r),
		},
		data.ImportStep("administrator_login", "administrator_password"),
		{
			PreConfig: func() {
				restoreVariables["point_in_time_utc"] = config.StringVariable(time.Now().UTC().Format(time.RFC3339))
				time.Sleep(time.Minute)
			},
			Config:          r.restore(data, true),
			ConfigVariables: restoreVariables,
			Check:           check.That("azurerm_horizondb_cluster.restored").ExistsInAzure(r),
		},
		restoreImport,
	})
}

func TestAccHorizondbCluster_identity(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceSequentialTest(t, r, []acceptance.TestStep{
		{
			Config: r.identity(data, 0),
			Check:  check.That(data.ResourceName).ExistsInAzure(r),
		},
		data.ImportStep("administrator_login", "administrator_password"),
		{
			Config: r.identity(data, 2),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(data.ResourceName, plancheck.ResourceActionUpdate),
				},
			},
			Check: check.That(data.ResourceName).ExistsInAzure(r),
		},
		data.ImportStep("administrator_login", "administrator_password"),
		{
			Config: r.identity(data, 1),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(data.ResourceName, plancheck.ResourceActionUpdate),
				},
			},
			Check: check.That(data.ResourceName).ExistsInAzure(r),
		},
		data.ImportStep("administrator_login", "administrator_password"),
		{
			Config: r.identity(data, 0),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(data.ResourceName, plancheck.ResourceActionUpdate),
				},
			},
			Check: check.That(data.ResourceName).ExistsInAzure(r),
		},
		data.ImportStep("administrator_login", "administrator_password"),
	})
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

func (r HorizondbClusterResource) template(data acceptance.TestData) string {
	return r.templateWithFeatures(data, "")
}

func (HorizondbClusterResource) templateWithFeatures(data acceptance.TestData, features string) string {
	return fmt.Sprintf(`
provider "azurerm" {
  features {
    %s
  }
}

resource "azurerm_resource_group" "test" {
  name     = "acctestRG-horizondb-%d"
  location = "%s"
}
`, features, data.RandomInteger, data.Locations.Primary)
}

func (r HorizondbClusterResource) basic(data acceptance.TestData) string {
	return r.withPool(data, "primary", 1, "Disabled", 4)
}

func (HorizondbClusterResource) pool() string {
	return `
pool {
  name                   = "primary"
  node_count             = 1
  high_availability_mode = "Disabled"

  compute {
    processor  = "AMD"
    generation = 1
    vcores     = 4
  }
}
`
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

  %s
}
`, r.template(data), data.RandomInteger, password, r.pool())
}

func (r HorizondbClusterResource) withPool(data acceptance.TestData, name string, nodes int, mode string, cores int) string {
	return r.withPoolAndFeatures(data, name, nodes, mode, cores, "")
}

func (r HorizondbClusterResource) withPoolAndFeatures(data acceptance.TestData, name string, nodes int, mode string, cores int, features string) string {
	return fmt.Sprintf(`
%s
resource "azurerm_horizondb_cluster" "test" {
  name                   = "acctest-horizondb-%d"
  resource_group_name    = azurerm_resource_group.test.name
  location               = azurerm_resource_group.test.location
  postgresql_version     = "18"
  administrator_login    = "acctestadmin"
  administrator_password = "P@ssword-123!"

  pool {
    name                   = %q
    node_count             = %d
    high_availability_mode = %q

    compute {
      processor  = "AMD"
      generation = 1
      vcores     = %d
    }
  }

}
`, r.templateWithFeatures(data, features), data.RandomInteger, name, nodes, mode, cores)
}

func (r HorizondbClusterResource) complete(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s

resource "azurerm_horizondb_cluster" "test" {
  name                          = "acctest-horizondb-%d"
  resource_group_name           = azurerm_resource_group.test.name
  location                      = azurerm_resource_group.test.location
  postgresql_version            = "18"
  administrator_login           = "acctestadmin"
  administrator_password        = "P@ssword-456!"
  backup_retention_days         = 14
  public_network_access_enabled = true

  pool {
    name                   = "primary"
    node_count             = 1
    high_availability_mode = "Disabled"

    compute {
      processor  = "AMD"
      generation = 1
      vcores     = 4
    }
  }

  tags = {
    environment = "acceptance"
    retention   = "14"
  }
}
`, r.template(data), data.RandomInteger)
}

func (r HorizondbClusterResource) resetSettings(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s

resource "azurerm_horizondb_cluster" "test" {
  name                          = "acctest-horizondb-%d"
  resource_group_name           = azurerm_resource_group.test.name
  location                      = azurerm_resource_group.test.location
  postgresql_version            = "18"
  administrator_login           = "acctestadmin"
  administrator_password        = "P@ssword-123!"
  backup_retention_days         = 7
  public_network_access_enabled = false

  %s
}
`, r.template(data), data.RandomInteger, r.pool())
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

  %s

  tags = {
    environment = "acceptance"
    retention   = "%d"
  }
}
`, r.template(data), data.RandomInteger, retention, publicNetworkAccessEnabled, r.pool(), retention)
}

func (r HorizondbClusterResource) requiresImport(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s
resource "azurerm_horizondb_cluster" "import" {
  name                   = azurerm_horizondb_cluster.test.name
  resource_group_name    = azurerm_horizondb_cluster.test.resource_group_name
  location               = azurerm_horizondb_cluster.test.location
  postgresql_version     = azurerm_horizondb_cluster.test.postgresql_version
  administrator_login    = azurerm_horizondb_cluster.test.administrator_login
  administrator_password = azurerm_horizondb_cluster.test.administrator_password

  %s
}
`, r.basic(data), r.pool())
}

func (r HorizondbClusterResource) geoRedundantBackup(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s

resource "azurerm_horizondb_cluster" "test" {
  name                         = "acctest-horizondb-%d"
  resource_group_name          = azurerm_resource_group.test.name
  location                     = azurerm_resource_group.test.location
  postgresql_version           = "18"
  administrator_login          = "acctestadmin"
  administrator_password       = "P@ssword-123!"
  geo_redundant_backup_enabled = true

  %s
}
`, r.template(data), data.RandomInteger, r.pool())
}

func (r HorizondbClusterResource) restore(data acceptance.TestData, crossRegion bool) string {
	source := r.basic(data)
	targetLocation := "azurerm_resource_group.test.location"
	sourceLocation := ""
	if crossRegion {
		source = r.geoRedundantBackup(data)
		targetLocation = fmt.Sprintf("%q", data.Locations.Secondary)
		sourceLocation = "source_location = azurerm_horizondb_cluster.test.location"
	}
	return fmt.Sprintf(`
%s

variable "point_in_time_utc" {
  type = string
}

resource "azurerm_horizondb_cluster" "restored" {
  name                   = "acctest-horizondb-restored-%d"
  resource_group_name    = azurerm_resource_group.test.name
  location               = %s
  create_mode            = "PointInTimeRestore"
  administrator_login    = "acctestadmin"
  administrator_password = "P@ssword-123!"

  restore {
    source_horizondb_cluster_id = azurerm_horizondb_cluster.test.id
    point_in_time_utc           = var.point_in_time_utc
    %s
  }

  %s
}
`, source, data.RandomInteger, targetLocation, sourceLocation, r.pool())
}

func (r HorizondbClusterResource) identityTemplate(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s

resource "azurerm_user_assigned_identity" "test" {
  count               = 2
  name                = "acctest-horizondb-%d-${count.index}"
  location            = azurerm_resource_group.test.location
  resource_group_name = azurerm_resource_group.test.name
}
`, r.template(data), data.RandomInteger)
}

func (r HorizondbClusterResource) identity(data acceptance.TestData, count int) string {
	identity := ""
	if count > 0 {
		identity = fmt.Sprintf(`
identity {
  type         = "UserAssigned"
  identity_ids = slice(azurerm_user_assigned_identity.test[*].id, 0, %d)
}
`, count)
	}
	return fmt.Sprintf(`
%s

resource "azurerm_horizondb_cluster" "test" {
  name                   = "acctest-horizondb-%d"
  resource_group_name    = azurerm_resource_group.test.name
  location               = azurerm_resource_group.test.location
  postgresql_version     = "18"
  administrator_login    = "acctestadmin"
  administrator_password = "P@ssword-123!"

  %s

  %s

  tags = {
    identity_count = "%d"
  }
}
`, r.identityTemplate(data), data.RandomInteger, identity, r.pool(), count)
}
