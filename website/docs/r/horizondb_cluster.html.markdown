---
subcategory: "HorizonDB"
layout: "azurerm"
page_title: "Azure Resource Manager: azurerm_horizondb_cluster"
description: |-
  Manages a HorizonDB Cluster.
---

# azurerm_horizondb_cluster

Manages a HorizonDB Cluster.

-> **Note:** The optional `pool` block provisions compute after the Cluster is created. Without this block, a new Cluster has no compute Pool and cannot serve database traffic. This API version supports at most one Pool per Cluster.

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
      processor  = "AMD"
      generation = 1
      vcores     = 4
    }
  }

  tags = {
    environment = "example"
  }
}
```

## Arguments Reference

The following arguments are supported:

* `name` - (Required) The name of the HorizonDB Cluster. Changing this forces a new resource to be created.

~> **Note:** The name must contain between 1 and 63 lowercase letters, numbers, or hyphens and must start and end with a letter or number.

* `resource_group_name` - (Required) The name of the Resource Group where the HorizonDB Cluster should exist. Changing this forces a new resource to be created.

* `location` - (Required) The Azure Region where the HorizonDB Cluster should exist. Changing this forces a new resource to be created.

* `ai_model_management_enabled` - (Optional) Whether `ai model management` is enabled. Defaults to `false`.

-> **Note:** This setting is always applied in a separate update after the Cluster has been created, including when it defaults to `false`. Removing this argument from configuration resets it to `false` on the next apply.

* `backup_retention_days` - (Optional) The backup retention period in days.

~> **Note:** Possible values range between `1` and `35`. When omitted during creation, Azure uses its default of `7` days. Removing this argument from an existing configuration preserves the current retention period rather than resetting it.

* `create_mode` - (Optional) The creation mode of the HorizonDB Cluster. Possible values are `Create` and `PointInTimeRestore`. Defaults to `Create`. Changing this forces a new resource to be created.

* `customer_managed_key` - (Optional) A `customer_managed_key` block as defined below.

~> **Note:** Adding or removing this block forces a new resource to be created. Updating a key or encryption identity within the block does not force replacement. Customer-managed encryption requires an `identity` block and cannot be used when `geo_redundant_backup_enabled` is `true`.

* `geo_redundant_backup_enabled` - (Optional) Is geo-redundant backup enabled? Defaults to `false`. Changing this forces a new resource to be created.

* `identity` - (Optional) An `identity` block as defined below. Changing this forces a new resource to be created.

* `pool` - (Optional) A `pool` block as defined below.

~> **Note:** This resource owns the Cluster's singleton Pool. Refresh and import discover an existing Pool, even if it was created outside Terraform. Omitting the block from configuration plans its deletion. Do not manage the same Pool's compute configuration through another resource or deployment.

~> **Note:** Removing the `pool` block deletes the Pool, its Pool-scoped Endpoint, and that Endpoint's firewall rules, without deleting the Cluster. Because it is the only Pool, existing Cluster private endpoint connections are rejected. Customer-owned private endpoints are not deleted. Remove dependent firewall resources before removing the Pool.

-> **Note:** New Pools use the default parameter group for the Cluster's region and PostgreSQL version with `Immediate` application. Compute updates preserve the Pool's parameter-group association, and this resource does not automatically restart Pools.

* `postgresql_version` - (Optional) The PostgreSQL major version. Possible values are `17` and `18`. Changing this forces a new resource to be created.

~> **Note:** `postgresql_version` is required when `create_mode` is `Create` and must be omitted when `create_mode` is `PointInTimeRestore`. For restored Clusters, the version is obtained from the source backup and exported by this resource.

* `public_network_access_enabled` - (Optional) Is public network access enabled? Defaults to `false`.

* `restore` - (Optional) A `restore` block as defined below. Changing this forces a new resource to be created.

~> **Note:** This block is required when `create_mode` is `PointInTimeRestore` and must be omitted when `create_mode` is `Create`.

* `tags` - (Optional) A mapping of tags to assign to the resource.

---

A `compute` block supports the following:

* `generation` - (Required) The compute generation.

~> **Note:** The generation must be between `1` and `2147483647`. Azure validates availability for the selected region and processor.

* `processor` - (Required) The processor type. Possible values are `AMD` and `Intel`.

* `vcores` - (Required) The provisioned compute capacity in vCores.

~> **Note:** The number of vCores must be between `2` and `192`. Azure validates the supported combinations of processor, generation, and capacity.

---

A `customer_managed_key` block supports the following:

* `key_vault_key_id` - (Required) The versionless URI of the Key Vault Key used to encrypt the Cluster.

~> **Note:** The URI must use HTTPS and have the form `https://example.vault.azure.net/keys/example-key`, without a key version. Azure Government and Azure China Key Vault endpoints are also supported.

