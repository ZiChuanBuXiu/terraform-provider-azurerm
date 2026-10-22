// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance/check"
)

func TestAccHorizondbParameterGroupConnectionsDataSource_empty(t *testing.T) {
	data := acceptance.BuildTestData(t, "data.azurerm_horizondb_parameter_group_connections", "test")
	data.DataSourceTest(t, []acceptance.TestStep{{
		Config: fmt.Sprintf(`
%s
data "azurerm_horizondb_parameter_group_connections" "test" {
  horizondb_parameter_group_id = azurerm_horizondb_parameter_group.test.id
}
`, (HorizondbParameterGroupResource{}).complete(data)),
		Check: check.That(data.ResourceName).Key("horizondb_pool_ids.#").HasValue("0"),
	}})
}
func TestAccHorizondbParameterGroupConnectionsDataSource_connected(t *testing.T) {
	r := HorizondbPoolParameterGroupAssociationResource{}
	r.preCheck(t)
	data := acceptance.BuildTestData(t, "data.azurerm_horizondb_parameter_group_connections", "test")
	data.DataSourceTestInSequence(t, []acceptance.TestStep{{
		Config: fmt.Sprintf(`
%s
data "azurerm_horizondb_parameter_group_connections" "test" {
  horizondb_parameter_group_id = azurerm_horizondb_pool_parameter_group_association.test.horizondb_parameter_group_id
}
`, r.complete(data)),
		Check: acceptance.ComposeTestCheckFunc(
			check.That(data.ResourceName).Key("id").Exists(),
			check.That(data.ResourceName).Key("horizondb_pool_ids.#").HasValue("1"),
		),
	}})
}
