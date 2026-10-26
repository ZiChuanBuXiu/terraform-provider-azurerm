// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb_test

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/clusters"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/passwordauthentications"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/pools"
	"github.com/hashicorp/go-azure-sdk/sdk/client/pollers"
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
		{
			Config: r.withAdministratorPassword(data, "P@ssword-123!"),
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
	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				data.CheckWithClient(r.waitForRestorePoint),
			),
		},
		{RefreshState: true},
		data.ImportStep("administrator_login", "administrator_password"),
		{
			Config: r.restore(data, false),
			Check:  check.That("azurerm_horizondb_cluster.restored").ExistsInAzure(r),
		},
		data.ImportStepFor("azurerm_horizondb_cluster.restored", "administrator_login", "administrator_password"),
	})
}

func TestAccHorizondbCluster_restoreCrossRegion(t *testing.T) {
	r := HorizondbClusterResource{}
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	if data.Locations.Primary == data.Locations.Secondary {
		t.Skip("cross-region restore requires distinct primary and secondary locations with HorizonDB support")
	}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.geoRedundantBackup(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				data.CheckWithClient(r.waitForRestorePoint),
			),
		},
		{RefreshState: true},
		data.ImportStep("administrator_login", "administrator_password"),
		{
			Config: r.restore(data, true),
			Check:  check.That("azurerm_horizondb_cluster.restored").ExistsInAzure(r),
		},
		// The API does not return the requested timestamp for cross-region restores.
		data.ImportStepFor("azurerm_horizondb_cluster.restored", "administrator_login", "administrator_password", "restore.0.point_in_time_utc"),
	})
}

