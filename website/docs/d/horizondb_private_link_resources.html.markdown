---
subcategory: "HorizonDB"
layout: "azurerm"
page_title: "Azure Resource Manager: Data Source: azurerm_horizondb_private_link_resources"
description: |-
  Gets information about existing HorizonDB Private Link Resources.
---

# Data Source: azurerm_horizondb_private_link_resources

Gets information about existing HorizonDB Private Link Resources.

## Example Usage

```hcl
variable "horizondb_cluster_id" {
  type        = string
  description = "The ID of an existing HorizonDB Cluster."
}

data "azurerm_horizondb_private_link_resources" "example" {
  horizondb_cluster_id = var.horizondb_cluster_id
}
```

## Arguments Reference

The following arguments are supported:

* `horizondb_cluster_id` - (Required) The ID of the HorizonDB Cluster.

## Attributes Reference

In addition to the Arguments listed above - the following Attributes are exported:

* `id` - The ID of the HorizonDB Cluster.

* `private_link_resources` - A list of `private_link_resources` blocks as defined below.

---

A `private_link_resources` block exports the following:

* `id` - The ID of the Private Link Resource.

* `group_id` - The group identifier of the Private Link Resource.

* `required_members` - The member names required by the Private Link Resource.

* `required_zone_names` - The private DNS zones required by the Private Link Resource.

## Timeouts

The `timeouts` block allows you to specify [timeouts](https://developer.hashicorp.com/terraform/language/resources/configure#define-operation-timeouts) for certain actions:

* `read` - (Defaults to 5 minutes) Used when retrieving the HorizonDB Private Link Resources.
