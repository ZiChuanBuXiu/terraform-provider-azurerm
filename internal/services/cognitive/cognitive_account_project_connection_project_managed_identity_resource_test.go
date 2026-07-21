// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package cognitive_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-sdk/resource-manager/cognitive/2026-03-01/projectconnectionresource"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance/check"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

type CognitiveAccountProjectConnectionProjectManagedIdentityResource struct{}

func TestAccCognitiveAccountProjectConnectionProjectManagedIdentity_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_cognitive_account_project_connection_project_managed_identity", "test")
	r := CognitiveAccountProjectConnectionProjectManagedIdentityResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		data.ImportStep("metadata"),
	})
}

func TestAccCognitiveAccountProjectConnectionProjectManagedIdentity_requiresImport(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_cognitive_account_project_connection_project_managed_identity", "test")
	r := CognitiveAccountProjectConnectionProjectManagedIdentityResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		data.RequiresImportErrorStep(r.requiresImport),
	})
}

func TestAccCognitiveAccountProjectConnectionProjectManagedIdentity_complete(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_cognitive_account_project_connection_project_managed_identity", "test")
	r := CognitiveAccountProjectConnectionProjectManagedIdentityResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.complete(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		data.ImportStep("metadata"),
	})
}

func TestAccCognitiveAccountProjectConnectionProjectManagedIdentity_update(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_cognitive_account_project_connection_project_managed_identity", "test")
	r := CognitiveAccountProjectConnectionProjectManagedIdentityResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		data.ImportStep("metadata"),
		{
			Config: r.updated(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		data.ImportStep("metadata"),
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		data.ImportStep("metadata"),
	})
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) Exists(ctx context.Context, clients *clients.Client, state *pluginsdk.InstanceState) (*bool, error) {
	id, err := projectconnectionresource.ParseProjectConnectionID(state.ID)
	if err != nil {
		return nil, err
	}

	resp, err := clients.Cognitive.ProjectConnectionResourceClient.ProjectConnectionsGet(ctx, *id)
	if err != nil {
		return nil, fmt.Errorf("retrieving %s: %+v", *id, err)
	}

	return pointer.To(resp.Model != nil), nil
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) template(data acceptance.TestData) string {
	return fmt.Sprintf(`
resource "azurerm_resource_group" "test" {
  name     = "acctestRG-cognitive-pmi-%[1]d"
  location = "%[2]s"
}

resource "azurerm_cognitive_account" "test" {
  name                       = "acctest-cogacc-%[1]d"
  location                   = azurerm_resource_group.test.location
  resource_group_name        = azurerm_resource_group.test.name
  kind                       = "AIServices"
  sku_name                   = "S0"
  project_management_enabled = true
  custom_subdomain_name      = "acctestaiservices-%[1]d"

  identity {
    type = "SystemAssigned"
  }
}

resource "azurerm_cognitive_account_project" "test" {
  name                 = "acctest-proj-%[1]d"
  cognitive_account_id = azurerm_cognitive_account.test.id
  location             = azurerm_resource_group.test.location

  identity {
    type = "SystemAssigned"
  }
}
`, data.RandomInteger, data.Locations.Primary)
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) basic(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurerm" {
  features {}
}

%[1]s

resource "azurerm_cognitive_account" "target" {
  name                  = "acctest-target-%[2]d"
  location              = azurerm_resource_group.test.location
  resource_group_name   = azurerm_resource_group.test.name
  kind                  = "AIServices"
  sku_name              = "S0"
  custom_subdomain_name = "acctesttarget%[2]d"
}

resource "azurerm_cognitive_account_project_connection_project_managed_identity" "test" {
  name                         = "acctest-conn-%[2]d"
  cognitive_account_project_id = azurerm_cognitive_account_project.test.id
  category                     = "AIServices"
  target                       = azurerm_cognitive_account.target.endpoint

  metadata = {
    ApiType    = "Azure"
    ResourceId = azurerm_cognitive_account.target.id
    Location   = azurerm_cognitive_account.target.location
  }
}
`, r.template(data), data.RandomInteger)
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) requiresImport(data acceptance.TestData) string {
	return fmt.Sprintf(`
