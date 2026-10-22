---
subcategory: "HorizonDB"
layout: "azurerm"
page_title: "Azure Resource Manager: Data Source: azurerm_horizondb_parameter_group_connections"
description: |-
  Gets information about existing HorizonDB Parameter Group Connections.
---

# Data Source: azurerm_horizondb_parameter_group_connections

Gets information about existing HorizonDB Parameter Group Connections.

## Example Usage

```hcl
variable "horizondb_parameter_group_id" {
  type        = string
  description = "The ID of an existing customer-created HorizonDB Parameter Group."
}

data "azurerm_horizondb_parameter_group_connections" "example" {
  horizondb_parameter_group_id = var.horizondb_parameter_group_id
}
```

## Arguments Reference

The following arguments are supported:

* `horizondb_parameter_group_id` - (Required) The ID of the customer-created HorizonDB Parameter Group.

## Attributes Reference

In addition to the Arguments listed above - the following Attributes are exported:

* `id` - The ID of the HorizonDB Parameter Group.

* `horizondb_pool_ids` - A set of connected HorizonDB Pool IDs. This set is empty when no Pools are connected.

## Timeouts

The `timeouts` block allows you to specify [timeouts](https://developer.hashicorp.com/terraform/language/resources/configure#define-operation-timeouts) for certain actions:

* `read` - (Defaults to 5 minutes) Used when retrieving the HorizonDB Parameter Group Connections.
