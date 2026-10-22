---
subcategory: "HorizonDB"
layout: "azurerm"
page_title: "Azure Resource Manager: azurerm_horizondb_pool_parameter_group_association"
description: |-
  Lists HorizonDB Pool Parameter Group Association resources.
---

# List resource: azurerm_horizondb_pool_parameter_group_association

Lists HorizonDB Pool Parameter Group Association resources.

Only associations with customer-created parameter groups are returned. Pools using their default parameter group are excluded.

## Example Usage

```hcl
list "azurerm_horizondb_pool_parameter_group_association" "example" {
  provider = azurerm
  config {
    horizondb_cluster_id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/resourceGroup1/providers/Microsoft.HorizonDb/clusters/cluster1"
  }
}
```

## Argument Reference

This list resource supports the following arguments:

* `horizondb_cluster_id` - (Required) The ID of the HorizonDB Cluster whose Pool associations should be returned.
