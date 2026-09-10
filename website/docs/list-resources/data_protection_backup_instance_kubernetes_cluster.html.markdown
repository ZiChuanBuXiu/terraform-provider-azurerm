---
subcategory: "DataProtection"
layout: "azurerm"
page_title: "Azure Resource Manager: azurerm_data_protection_backup_instance_kubernetes_cluster"
description: |-
  Lists Data Protection Backup Instance resources for Kubernetes Clusters in a Backup Vault.
---

# List resource: azurerm_data_protection_backup_instance_kubernetes_cluster

Lists Data Protection Backup Instance resources for Kubernetes Clusters in a Backup Vault.

## Example Usage

### List Backup Instances for Kubernetes Clusters in a Backup Vault

```hcl
data "azurerm_data_protection_backup_vault" "example" {
  name                = "existing-data-protection-backup-vault"
  resource_group_name = "existing-resource-group"
}

list "azurerm_data_protection_backup_instance_kubernetes_cluster" "example" {
  provider = azurerm
  config {
    vault_id = data.azurerm_data_protection_backup_vault.example.id
  }
}
```

## Argument Reference

This list resource supports the following arguments:

* `vault_id` - (Required) The ID of the Data Protection Backup Vault to query.