* `user_assigned_identity_id` - (Required) The ID of the User Assigned Identity used to access the Key Vault Key.

~> **Note:** This identity must be included in `identity.identity_ids` and have permission to access the key. When changing both the key and encryption identity in one apply, the provider rotates the key first and waits for completion before rotating the identity. The existing identity must be able to access the new key during that transition.

---

An `identity` block supports the following:

* `type` - (Required) The type of Managed Identity assigned to the Cluster. The only possible value is `UserAssigned`.

* `identity_ids` - (Required) A set of User Assigned Identity IDs assigned to the Cluster.

~> **Note:** Changing the set of assigned identities forces a new resource to be created. To rotate the encryption identity without replacing the Cluster, assign both the current and future encryption identities when creating the Cluster.

---

A `pool` block supports the following:

* `compute` - (Required) A `compute` block as defined above.

* `high_availability_mode` - (Required) The high-availability mode. Possible values are `AnyZone`, `Disabled`, and `ZoneRedundant`.

~> **Note:** Use `Disabled` with one node, or `AnyZone` or `ZoneRedundant` with two or more nodes. Change the mode and node count together when crossing these ranges. `ZoneRedundant` requires allocation across availability zones; changing the mode can replace nodes and disconnect their clients.

* `name` - (Required) The name of the Pool.

~> **Note:** The name must contain between 1 and 63 lowercase letters, numbers, or hyphens, must start and end with a letter or number, and must differ from the Cluster name. In-place renaming is rejected. Apply removal of the `pool` block before adding a Pool with a different name; this does not require replacement of the Cluster.

* `node_count` - (Required) The number of nodes allocated to the Pool.

~> **Note:** The node count must be between `1` and `16`. Zero nodes is not a supported configuration; stopping compute is a separate Azure operation.

---

A `restore` block supports the following:

* `point_in_time_utc` - (Required) The restore timestamp in RFC3339 format.

~> **Note:** For a same-region restore, HorizonDB uses the latest available backup at or before this timestamp. For a cross-region restore, the timestamp is required but ignored by the service, which uses the latest available geo-replicated backup.

* `source_horizondb_cluster_id` - (Required) The ID of the source HorizonDB Cluster.

* `source_location` - (Optional) The Azure Region of the source HorizonDB Cluster.

~> **Note:** This is required for cross-region restores and optional for same-region restores. Cross-region restores require an eligible geo-replicated backup from the source Cluster.

-> **Note:** Restoring a Cluster does not clone the source Pool or its child resources. If `pool` is configured, a new Pool is created after restoration using the restored PostgreSQL version to select its default parameter group.

## Attributes Reference

In addition to the Arguments listed above - the following Attributes are exported:

* `id` - The ID of the HorizonDB Cluster.

* `earliest_restore_point_utc` - The earliest available restore point in UTC.

* `pool` - A `pool` block as defined below.

---

A `pool` block exports the following:

* `id` - The ID of the HorizonDB Pool.

## Timeouts

The `timeouts` block allows you to specify [timeouts](https://developer.hashicorp.com/terraform/language/resources/configure#define-operation-timeouts) for certain actions:

* `create` - (Defaults to 1 hour) Used when creating the HorizonDB Cluster.
* `read` - (Defaults to 5 minutes) Used when retrieving the HorizonDB Cluster.
* `update` - (Defaults to 1 hour) Used when updating the HorizonDB Cluster.
* `delete` - (Defaults to 1 hour) Used when deleting the HorizonDB Cluster.

## Import

A HorizonDB Cluster can be imported using the `resource id`, e.g.

```shell
terraform import azurerm_horizondb_cluster.example /subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/resourceGroup1/providers/Microsoft.HorizonDb/clusters/cluster1
```

~> **Note:** HorizonDB does not return the original `point_in_time_utc` for cross-region restores. Import cannot recover that value. When managing an imported cross-region restore, specify the original restore configuration and use `lifecycle.ignore_changes` for `restore[0].point_in_time_utc` to avoid replacement caused solely by the missing timestamp.

## API Providers
<!-- This section is generated, changes will be overwritten -->
This resource uses the following Azure API Providers:

* `Microsoft.HorizonDb` - 2026-10-01-preview
