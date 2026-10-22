---
subcategory: "HorizonDB"
layout: "azurerm"
page_title: "Azure Resource Manager: azurerm_horizondb_parameter_group"
description: |-
  Manages a HorizonDB Parameter Group.
---

# azurerm_horizondb_parameter_group

Manages a HorizonDB Parameter Group.

## Example Usage

```hcl
resource "azurerm_resource_group" "example" {
  name     = "example-resources"
  location = "East US 2"
}

resource "azurerm_horizondb_parameter_group" "example" {
  name                = "example-horizondb-parameter-group"
  resource_group_name = azurerm_resource_group.example.name
  location            = azurerm_resource_group.example.location
  postgresql_version  = "18"
  description         = "Application parameter overrides"

  parameters = {
    application_name = "example-application"
    xmlbinary        = "base64"
  }

  tags = {
    environment = "example"
  }
}
```

## Arguments Reference

The following arguments are supported:

* `name` - (Required) The name of the HorizonDB Parameter Group. Changing this forces a new resource to be created.

~> **Note:** The name must contain between 2 and 63 letters, numbers, or hyphens and must start and end with a letter or number.

* `resource_group_name` - (Required) The name of the Resource Group where the HorizonDB Parameter Group should exist. Changing this forces a new resource to be created.

* `location` - (Required) The Azure Region where the HorizonDB Parameter Group should exist. Changing this forces a new resource to be created.

* `postgresql_version` - (Required) The PostgreSQL major version. Possible values are `17` and `18`. Changing this forces a new resource to be created.

* `description` - (Optional) A description of the HorizonDB Parameter Group.

~> **Note:** The description can contain up to 1024 characters.

* `parameters` - (Optional) A map of parameter names to string values containing the user overrides managed by this resource.

~> **Note:** New groups inherit parameters that are not explicitly configured from the region's default group for the selected PostgreSQL version. Existing overrides can be added or changed, but removing an override is currently unsupported and causes an error before the update is applied. Retain existing keys in this map. Setting a value equal to its default is still an explicit override, not a reset to inheritance.

-> **Note:** Azure validates the available parameters, allowed values, and read-only restrictions. Terraform state includes all user overrides returned by Azure, including imported or externally added overrides, but excludes inherited defaults. Empty strings are permitted for parameters that accept them.

* `tags` - (Optional) A mapping of tags to assign to the resource.

## Attributes Reference

In addition to the Arguments listed above - the following Attributes are exported:

* `id` - The ID of the HorizonDB Parameter Group.

## Timeouts

The `timeouts` block allows you to specify [timeouts](https://developer.hashicorp.com/terraform/language/resources/configure#define-operation-timeouts) for certain actions:

* `create` - (Defaults to 60 minutes) Used when creating the HorizonDB Parameter Group.
* `read` - (Defaults to 5 minutes) Used when retrieving the HorizonDB Parameter Group.
* `update` - (Defaults to 60 minutes) Used when updating the HorizonDB Parameter Group.
* `delete` - (Defaults to 60 minutes) Used when deleting the HorizonDB Parameter Group.

~> **Note:** Azure rejects deletion while Pools are connected. Remove their [parameter-group associations](horizondb_pool_parameter_group_association.html) first. To replace a connected group without reverting its Pools to defaults, create a differently named group compatible with those Pools, update their associations to use it, and then delete the old group. A destroy-before-create replacement of a connected group will fail. This resource does not automatically detach Pools or restart them. Dynamic parameter application follows each Pool's apply method; static changes require a customer-initiated Pool Restart. Stop/Start and scheduled maintenance do not apply pending changes.

## Import

A HorizonDB Parameter Group can be imported using the `resource id`, e.g.

```shell
terraform import azurerm_horizondb_parameter_group.example /subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/resourceGroup1/providers/Microsoft.HorizonDb/parameterGroups/parameterGroup1
```

## API Providers
<!-- This section is generated, changes will be overwritten -->
This resource uses the following Azure API Providers:

* `Microsoft.HorizonDb` - 2026-10-01-preview
