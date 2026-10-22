---
subcategory: "HorizonDB"
layout: "azurerm"
page_title: "Azure Resource Manager: azurerm_horizondb_pool_parameter_group_association"
description: |-
  Manages a HorizonDB Pool Parameter Group Association.
---

# azurerm_horizondb_pool_parameter_group_association

Manages a HorizonDB Pool Parameter Group Association.

~> **Note:** This resource changes the parameter-group association of an existing Pool. It does not create or delete the Pool. Manage only one association per Pool and do not manage the same association through another resource or deployment.

## Example Usage

```hcl
resource "azurerm_resource_group" "example" {
  name     = "example-resources"
  location = "East US 2"
}

resource "azurerm_horizondb_cluster" "example" {
  name                = "example-horizondb-cluster"
  resource_group_name = azurerm_resource_group.example.name
  location            = azurerm_resource_group.example.location
  postgresql_version  = "18"

  pool {
    name                   = "example-pool"
    node_count             = 1
    high_availability_mode = "Disabled"

    compute {
      type       = "Provisioned"
      processor  = "AMD"
      generation = 1
      vcores     = 4
    }
  }
}

resource "azurerm_horizondb_parameter_group" "example" {
  name                = "example-horizondb-parameter-group"
  resource_group_name = azurerm_resource_group.example.name
  location            = azurerm_resource_group.example.location
  postgresql_version  = "18"
  parameters = {
    application_name = "example-application"
  }
}

resource "azurerm_horizondb_pool_parameter_group_association" "example" {
  horizondb_pool_id            = azurerm_horizondb_cluster.example.pool[0].id
  horizondb_parameter_group_id = azurerm_horizondb_parameter_group.example.id
  apply_method                 = "PendingReboot"
}
```

-> **Note:** The Cluster's `pool` block creates the Pool with its default parameter group. This association then attaches the customer-created group. Updating Pool compute through the Cluster resource does not reset the association.

## Arguments Reference

The following arguments are supported:

* `horizondb_pool_id` - (Required) The ID of the existing HorizonDB Pool. Changing this forces a new resource to be created.

* `horizondb_parameter_group_id` - (Required) The ID of the customer-created HorizonDB Parameter Group to associate with the Pool.

~> **Note:** The group must be compatible with the Pool's region and PostgreSQL version. System-managed default parameter groups cannot be specified here. New Pools must initially use their default group before this resource attaches a customer-created group.

* `apply_method` - (Required) How dynamic parameter changes become effective. Possible values are `Immediate` and `PendingReboot`.

~> **Note:** `Immediate` applies dynamic changes without a restart. `PendingReboot` defers dynamic changes until a customer-initiated Pool Restart. Static changes always require that Restart, regardless of this setting. Stop/Start and scheduled maintenance do not apply pending changes. This resource never restarts Pools automatically.

## Attributes Reference

In addition to the Arguments listed above - the following Attributes are exported:

* `id` - The ID of the associated HorizonDB Pool, which also identifies this singleton association.

* `sync_status` - The synchronization status reported by Azure for the Pool's parameter-group association.

## Timeouts

The `timeouts` block allows you to specify [timeouts](https://developer.hashicorp.com/terraform/language/resources/configure#define-operation-timeouts) for certain actions:

* `create` - (Defaults to 60 minutes) Used when creating the HorizonDB Pool Parameter Group Association.
* `read` - (Defaults to 5 minutes) Used when retrieving the HorizonDB Pool Parameter Group Association.
* `update` - (Defaults to 60 minutes) Used when updating the HorizonDB Pool Parameter Group Association.
* `delete` - (Defaults to 60 minutes) Used when deleting the HorizonDB Pool Parameter Group Association.

~> **Note:** Destroying this resource switches the Pool to the corresponding default parameter group for its region and PostgreSQL version, using the configured `apply_method`. Pending effective changes may still require a manual Restart. The Pool and customer-created group are not deleted. A remote ownership change detected during deletion causes an error rather than detaching a different association.

## Import

A HorizonDB Pool Parameter Group Association can be imported using the associated Pool's `resource id`, e.g.

```shell
terraform import azurerm_horizondb_pool_parameter_group_association.example /subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/resourceGroup1/providers/Microsoft.HorizonDb/clusters/cluster1/pools/pool1
```

-> **Note:** The Pool must be associated with a customer-created parameter group. An association to a system-managed default group is not an instance of this resource.

## API Providers
<!-- This section is generated, changes will be overwritten -->
This resource uses the following Azure API Providers:

* `Microsoft.HorizonDb` - 2026-10-01-preview
