// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/provider/framework"
)

func TestAccHorizondbParameterGroup_list(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_horizondb_parameter_group", "test")
	var identities, values []querycheck.QueryResultCheck
	for index := range 2 {
		identity := map[string]knownvalue.Check{
			"name":                knownvalue.StringExact(fmt.Sprintf("acctest-parameters-%d-%d", data.RandomInteger, index)),
			"resource_group_name": knownvalue.StringExact(fmt.Sprintf("acctestRG-horizondb-%d", data.RandomInteger)),
			"subscription_id":     knownvalue.StringExact(data.Subscriptions.Primary),
		}
		identities = append(identities, querycheck.ExpectIdentity("azurerm_horizondb_parameter_group.list", identity))
		values = append(values, querycheck.ExpectResourceKnownValues("azurerm_horizondb_parameter_group.list",
			queryfilter.ByResourceIdentity(identity), []querycheck.KnownValueCheck{{
				Path: tfjsonpath.New("postgresql_version"), KnownValue: knownvalue.StringExact("18"),
			}}))
	}
	resource.Test(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_14_0)},
		ProtoV5ProviderFactories: framework.ProtoV5ProviderFactoriesInit(context.Background(), "azurerm"),
		Steps: []resource.TestStep{
			{Config: fmt.Sprintf(`
%s
resource "azurerm_horizondb_parameter_group" "test" {
  count               = 2
  name                = "acctest-parameters-%d-${count.index}"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
  postgresql_version  = "18"
}
`, (HorizondbParameterGroupResource{}).template(data), data.RandomInteger)},
			{
				Query: true,
				Config: `
list "azurerm_horizondb_parameter_group" "list" {
  provider = azurerm
  config {}
}
`,
				QueryResultChecks: append(identities, querycheck.ExpectLengthAtLeast("azurerm_horizondb_parameter_group.list", 2)),
			},
			{
				Query: true,
				Config: `
list "azurerm_horizondb_parameter_group" "list" {
  provider         = azurerm
  include_resource = true
  config {
    resource_group_name = azurerm_resource_group.test.name
  }
}
`,
				QueryResultChecks: append(append(values, identities...), querycheck.ExpectLength("azurerm_horizondb_parameter_group.list", 2)),
			},
		},
	})
}
