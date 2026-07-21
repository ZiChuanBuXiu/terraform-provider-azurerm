// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package cognitive_test

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
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

func TestAccCognitiveAccountProjectConnectionProjectManagedIdentity_list(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_cognitive_account_project_connection_project_managed_identity", "test")
	r := CognitiveAccountProjectConnectionProjectManagedIdentityResource{}

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0),
		},
		ProtoV5ProviderFactories: framework.ProtoV5ProviderFactoriesInit(context.Background(), "azurerm"),
		Steps: []resource.TestStep{
			{
				Config: r.basicList(data),
			},
			{
				Query:  true,
				Config: r.listQuery(data),
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectLengthAtLeast("azurerm_cognitive_account_project_connection_project_managed_identity.list", 2),
					querycheck.ExpectIdentity(
						"azurerm_cognitive_account_project_connection_project_managed_identity.list",
						map[string]knownvalue.Check{
							"name":                knownvalue.StringRegexp(regexp.MustCompile(strconv.Itoa(data.RandomInteger))),
							"resource_group_name": knownvalue.StringRegexp(regexp.MustCompile(strconv.Itoa(data.RandomInteger))),
							"subscription_id":     knownvalue.StringExact(data.Subscriptions.Primary),
							"account_name":        knownvalue.StringRegexp(regexp.MustCompile(strconv.Itoa(data.RandomInteger))),
							"project_name":        knownvalue.StringRegexp(regexp.MustCompile(strconv.Itoa(data.RandomInteger))),
						},
					),
					querycheck.ExpectResourceKnownValues(
						"azurerm_cognitive_account_project_connection_project_managed_identity.list",
						queryfilter.ByDisplayName(knownvalue.StringRegexp(regexp.MustCompile("acctest-conn2-"))),
						[]querycheck.KnownValueCheck{
							{
								Path:       tfjsonpath.New("authentication_type"),
								KnownValue: knownvalue.StringExact("ProjectManagedIdentity"),
							},
						},
					),
				},
			},
		},
	})
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) listQuery(data acceptance.TestData) string {
	return `
list "azurerm_cognitive_account_project_connection_project_managed_identity" "list" {
  provider = azurerm
  config {
    cognitive_account_project_id = azurerm_cognitive_account_project.test.id
  }
  include_resource = true
}
`
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) basicList(data acceptance.TestData) string {
	return fmt.Sprintf(`
%[1]s

resource "azurerm_storage_account" "test" {
  name                     = "acctestsa%[3]s"
  resource_group_name      = azurerm_resource_group.test.name
  location                 = azurerm_resource_group.test.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
}

resource "azurerm_cognitive_account_project_connection_project_managed_identity" "test2" {
  name                         = "acctest-conn2-%[2]d"
  cognitive_account_project_id = azurerm_cognitive_account_project.test.id
  category                     = "AzureStorageAccount"
  target                       = azurerm_storage_account.test.primary_blob_endpoint

  metadata = {
    ApiType    = "Azure"
    ResourceId = azurerm_storage_account.test.id
    Location   = azurerm_storage_account.test.location
  }
}
`, r.basic(data), data.RandomInteger, data.RandomString)
}