%[1]s

resource "azurerm_cognitive_account_project_connection_project_managed_identity" "import" {
  name                         = azurerm_cognitive_account_project_connection_project_managed_identity.test.name
  cognitive_account_project_id = azurerm_cognitive_account_project_connection_project_managed_identity.test.cognitive_account_project_id
  category                     = azurerm_cognitive_account_project_connection_project_managed_identity.test.category
  target                       = azurerm_cognitive_account_project_connection_project_managed_identity.test.target

  metadata = {
    ApiType    = azurerm_cognitive_account_project_connection_project_managed_identity.test.metadata.ApiType
    ResourceId = azurerm_cognitive_account_project_connection_project_managed_identity.test.metadata.ResourceId
    Location   = azurerm_cognitive_account_project_connection_project_managed_identity.test.metadata.Location
  }
}
`, r.basic(data))
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) overwritePrerequisites(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurerm" {
  features {
    skip_import_check_on_create_and_allow_overwriting_existing_resources = true
  }
}

%[1]s

resource "azurerm_cognitive_account" "target" {
  name                  = "acctest-target-%[2]d"
  location              = azurerm_resource_group.test.location
  resource_group_name   = azurerm_resource_group.test.name
  kind                  = "AIServices"
  sku_name              = "S0"
  custom_subdomain_name = "acctesttarget%[2]d"
}
`, r.template(data), data.RandomInteger)
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) overwriteExisting(data acceptance.TestData) string {
	return fmt.Sprintf(`
%[1]s

resource "azurerm_cognitive_account_project_connection_project_managed_identity" "test" {
  name                         = "acctest-conn-%[2]d"
  cognitive_account_project_id = azurerm_cognitive_account_project.test.id
  category                     = "AIServices"
  target                       = azurerm_cognitive_account.target.endpoint

  metadata = {
    ApiType    = "Azure"
    ResourceId = azurerm_cognitive_account.target.id
    Location   = azurerm_cognitive_account.target.location
  }
}
`, r.overwritePrerequisites(data), data.RandomInteger)
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) complete(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurerm" {
  features {}
}

%[1]s

resource "azurerm_cognitive_account" "target" {
  name                = "acctest-openai-%[2]d"
  location            = azurerm_resource_group.test.location
  resource_group_name = azurerm_resource_group.test.name
  kind                = "OpenAI"
  sku_name            = "S0"
}

resource "azurerm_cognitive_account_project_connection_project_managed_identity" "test" {
  name                         = "acctest-conn-%[2]d"
  cognitive_account_project_id = azurerm_cognitive_account_project.test.id
  category                     = "AzureOpenAI"
  target                       = azurerm_cognitive_account.target.endpoint

  metadata = {
    ApiType    = "Azure"
    ResourceId = azurerm_cognitive_account.target.id
    Location   = azurerm_cognitive_account.target.location
  }
}
`, r.template(data), data.RandomInteger)
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) updated(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurerm" {
  features {}
}

%[1]s

resource "azurerm_cognitive_account" "target2" {
  name                  = "acctest-target2-%[2]d"
  location              = azurerm_resource_group.test.location
  resource_group_name   = azurerm_resource_group.test.name
  kind                  = "AIServices"
  sku_name              = "S0"
  custom_subdomain_name = "acctesttarget2%[2]d"
}

resource "azurerm_cognitive_account_project_connection_project_managed_identity" "test" {
  name                         = "acctest-conn-%[2]d"
  cognitive_account_project_id = azurerm_cognitive_account_project.test.id
  category                     = "AIServices"
  target                       = azurerm_cognitive_account.target2.endpoint

  metadata = {
    ApiType    = "Azure"
    ResourceId = azurerm_cognitive_account.target2.id
    Location   = azurerm_cognitive_account.target2.location
  }
}
`, r.template(data), data.RandomInteger)
}
