// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	customstatecheck "github.com/hashicorp/terraform-provider-azurerm/internal/acceptance/statecheck"
)

func TestAccHorizondbPoolParameterGroupAssociation_resourceIdentity(t *testing.T) {
	for _, v := range []string{"ARM_TEST_HORIZONDB_POOL_ID"} {
		if os.Getenv(v) == "" {
			t.Skipf("Skipping as `%s` was not specified", v)
		}
	}

	data := acceptance.BuildTestData(t, "azurerm_horizondb_pool_parameter_group_association", "test")
	r := HorizondbPoolParameterGroupAssociationResource{}

	checkedFields := map[string]struct{}{
		"cluster_name":        {},
		"name":                {},
		"resource_group_name": {},
		"subscription_id":     {},
	}

	data.ResourceIdentityTest(t, []acceptance.TestStep{
		{
			Config: r.basic(data),
			ConfigStateChecks: []statecheck.StateCheck{
				customstatecheck.ExpectAllIdentityFieldsAreChecked("azurerm_horizondb_pool_parameter_group_association.test", checkedFields),
				customstatecheck.ExpectStateContainsIdentityValueAtPath("azurerm_horizondb_pool_parameter_group_association.test", tfjsonpath.New("cluster_name"), tfjsonpath.New("horizondb_pool_id")),
				customstatecheck.ExpectStateContainsIdentityValueAtPath("azurerm_horizondb_pool_parameter_group_association.test", tfjsonpath.New("name"), tfjsonpath.New("horizondb_pool_id")),
				customstatecheck.ExpectStateContainsIdentityValueAtPath("azurerm_horizondb_pool_parameter_group_association.test", tfjsonpath.New("resource_group_name"), tfjsonpath.New("horizondb_pool_id")),
				customstatecheck.ExpectStateContainsIdentityValueAtPath("azurerm_horizondb_pool_parameter_group_association.test", tfjsonpath.New("subscription_id"), tfjsonpath.New("horizondb_pool_id")),
			},
		},
		data.ImportBlockWithResourceIdentityStep(false),
		data.ImportBlockWithIDStep(false),
	}, false)
}
