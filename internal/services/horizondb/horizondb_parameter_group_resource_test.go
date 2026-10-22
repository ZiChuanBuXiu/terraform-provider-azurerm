// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/parametergroups"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance/check"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

type HorizondbParameterGroupResource struct{}

func TestAccHorizondbParameterGroup_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_parameter_group", "test")
	r := HorizondbParameterGroupResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{Config: r.basic(data), Check: check.That(data.ResourceName).ExistsInAzure(r)}, data.ImportStep(),
	})
}
func TestAccHorizondbParameterGroup_complete(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_parameter_group", "test")
	r := HorizondbParameterGroupResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{Config: r.complete(data), Check: check.That(data.ResourceName).ExistsInAzure(r)}, data.ImportStep(),
	})
}
func TestAccHorizondbParameterGroup_requiresImport(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_parameter_group", "test")
	r := HorizondbParameterGroupResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{Config: r.basic(data), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		data.RequiresImportErrorStep(r.requiresImport),
	})
}
func TestAccHorizondbParameterGroup_update(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_parameter_group", "test")
	r := HorizondbParameterGroupResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{Config: r.basic(data), Check: check.That(data.ResourceName).ExistsInAzure(r)}, data.ImportStep(),
		{Config: r.complete(data), Check: check.That(data.ResourceName).ExistsInAzure(r)}, data.ImportStep(),
		{Config: r.settings(data, "", ""), Check: check.That(data.ResourceName).ExistsInAzure(r)}, data.ImportStep(),
	})
}
func TestAccHorizondbParameterGroup_removalRejected(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_parameter_group", "test")
	r := HorizondbParameterGroupResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{Config: r.complete(data), Check: check.That(data.ResourceName).ExistsInAzure(r)},
		{Config: r.basic(data), ExpectError: regexp.MustCompile("removing parameter overrides")},
	})
}
func (HorizondbParameterGroupResource) Exists(ctx context.Context, client *clients.Client, state *pluginsdk.InstanceState) (*bool, error) {
	id, err := parametergroups.ParseProviderParameterGroupID(state.ID)
	if err != nil {
		return nil, err
	}
	resp, err := client.HorizonDB.ParameterGroupsClient.Get(ctx, *id)
	if err != nil {
		return nil, fmt.Errorf("retrieving %s: %+v", id, err)
	}
	return pointer.To(resp.Model != nil), nil
}
func (HorizondbParameterGroupResource) template(data acceptance.TestData) string {
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
func (r HorizondbParameterGroupResource) basic(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s
resource "azurerm_horizondb_parameter_group" "test" {
  name                = "acctest-parameters-%d"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
  postgresql_version  = "18"
}
`, r.template(data), data.RandomInteger)
}
func (r HorizondbParameterGroupResource) complete(data acceptance.TestData) string {
	return r.settings(data, "Acceptance test parameters", "acceptance")
}
func (r HorizondbParameterGroupResource) settings(data acceptance.TestData, description, applicationName string) string {
	return fmt.Sprintf(`
%s
resource "azurerm_horizondb_parameter_group" "test" {
  name                = "acctest-parameters-%d"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
  postgresql_version  = "18"
  description         = %q
  parameters = {
    application_name = %q
    xmlbinary        = "base64"
  }
  tags = {
    environment = %q
  }
}
`, r.template(data), data.RandomInteger, description, applicationName, applicationName)
}
func (r HorizondbParameterGroupResource) requiresImport(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s
resource "azurerm_horizondb_parameter_group" "import" {
  name                = azurerm_horizondb_parameter_group.test.name
  resource_group_name = azurerm_horizondb_parameter_group.test.resource_group_name
  location            = azurerm_horizondb_parameter_group.test.location
  postgresql_version  = azurerm_horizondb_parameter_group.test.postgresql_version
}
`, r.basic(data))
}
