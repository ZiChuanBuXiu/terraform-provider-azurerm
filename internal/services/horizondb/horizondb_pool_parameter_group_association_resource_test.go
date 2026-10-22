// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/defaultparametergroups"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/pools"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance/check"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

type HorizondbPoolParameterGroupAssociationResource struct{}

func TestAccHorizondbPoolParameterGroupAssociation_basic(t *testing.T) {
	r := HorizondbPoolParameterGroupAssociationResource{}
	r.preCheck(t)
	data := acceptance.BuildTestData(t, "azurerm_horizondb_pool_parameter_group_association", "test")
	data.ResourceSequentialTest(t, r, []acceptance.TestStep{
		{Config: r.basic(data), Check: check.That(data.ResourceName).ExistsInAzure(r)}, data.ImportStep(),
	})
}
func TestAccHorizondbPoolParameterGroupAssociation_complete(t *testing.T) {
	r := HorizondbPoolParameterGroupAssociationResource{}
	r.preCheck(t)
	data := acceptance.BuildTestData(t, "azurerm_horizondb_pool_parameter_group_association", "test")
	data.ResourceSequentialTest(t, r, []acceptance.TestStep{
		{Config: r.complete(data), Check: check.That(data.ResourceName).ExistsInAzure(r)}, data.ImportStep(),
	})
}
func TestAccHorizondbPoolParameterGroupAssociation_requiresImport(t *testing.T) {
	r := HorizondbPoolParameterGroupAssociationResource{}
	r.preCheck(t)
	data := acceptance.BuildTestData(t, "azurerm_horizondb_pool_parameter_group_association", "test")
	data.ResourceSequentialTest(t, r, []acceptance.TestStep{
		{Config: r.basic(data), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.RequiresImportErrorStep(r.requiresImport),
	})
}
func TestAccHorizondbPoolParameterGroupAssociation_update(t *testing.T) {
	r := HorizondbPoolParameterGroupAssociationResource{}
	r.preCheck(t)
	data := acceptance.BuildTestData(t, "azurerm_horizondb_pool_parameter_group_association", "test")
	data.ResourceSequentialTest(t, r, []acceptance.TestStep{
		{Config: r.settings(data, 0, "Immediate"), Check: check.That(data.ResourceName).ExistsInAzure(r)}, data.ImportStep(),
		{Config: r.settings(data, 1, "PendingReboot"), Check: check.That(data.ResourceName).ExistsInAzure(r)}, data.ImportStep(),
		{Config: r.settings(data, 0, "Immediate"), Check: check.That(data.ResourceName).ExistsInAzure(r)}, data.ImportStep(),
	})
}
func (HorizondbPoolParameterGroupAssociationResource) preCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("ARM_TEST_HORIZONDB_POOL_ID") == "" {
		t.Skip("ARM_TEST_HORIZONDB_POOL_ID must identify a dedicated PostgreSQL 18 test pool in ARM_TEST_LOCATION, initially using its default parameter group")
	}
	if _, err := pools.ParsePoolID(os.Getenv("ARM_TEST_HORIZONDB_POOL_ID")); err != nil {
		t.Fatal(err)
	}
}
func (HorizondbPoolParameterGroupAssociationResource) Exists(ctx context.Context, client *clients.Client, state *pluginsdk.InstanceState) (*bool, error) {
	id, err := pools.ParsePoolID(state.ID)
	if err != nil {
		return nil, err
	}
	resp, err := client.HorizonDB.PoolsClient.Get(ctx, *id)
	if err != nil {
		return nil, fmt.Errorf("retrieving %s: %+v", id, err)
	}
	if resp.Model == nil || resp.Model.Properties == nil {
		return nil, fmt.Errorf("retrieving %s: pool properties are missing", id)
	}
	if _, err := defaultparametergroups.ParseParameterGroupIDInsensitively(resp.Model.Properties.ParameterGroup.ParameterGroupResourceId); err == nil {
		return pointer.To(false), nil
	}
	return pointer.To(strings.EqualFold(resp.Model.Properties.ParameterGroup.ParameterGroupResourceId, state.Attributes["horizondb_parameter_group_id"])), nil
}
func (r HorizondbPoolParameterGroupAssociationResource) basic(data acceptance.TestData) string {
	return r.settings(data, 0, "Immediate")
}
func (r HorizondbPoolParameterGroupAssociationResource) complete(data acceptance.TestData) string {
	return r.settings(data, 0, "PendingReboot")
}
func (HorizondbPoolParameterGroupAssociationResource) settings(data acceptance.TestData, groupIndex int, applyMethod string) string {
	return fmt.Sprintf(`
%s
resource "azurerm_horizondb_parameter_group" "test" {
  count               = 2
  name                = "acctest-parameters-%d-${count.index}"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
  postgresql_version  = "18"
  parameters = {
    application_name = "acceptance-${count.index}"
  }
}
resource "azurerm_horizondb_pool_parameter_group_association" "test" {
  horizondb_pool_id            = %q
  horizondb_parameter_group_id = azurerm_horizondb_parameter_group.test[%d].id
  apply_method                 = %q
}
`, (HorizondbParameterGroupResource{}).template(data), data.RandomInteger, os.Getenv("ARM_TEST_HORIZONDB_POOL_ID"), groupIndex, applyMethod)
}
func (r HorizondbPoolParameterGroupAssociationResource) requiresImport(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s
resource "azurerm_horizondb_pool_parameter_group_association" "import" {
  horizondb_pool_id            = azurerm_horizondb_pool_parameter_group_association.test.horizondb_pool_id
  horizondb_parameter_group_id = azurerm_horizondb_pool_parameter_group_association.test.horizondb_parameter_group_id
  apply_method                 = azurerm_horizondb_pool_parameter_group_association.test.apply_method
}
`, r.basic(data))
}
