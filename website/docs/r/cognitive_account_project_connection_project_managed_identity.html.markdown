---
subcategory: "Cognitive Services"
layout: "azurerm"
page_title: "Azure Resource Manager: azurerm_cognitive_account_project_connection_project_managed_identity"
description: |-
  Manages a Cognitive Services (Microsoft Foundry) Account Project Connection with project managed identity authentication.
---

# azurerm_cognitive_account_project_connection_project_managed_identity

Manages a Cognitive Services (Microsoft Foundry) Account Project Connection with project managed identity authentication.

## Example Usage

```hcl
resource "azurerm_resource_group" "example" {
  name     = "example-resource-group"
  location = "West Europe"
}

resource "azurerm_cognitive_account" "example" {
  name                       = "example-cognitive-account"
  location                   = azurerm_resource_group.example.location
  resource_group_name        = azurerm_resource_group.example.name
  kind                       = "AIServices"
  sku_name                   = "S0"
  project_management_enabled = true
  custom_subdomain_name      = "examplecognitiveaccount"

  identity {
    type = "SystemAssigned"
  }
}

resource "azurerm_cognitive_account_project" "example" {
  name                 = "example-cognitive-account-project"
  cognitive_account_id = azurerm_cognitive_account.example.id
  location             = azurerm_resource_group.example.location

  identity {
    type = "SystemAssigned"
  }
}

resource "azurerm_cognitive_account" "target" {
  name                = "example-cognitive-account-target"
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name
  kind                = "OpenAI"
  sku_name            = "S0"
}

resource "azurerm_cognitive_account_project_connection_project_managed_identity" "example" {
  name                         = "example-cognitive-account-project"
  cognitive_account_project_id = azurerm_cognitive_account_project.example.id
  category                     = "AzureOpenAI"
  target                       = azurerm_cognitive_account.target.endpoint

  metadata = {
    ApiType    = "Azure"
    ResourceId = azurerm_cognitive_account.target.id
    Location   = azurerm_cognitive_account.target.location
  }
}
```

## Arguments Reference

The following arguments are supported:

* `name` - (Required) The name of the Cognitive Services Account Project Connection. Changing this forces a new resource to be created.

* `cognitive_account_project_id` - (Required) The ID of the Cognitive Services Account Project. Changing this forces a new resource to be created.

* `category` - (Required) The category of the connection. Possible values are `AzureOpenAI`, `AIServices`, `CognitiveService`, `CognitiveSearch`, `ApiManagement`, `RemoteTool`, `RemoteA2A`, `AzureContainerAppEnvironment`, `AzureStorageAccount`, `PlaywrightWorkspace`, and `OpenAPI`. Changing this forces a new resource to be created.

* `metadata` - (Required) A mapping of metadata key-value pairs for the connection.

* `target` - (Required) The target endpoint or resource for the connection.

## Attributes Reference

In addition to the Arguments listed above - the following Attributes are exported:

* `id` - The ID of the Cognitive Services (Microsoft Foundry) Account Project Connection with project managed identity authentication.

* `authentication_type` - The authentication type of the connection.

## Timeouts

The `timeouts` block allows you to specify [timeouts](https://developer.hashicorp.com/terraform/language/resources/configure#define-operation-timeouts) for certain actions:

* `create` - (Defaults to 30 minutes) Used when creating the Cognitive Services (Microsoft Foundry) Account Project Connection with project managed identity authentication.
* `read` - (Defaults to 5 minutes) Used when retrieving the Cognitive Services (Microsoft Foundry) Account Project Connection with project managed identity authentication.
* `update` - (Defaults to 30 minutes) Used when updating the Cognitive Services (Microsoft Foundry) Account Project Connection with project managed identity authentication.
* `delete` - (Defaults to 30 minutes) Used when deleting the Cognitive Services (Microsoft Foundry) Account Project Connection with project managed identity authentication.

## Import

A Cognitive Services (Microsoft Foundry) Account Project Connection with project managed identity authentication can be imported using the `resource id`, e.g.

```shell
terraform import azurerm_cognitive_account_project_connection_project_managed_identity.example /subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/resourceGroup1/providers/Microsoft.CognitiveServices/accounts/account1/projects/project1/connections/connection1
```

## API Providers
<!-- This section is generated, changes will be overwritten -->
This resource uses the following Azure API Providers:

* `Microsoft.CognitiveServices` - 2026-03-01
