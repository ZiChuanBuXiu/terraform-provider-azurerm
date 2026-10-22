package horizondb_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance/check"
)

func TestAccHorizondbDefaultParameterGroupDataSource_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "data.azurerm_horizondb_default_parameter_group", "test")
	data.DataSourceTest(t, []acceptance.TestStep{
		{
			Config: fmt.Sprintf(`
provider "azurerm" {
  features {}
}

data "azurerm_horizondb_default_parameter_group" "test" {
  location           = %q
  postgresql_version = "17"
}
`, data.Locations.Primary),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).Key("id").Exists(),
				check.That(data.ResourceName).Key("name").Exists(),
				check.That(data.ResourceName).Key("postgresql_version").HasValue("17"),
			),
		},
	})
}

func TestAccHorizondbPrivateLinkResourcesDataSource_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "data.azurerm_horizondb_private_link_resources", "test")
	r := HorizondbClusterResource{}
	data.DataSourceTest(t, []acceptance.TestStep{
		{
			Config: fmt.Sprintf(`
%s

data "azurerm_horizondb_private_link_resources" "test" {
  horizondb_cluster_id = azurerm_horizondb_cluster.test.id
}
`, r.complete(data)),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).Key("id").MatchesOtherKey(check.That("azurerm_horizondb_cluster.test").Key("id")),
				check.That(data.ResourceName).Key("private_link_resources.0.group_id").Exists(),
				check.That(data.ResourceName).Key("private_link_resources.0.required_zone_names.0").HasValue("privatelink.horizondb.azure.com"),
			),
		},
	})
}
