---
subcategory: "HorizonDB"
layout: "azurerm"
page_title: "Azure Resource Manager: Data Source: azurerm_horizondb_default_parameter_group"
description: |-
  Gets information about an existing HorizonDB Default Parameter Group.
---

# Data Source: azurerm_horizondb_default_parameter_group

Gets information about an existing HorizonDB Default Parameter Group.

## Example Usage

```hcl
data "azurerm_horizondb_default_parameter_group" "example" {
  location           = "West Europe"
  postgresql_version = "17"
}
```

## Arguments Reference

The following arguments are supported:

* `location` - (Required) The Azure region containing the default parameter group.

* `postgresql_version` - (Required) The PostgreSQL major version of the default parameter group. Possible values are `17` and `18`.

## Attributes Reference

In addition to the Arguments listed above - the following Attributes are exported:

* `id` - The ID of the HorizonDB Default Parameter Group.

* `description` - The description of the default parameter group.

* `name` - The name of the default parameter group.

* `parameters` - A map of parameter names to their default values.

## Timeouts

The `timeouts` block allows you to specify [timeouts](https://developer.hashicorp.com/terraform/language/resources/configure#define-operation-timeouts) for certain actions:

* `read` - (Defaults to 5 minutes) Used when retrieving the HorizonDB Default Parameter Group.