func TestAccHorizondbCluster_versionRequired(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{{
		Config: fmt.Sprintf(`
%s
resource "azurerm_horizondb_cluster" "test" {
  name                   = "acctest-horizondb-%d"
  resource_group_name    = azurerm_resource_group.test.name
  location               = azurerm_resource_group.test.location
  administrator_login    = "acctestadmin"
  administrator_password = "P@ssword-123!"

  %s
}
`, r.template(data), data.RandomInteger, r.pool()),
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
  name                   = "acctest-horizondb-%d"
  resource_group_name    = azurerm_resource_group.test.name
  location               = azurerm_resource_group.test.location
  create_mode            = "PointInTimeRestore"
  administrator_login    = "acctestadmin"
  administrator_password = "P@ssword-123!"

  %s
}
`, r.template(data), data.RandomInteger, r.pool()),
			ExpectError: regexp.MustCompile("`restore` is required when `create_mode` is `PointInTimeRestore`"),
		},
		{
			Config: fmt.Sprintf(`
%s
resource "azurerm_horizondb_cluster" "test" {
  name                   = "acctest-horizondb-%d"
  resource_group_name    = azurerm_resource_group.test.name
  location               = azurerm_resource_group.test.location
  postgresql_version     = "18"
  administrator_login    = "acctestadmin"
  administrator_password = "P@ssword-123!"

  restore {
    source_horizondb_cluster_id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/example-rg/providers/Microsoft.HorizonDb/clusters/source"
    point_in_time_utc           = "2026-10-01T00:00:00Z"
  }

  %s
}
`, r.template(data), data.RandomInteger, r.pool()),
			ExpectError: regexp.MustCompile("`restore` can only be specified when `create_mode` is `PointInTimeRestore`"),
		},
		{
			Config: fmt.Sprintf(`
%s

resource "azurerm_horizondb_cluster" "test" {
  name                   = "acctest-horizondb-%d"
  resource_group_name    = azurerm_resource_group.test.name
  location               = azurerm_resource_group.test.location
  administrator_login    = "acctestadmin"
  administrator_password = "P@ssword-123!"
  create_mode            = "PointInTimeRestore"
  postgresql_version     = "18"

  restore {
    source_horizondb_cluster_id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/example-rg/providers/Microsoft.HorizonDb/clusters/source"
    point_in_time_utc           = "2026-10-01T00:00:00Z"
  }

  %s
}
`, r.template(data), data.RandomInteger, r.pool()),
			ExpectError: regexp.MustCompile("`postgresql_version` must be omitted when `create_mode` is `PointInTimeRestore`"),
		},
	})
}

func TestAccHorizondbCluster_pool(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{Config: r.withPool(data, "primary", 1, "Disabled", 4), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
		{
			Config: r.withPoolComputeType(data),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			},
		},
		data.ImportStep("administrator_login", "administrator_password"),
		{Config: r.withPool(data, "primary", 2, "ZoneRedundant", 8), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
		{Config: r.withPool(data, "primary", 2, "AnyZone", 8), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
		{Config: r.withPool(data, "primary", 2, "AnyZone", 16), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.ImportStep("administrator_login", "administrator_password"),
		{Config: r.basic(data), Check: check.That(data.ResourceName).ExistsInAzure(r)},
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
			Config:      r.withPool(data, "primary", 2, "Disabled", 4),
			ExpectError: regexp.MustCompile("`pool.high_availability_mode` must be `Disabled`"),
		},
		{
			Config:      r.withPool(data, fmt.Sprintf("acctest-horizondb-%d", data.RandomInteger), 1, "Disabled", 4),
			ExpectError: regexp.MustCompile("`pool.name` must differ"),
		},
	})
}

func TestAccHorizondbCluster_identity(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceSequentialTest(t, r, []acceptance.TestStep{
		{
			Config: r.identity(data, false),
			Check:  check.That(data.ResourceName).ExistsInAzure(r),
		},
		data.ImportStep("administrator_login", "administrator_password"),
		{
			Config: r.identity(data, true),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(data.ResourceName, plancheck.ResourceActionReplace),
				},
			},
			Check: check.That(data.ResourceName).ExistsInAzure(r),
		},
		data.ImportStep("administrator_login", "administrator_password"),
		{
			Config: r.identity(data, false),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(data.ResourceName, plancheck.ResourceActionReplace),
				},
			},
			Check: check.That(data.ResourceName).ExistsInAzure(r),
		},
		data.ImportStep("administrator_login", "administrator_password"),
	})
}

func TestAccHorizondbCluster_overwriteExisting(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_cluster", "test")
	r := HorizondbClusterResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.overwritePrerequisites(data),
			Check:  data.CheckWithClientForResource(r.prepareOverwrite(data), "azurerm_horizondb_cluster.source"),
		},
		{
			Config: r.overwriteExisting(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				data.CheckWithClient(r.checkOverwritten),
			),
		},
		data.ImportStep("administrator_login", "administrator_password"),
	})
}

func (r HorizondbClusterResource) prepareOverwrite(data acceptance.TestData) acceptance.ClientCheckFunc {
	return func(ctx context.Context, client *clients.Client, state *pluginsdk.InstanceState) error {
		source, err := clusters.ParseClusterID(state.ID)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
		defer cancel()

		sourcePoolId := pools.NewPoolID(source.SubscriptionId, source.ResourceGroupName, source.ClusterName, "primary")
		sourcePool, err := client.HorizonDB.PoolsClient.Get(ctx, sourcePoolId)
		if err != nil {
			return fmt.Errorf("retrieving %s: %+v", sourcePoolId, err)
		}
		if sourcePool.Model == nil || sourcePool.Model.Properties == nil {
			return fmt.Errorf("retrieving %s: pool properties are missing", sourcePoolId)
		}
		id := clusters.NewClusterID(source.SubscriptionId, source.ResourceGroupName, fmt.Sprintf("acctest-horizondb-%d", data.RandomInteger))
		payload := clusters.Cluster{
			Location: data.Locations.Primary,
			Properties: &clusters.ClusterProperties{
				CreationMode:      pointer.To(clusters.ClusterCreateModeCreate),
				PostgreSqlVersion: pointer.To(clusters.PostgreSqlVersion("18")),
				Backup: &clusters.BackupProperties{
					GeoRedundancy: clusters.GeoRedundancyStateDisabled,
					RetentionDays: pointer.To(int64(7)),
				},
				Network: &clusters.Network{PublicAccess: pointer.To(clusters.PublicNetworkAccessStateEnabled)},
			},
		}
		if err := client.HorizonDB.ClustersClient.CreateOrReplaceThenPoll(ctx, id, payload); err != nil {
			return fmt.Errorf("creating pre-existing %s: %+v", id, err)
		}
		poolId := pools.NewPoolID(id.SubscriptionId, id.ResourceGroupName, id.ClusterName, "primary")
		properties := *sourcePool.Model.Properties
		properties.ProvisioningState = nil
		properties.State = nil
		if err := client.HorizonDB.PoolsClient.CreateOrReplaceThenPoll(ctx, poolId, pools.Pool{Properties: &properties}); err != nil {
			return fmt.Errorf("creating pre-existing %s: %+v", poolId, err)
		}
		return nil
	}
}

func (HorizondbClusterResource) checkOverwritten(ctx context.Context, client *clients.Client, state *pluginsdk.InstanceState) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	id, err := clusters.ParseClusterID(state.ID)
	if err != nil {
		return err
	}
	result, err := client.HorizonDB.PoolsClient.ListComplete(ctx, pools.NewClusterID(id.SubscriptionId, id.ResourceGroupName, id.ClusterName))
	if err != nil {
		return fmt.Errorf("listing pools for %s: %+v", id, err)
	}
	if len(result.Items) != 1 {
		return fmt.Errorf("expected overwrite to retain one pool for %s, got %d pools", id, len(result.Items))
	}
	if result.Items[0].Properties == nil || result.Items[0].Properties.ComputeModel.VCores != 8 {
		return errors.New("expected overwrite to update the pre-existing pool to 8 vCores")
	}
	authId := passwordauthentications.NewClusterID(id.SubscriptionId, id.ResourceGroupName, id.ClusterName)
	auth, err := client.HorizonDB.PasswordAuthenticationsClient.Get(ctx, authId)
	if err != nil {
		return fmt.Errorf("retrieving password authentication for %s: %+v", id, err)
	}
	if auth.Model == nil || auth.Model.Properties == nil || auth.Model.Properties.State != passwordauthentications.AuthenticationStateEnabled {
		return errors.New("expected overwrite to configure enabled password authentication")
	}
	return nil
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

func (r HorizondbClusterResource) withPoolComputeType(data acceptance.TestData) string {
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
    name                   = "primary"
    node_count             = 1
    high_availability_mode = "Disabled"

    compute {
      type       = "Provisioned"
      processor  = "AMD"
      generation = 1
      vcores     = 4
    }
  }
}
`, r.template(data), data.RandomInteger)
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

resource "azurerm_horizondb_cluster" "restored" {
  name                   = "acctest-horizondb-restored-%d"
  resource_group_name    = azurerm_resource_group.test.name
  location               = %s
  create_mode            = "PointInTimeRestore"
  administrator_login    = "acctestadmin"
  administrator_password = "P@ssword-123!"

  restore {
    source_horizondb_cluster_id = azurerm_horizondb_cluster.test.id
    point_in_time_utc           = azurerm_horizondb_cluster.test.earliest_restore_point_utc
    %s
  }

  %s
}
`, source, data.RandomInteger, targetLocation, sourceLocation, r.pool())
}

func (HorizondbClusterResource) waitForRestorePoint(ctx context.Context, client *clients.Client, state *pluginsdk.InstanceState) error {
	id, err := clusters.ParseClusterID(state.ID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Minute)
	defer cancel()

	poller := pollers.NewPoller(horizondbRestorePointPoller{client: client.HorizonDB.ClustersClient, id: *id}, 30*time.Second, pollers.DefaultNumberOfDroppedConnectionsToAllow)
	if err := poller.PollUntilDone(ctx); err != nil {
		return fmt.Errorf("waiting for a usable earliest restore point for %s: %+v", id, err)
	}
	return nil
}

type horizondbRestorePointPoller struct {
	client *clusters.ClustersClient
	id     clusters.ClusterId
}

func (p horizondbRestorePointPoller) Poll(ctx context.Context) (*pollers.PollResult, error) {
	resp, err := p.client.Get(ctx, p.id)
	if err != nil {
		return nil, fmt.Errorf("retrieving %s: %+v", p.id, err)
	}
	if resp.Model == nil || resp.Model.Properties == nil {
		return nil, fmt.Errorf("retrieving %s: cluster properties are missing", p.id)
	}
	status := pollers.PollingStatusInProgress
	if backup := resp.Model.Properties.Backup; backup != nil && pointer.From(backup.EarliestRestorePointUtc) != "" {
		point, err := backup.GetEarliestRestorePointUtcAsTime()
		if err != nil {
			return nil, fmt.Errorf("parsing earliest restore point for %s: %+v", p.id, err)
		}
		if point != nil && !point.IsZero() && point.Before(time.Now().UTC()) {
			status = pollers.PollingStatusSucceeded
		}
	}
	return &pollers.PollResult{Status: status, PollInterval: 30 * time.Second}, nil
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

func (r HorizondbClusterResource) identity(data acceptance.TestData, enabled bool) string {
	identity := ""
	if enabled {
		identity = `
identity {
  type         = "UserAssigned"
  identity_ids = azurerm_user_assigned_identity.test[*].id
}
`
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
}
`, r.identityTemplate(data), data.RandomInteger, identity, r.pool())
}

func (r HorizondbClusterResource) overwritePrerequisites(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s

resource "azurerm_horizondb_cluster" "source" {
  name                   = "acctest-horizondb-source-%d"
  resource_group_name    = azurerm_resource_group.test.name
  location               = azurerm_resource_group.test.location
  postgresql_version     = "18"
  administrator_login    = "acctestadmin"
  administrator_password = "P@ssword-123!"

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
}
`, r.templateWithFeatures(data, "skip_import_check_on_create_and_allow_overwriting_existing_resources = true"), data.RandomInteger)
}

func (r HorizondbClusterResource) overwriteExisting(data acceptance.TestData) string {
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
  public_network_access_enabled = false

  pool {
    name                   = "primary"
    node_count             = 1
    high_availability_mode = "Disabled"

    compute {
      processor  = "AMD"
      generation = 1
      vcores     = 8
    }
  }

  tags = {
    environment = "overwritten"
  }
}
`, r.overwritePrerequisites(data), data.RandomInteger)
}
