---
subcategory: "HorizonDB"
layout: "azurerm"
page_title: "Azure Resource Manager: azurerm_horizondb_parameter_group"
description: |-
  Lists HorizonDB Parameter Group resources.
---

# List resource: azurerm_horizondb_parameter_group

Lists HorizonDB Parameter Group resources.

## Example Usage

```hcl
list "azurerm_horizondb_parameter_group" "example" {
  provider = azurerm
  config {
    resource_group_name = "example-resources"
  }
}
```

## Argument Reference

This list resource supports the following arguments:

* `resource_group_name` - (Optional) The name of the Resource Group to query. When omitted, Parameter Groups across the subscription are returned.

* `subscription_id` - (Optional) The Subscription ID to query. Defaults to the subscription configured on the provider.
