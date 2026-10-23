// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/identity"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/clusters"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/pools"
	azureclient "github.com/hashicorp/go-azure-sdk/sdk/client"
	"github.com/hashicorp/go-azure-sdk/sdk/environments"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/features"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	horizondbclient "github.com/hashicorp/terraform-provider-azurerm/internal/services/horizondb/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const horizondbTestIdentityId = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/example-rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/example-identity"

func horizondbClusterResourceConfig(t *testing.T, values map[string]interface{}) *terraform.ResourceConfig {
	t.Helper()
	wrapped := sdk.WrappedResource(HorizondbClusterResource{})
	data := schema.TestResourceDataRaw(t, wrapped.Schema, values)
	raw, err := data.State().AttrsAsObjectValue(wrapped.CoreConfigSchema().ImpliedType())
	require.NoError(t, err)
	fields := raw.AsValueMap()
	for name, field := range fields {
		value, configured := values[name]
		if !configured || value == nil {
			fields[name] = cty.NullVal(field.Type())
			continue
		}
		switch value := value.(type) {
		case string:
			fields[name] = cty.StringVal(value)
		case bool:
			fields[name] = cty.BoolVal(value)
		case int:
			fields[name] = cty.NumberIntVal(int64(value))
		case int64:
			fields[name] = cty.NumberIntVal(value)
		default:
			fields[name] = field
		}
	}
	config := terraform.NewResourceConfigRaw(values)
	config.CtyValue = cty.ObjectVal(fields)
	return config
}

func TestHorizondbClusterSchema(t *testing.T) {
	resource := sdk.WrappedResource(HorizondbClusterResource{})
	require.NoError(t, resource.InternalValidate(nil, true))
	require.NotNil(t, resource.Identity)
	require.True(t, resource.Schema["ai_model_management_enabled"].Optional)
	require.False(t, resource.Schema["ai_model_management_enabled"].Computed)
	require.Equal(t, false, resource.Schema["ai_model_management_enabled"].Default)
	require.True(t, resource.Schema["backup_retention_days"].Optional)
	require.True(t, resource.Schema["backup_retention_days"].Computed)
	require.Nil(t, resource.Schema["backup_retention_days"].Default)
	require.True(t, resource.Schema["postgresql_version"].Optional)
	require.True(t, resource.Schema["postgresql_version"].Computed)
	require.True(t, resource.Schema["postgresql_version"].ForceNew)
	require.True(t, resource.Schema["create_mode"].Optional)
	require.True(t, resource.Schema["create_mode"].ForceNew)
	require.Equal(t, "Create", resource.Schema["create_mode"].Default)
	for _, value := range []string{"Create", "PointInTimeRestore"} {
		_, errs := resource.Schema["create_mode"].ValidateFunc(value, "create_mode")
		require.Empty(t, errs)
	}
	for _, value := range []string{"", "Default", "Replica", "create"} {
		_, errs := resource.Schema["create_mode"].ValidateFunc(value, "create_mode")
		require.NotEmpty(t, errs)
	}

	for _, name := range []string{"a", "a-1", strings.Repeat("a", 63)} {
		_, errs := resource.Schema["name"].ValidateFunc(name, "name")
		require.Empty(t, errs)
	}
	for _, name := range []string{"", "-a", "a-", "Upper", "a_b", strings.Repeat("a", 64)} {
		_, errs := resource.Schema["name"].ValidateFunc(name, "name")
		require.NotEmpty(t, errs)
	}
	for _, value := range []int{1, 7, 35} {
		_, errs := resource.Schema["backup_retention_days"].ValidateFunc(value, "backup_retention_days")
		require.Empty(t, errs)
	}
	for _, value := range []int{0, 36} {
		_, errs := resource.Schema["backup_retention_days"].ValidateFunc(value, "backup_retention_days")
		require.NotEmpty(t, errs)
	}
	keySchema := resource.Schema["customer_managed_key"].Elem.(*schema.Resource).Schema["key_vault_key_id"]
	for _, uri := range []string{
		"https://example.vault.azure.net/keys/example",
		"https://example.vault.usgovcloudapi.net/keys/example",
		"https://example.vault.azure.cn/keys/example/",
	} {
		_, errs := keySchema.ValidateFunc(uri, "key_vault_key_id")
		require.Empty(t, errs)
	}
	for _, uri := range []string{
		"http://example.vault.azure.net/keys/example",
		"https://example.vault.azure.net/keys/example/version",
		"https://example.vault.azure.net/secrets/example",
		"https://example.com/keys/example",
	} {
		_, errs := keySchema.ValidateFunc(uri, "key_vault_key_id")
		require.NotEmpty(t, errs)
	}
}

func TestConfiguredHorizondbBackupRetention(t *testing.T) {
	for _, test := range []struct {
		name      string
		config    cty.Value
		expected  *int64
		errorText string
	}{
		{name: "omitted", config: horizondbClusterResourceConfig(t, map[string]interface{}{}).CtyValue},
		{name: "explicit", config: horizondbClusterResourceConfig(t, map[string]interface{}{"backup_retention_days": 14}).CtyValue, expected: pointer.To(int64(14))},
		{name: "unknown", config: cty.UnknownVal(sdk.WrappedResource(HorizondbClusterResource{}).CoreConfigSchema().ImpliedType()), errorText: "cannot determine"},
	} {
		t.Run(test.name, func(t *testing.T) {
			actual, err := configuredHorizondbBackupRetention(test.config, 14)
			if test.errorText != "" {
				require.ErrorContains(t, err, test.errorText)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.expected, actual)
		})
	}
}

func TestHorizondbClusterBackupRetentionSerialization(t *testing.T) {
	state := HorizondbClusterResourceModel{
		Location:                  "eastus2",
		PostgresqlVersion:         "18",
		CreateMode:                string(clusters.ClusterCreateModeCreate),
		GeoRedundantBackupEnabled: false,
	}
	for _, test := range []struct {
		name      string
		retention *int64
		contains  string
	}{
		{name: "omitted", contains: `"geoRedundancy":"Disabled"`},
		{name: "explicit service default", retention: pointer.To(int64(7)), contains: `"retentionDays":7`},
		{name: "explicit minimum", retention: pointer.To(int64(1)), contains: `"retentionDays":1`},
		{name: "explicit maximum", retention: pointer.To(int64(35)), contains: `"retentionDays":35`},
	} {
		t.Run(test.name, func(t *testing.T) {
			payload, err := expandHorizondbCluster(state, test.retention)
			require.NoError(t, err)
			serialized, err := json.Marshal(payload)
			require.NoError(t, err)
			if test.retention == nil {
				require.NotContains(t, string(serialized), `"retentionDays"`)
			}
			require.Contains(t, string(serialized), test.contains)
		})
	}
}

func TestHorizondbClusterEncryptionDiff(t *testing.T) {
	resource := sdk.WrappedResource(HorizondbClusterResource{})
	config := func(key string) map[string]interface{} {
		values := map[string]interface{}{
			"name": "example", "resource_group_name": "example-rg", "location": "eastus2",
			"postgresql_version": "18",
			"identity": []interface{}{map[string]interface{}{
				"type": "UserAssigned", "identity_ids": []interface{}{horizondbTestIdentityId},
			}},
		}
		if key != "" {
			values["customer_managed_key"] = []interface{}{map[string]interface{}{
				"key_vault_key_id":          "https://example.vault.azure.net/keys/" + key,
				"user_assigned_identity_id": horizondbTestIdentityId,
			}}
		}
		return values
	}
	for _, test := range []struct {
		name, oldKey, newKey string
		replacement          bool
	}{
		{name: "enable CMK", newKey: "example", replacement: true},
		{name: "disable CMK", oldKey: "example", replacement: true},
		{name: "rotate CMK", oldKey: "example", newKey: "rotated"},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := schema.TestResourceDataRaw(t, resource.Schema, config(test.oldKey))
			data.SetId(clusters.NewClusterID("00000000-0000-0000-0000-000000000000", "example-rg", "example").ID())
			diff, err := resource.Diff(t.Context(), data.State(), terraform.NewResourceConfigRaw(config(test.newKey)), &clients.Client{})
			require.NoError(t, err)
			require.NotNil(t, diff)
			require.Equal(t, test.replacement, diff.RequiresNew())
		})
	}
}

func TestHorizondbClusterCreateModeDiff(t *testing.T) {
	wrapped := sdk.WrappedResource(HorizondbClusterResource{})
	id := clusters.NewClusterID(horizondbTestSubscription, "example-rg", "example")
	config := func(mode string) map[string]interface{} {
		values := map[string]interface{}{
			"name": id.ClusterName, "resource_group_name": id.ResourceGroupName, "location": "eastus2",
		}
		if mode != "" {
			values["create_mode"] = mode
		}
		if mode == "PointInTimeRestore" {
			values["restore"] = []interface{}{map[string]interface{}{
				"source_horizondb_cluster_id": clusters.NewClusterID(id.SubscriptionId, id.ResourceGroupName, "source").ID(),
				"point_in_time_utc":           "2026-10-01T00:00:00Z",
			}}
		} else {
			values["postgresql_version"] = "18"
		}
		return values
	}
	for _, test := range []struct {
		name, before, after string
		replacement         bool
	}{
		{name: "default", before: "Create"},
		{name: "explicit create", after: "Create"},
		{name: "switch to restore", before: "Create", after: "PointInTimeRestore", replacement: true},
		{name: "restore with computed version", before: "PointInTimeRestore", after: "PointInTimeRestore"},
		{name: "switch to create", before: "PointInTimeRestore", after: "Create", replacement: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := schema.TestResourceDataRaw(t, wrapped.Schema, config(test.before))
			data.SetId(id.ID())
			require.NoError(t, data.Set("postgresql_version", "18"))
			after := config(test.after)
			configData := schema.TestResourceDataRaw(t, wrapped.Schema, after)
			configData.SetId(id.ID())
			raw, err := configData.State().AttrsAsObjectValue(wrapped.CoreConfigSchema().ImpliedType())
			require.NoError(t, err)
			values := raw.AsValueMap()
			if _, configured := after["postgresql_version"]; !configured {
				values["postgresql_version"] = cty.NullVal(cty.String)
			}
			if _, configured := after["create_mode"]; !configured {
				values["create_mode"] = cty.NullVal(cty.String)
			}
			prior := data.State()
			prior.RawConfig = cty.ObjectVal(values)
			diff, err := wrapped.Diff(t.Context(), prior, terraform.NewResourceConfigRaw(after), &clients.Client{})
			require.NoError(t, err)
			require.Equal(t, test.replacement, diff.RequiresNew())
			if !test.replacement {
				require.Nil(t, diff)
			}
		})
	}
}

func TestHorizondbClusterConfig(t *testing.T) {
	restoreType := cty.List(cty.Object(map[string]cty.Type{
		"source_horizondb_cluster_id": cty.String,
		"source_location":             cty.String,
		"point_in_time_utc":           cty.String,
	}))
	cmkType := cty.List(cty.Object(map[string]cty.Type{"user_assigned_identity_id": cty.String}))
	identityType := cty.List(cty.Object(map[string]cty.Type{"identity_ids": cty.Set(cty.String)}))
	cmk := cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{"user_assigned_identity_id": cty.StringVal(horizondbTestIdentityId)})})
	assigned := cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
		"identity_ids": cty.SetVal([]cty.Value{cty.StringVal(strings.ToUpper(horizondbTestIdentityId))}),
	})})
	restore := cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
		"source_horizondb_cluster_id": cty.StringVal("source"),
		"source_location":             cty.StringVal("eastus2"),
		"point_in_time_utc":           cty.StringVal("2026-10-01T00:00:00Z"),
	})})
	tests := []struct {
		name   string
		values map[string]cty.Value
		error  string
	}{
		{name: "ordinary creation"},
		{name: "omitted create mode", values: map[string]cty.Value{"create_mode": cty.NullVal(cty.String)}},
		{name: "missing version", values: map[string]cty.Value{"postgresql_version": cty.NullVal(cty.String)}, error: "`postgresql_version` is required"},
		{name: "unknown version", values: map[string]cty.Value{"postgresql_version": cty.UnknownVal(cty.String)}},
		{name: "unknown create mode", values: map[string]cty.Value{"create_mode": cty.UnknownVal(cty.String), "postgresql_version": cty.NullVal(cty.String)}},
		{name: "restore", values: map[string]cty.Value{"create_mode": cty.StringVal("PointInTimeRestore"), "restore": restore, "postgresql_version": cty.NullVal(cty.String)}},
		{name: "restore with version", values: map[string]cty.Value{"create_mode": cty.StringVal("PointInTimeRestore"), "restore": restore}, error: "`postgresql_version` must be omitted"},
		{name: "restore with unknown version", values: map[string]cty.Value{"create_mode": cty.StringVal("PointInTimeRestore"), "restore": restore, "postgresql_version": cty.UnknownVal(cty.String)}, error: "`postgresql_version` must be omitted"},
		{name: "restore in create mode", values: map[string]cty.Value{"restore": restore}, error: "`restore` can only be specified"},
		{name: "restore in omitted create mode", values: map[string]cty.Value{"create_mode": cty.NullVal(cty.String), "restore": restore}, error: "`restore` can only be specified"},
		{name: "missing restore block", values: map[string]cty.Value{"create_mode": cty.StringVal("PointInTimeRestore"), "postgresql_version": cty.NullVal(cty.String)}, error: "`restore` is required"},
		{name: "empty restore block", values: map[string]cty.Value{"create_mode": cty.StringVal("PointInTimeRestore"), "restore": cty.ListValEmpty(restoreType.ElementType()), "postgresql_version": cty.NullVal(cty.String)}, error: "`restore` is required"},
		{name: "unknown restore", values: map[string]cty.Value{"create_mode": cty.StringVal("PointInTimeRestore"), "restore": cty.UnknownVal(restoreType), "postgresql_version": cty.NullVal(cty.String)}},
		{name: "unknown restore in create mode", values: map[string]cty.Value{"restore": cty.UnknownVal(restoreType)}},
		{name: "customer managed encryption", values: map[string]cty.Value{"customer_managed_key": cmk, "identity": assigned}},
		{name: "missing identity", values: map[string]cty.Value{"customer_managed_key": cmk}, error: "`identity` is required"},
		{name: "geo backup with encryption", values: map[string]cty.Value{"customer_managed_key": cmk, "identity": assigned, "geo_redundant_backup_enabled": cty.True}, error: "`geo_redundant_backup_enabled` cannot be true"},
		{name: "unknown cmk", values: map[string]cty.Value{"customer_managed_key": cty.UnknownVal(cmkType)}},
		{name: "unknown identity block", values: map[string]cty.Value{"customer_managed_key": cmk, "identity": cty.UnknownVal(identityType)}},
		{name: "unknown identity set", values: map[string]cty.Value{"customer_managed_key": cmk, "identity": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{"identity_ids": cty.UnknownVal(cty.Set(cty.String))})})}},
		{name: "unassigned encryption identity", values: map[string]cty.Value{"customer_managed_key": cmk, "identity": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{"identity_ids": cty.SetVal([]cty.Value{cty.StringVal("other")})})})}, error: "must be included"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := map[string]cty.Value{
				"create_mode":                  cty.StringVal("Create"),
				"postgresql_version":           cty.StringVal("18"),
				"restore":                      cty.NullVal(restoreType),
				"customer_managed_key":         cty.NullVal(cmkType),
				"identity":                     cty.NullVal(identityType),
				"geo_redundant_backup_enabled": cty.False,
			}
			for name, value := range test.values {
				config[name] = value
			}
			err := validateHorizondbClusterConfig(cty.ObjectVal(config))
			if test.error != "" {
				require.ErrorContains(t, err, test.error)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHorizondbClusterExpand(t *testing.T) {
	state := HorizondbClusterResourceModel{
		CreateMode:               "Create",
		Location:                 "East US 2",
		PostgresqlVersion:        "18",
		BackupRetentionDays:      7,
		AiModelManagementEnabled: true,
	}
	payload, err := expandHorizondbCluster(state, pointer.To(state.BackupRetentionDays))
	require.NoError(t, err)
	require.Equal(t, "eastus2", payload.Location)
	require.Equal(t, "18", string(pointer.From(payload.Properties.PostgreSqlVersion)))
	require.Equal(t, clusters.ClusterCreateModeCreate, pointer.From(payload.Properties.CreationMode))
	require.Nil(t, payload.Properties.PointInTimeUtc)
	require.Nil(t, payload.Properties.AiModelManagement, "AI model management is PATCH-only")
	require.Nil(t, payload.Identity)
	require.Equal(t, clusters.GeoRedundancyStateDisabled, payload.Properties.Backup.GeoRedundancy)
	require.Equal(t, clusters.PublicNetworkAccessStateDisabled, pointer.From(payload.Properties.Network.PublicAccess))

	state.CreateMode = "PointInTimeRestore"
	state.Restore = []HorizondbClusterRestore{{
		SourceHorizondbClusterId: clusters.NewClusterID("00000000-0000-0000-0000-000000000000", "source-rg", "source").ID(),
		SourceLocation:           "West US 2",
		PointInTimeUtc:           "2026-10-01T00:00:00Z",
	}}
	state.Identity = []identity.ModelUserAssigned{{Type: identity.TypeUserAssigned, IdentityIds: []string{horizondbTestIdentityId}}}
	state.CustomerManagedKey = []HorizondbClusterCustomerManagedKey{{KeyVaultKeyId: "https://example.vault.azure.net/keys/example", UserAssignedIdentityId: horizondbTestIdentityId}}
	payload, err = expandHorizondbCluster(state, pointer.To(state.BackupRetentionDays))
	require.NoError(t, err)
	require.Nil(t, payload.Properties.PostgreSqlVersion, "restore must not send the version, including a computed version")
	require.Equal(t, clusters.ClusterCreateModePointInTimeRestore, pointer.From(payload.Properties.CreationMode))
	require.Equal(t, "westus2", pointer.From(payload.Properties.CreationSourceClusterLocation))
	require.Equal(t, state.Restore[0].PointInTimeUtc, pointer.From(payload.Properties.PointInTimeUtc))
	require.Equal(t, clusters.HorizonDbClusterDataEncryptionTypeAzureKeyVault, pointer.From(payload.Properties.DataEncryption.Type))
	require.Contains(t, payload.Identity.IdentityIds, horizondbTestIdentityId)

	state.CreateMode = "Create"
	_, err = expandHorizondbCluster(state, nil)
	require.ErrorContains(t, err, "`restore` can only be specified")
	state.Restore = nil
	state.PostgresqlVersion = ""
	_, err = expandHorizondbCluster(state, nil)
	require.ErrorContains(t, err, "`postgresql_version` is required")
	state.CreateMode = "PointInTimeRestore"
	_, err = expandHorizondbCluster(state, nil)
	require.ErrorContains(t, err, "`restore` is required")
	state.CreateMode = "Replica"
	_, err = expandHorizondbCluster(state, nil)
	require.ErrorContains(t, err, "unsupported `create_mode`")
}

func TestHorizondbClusterUpdates(t *testing.T) {
	state := HorizondbClusterResourceModel{
		BackupRetentionDays: 14,
		Tags:                map[string]string{},
		CustomerManagedKey: []HorizondbClusterCustomerManagedKey{{
			KeyVaultKeyId:          "https://example.vault.azure.net/keys/rotated",
			UserAssignedIdentityId: horizondbTestIdentityId,
		}},
	}
	for _, test := range []struct {
		name                        string
		keyChanged, identityChanged bool
		count                       int
	}{
		{name: "key only", keyChanged: true, count: 1},
		{name: "identity only", identityChanged: true, count: 1},
		{name: "both rotations", keyChanged: true, identityChanged: true, count: 2},
		{name: "other properties", count: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			updates := expandHorizondbClusterUpdates(state, func(field string) bool {
				switch field {
				case "customer_managed_key.0.key_vault_key_id":
					return test.keyChanged
				case "customer_managed_key.0.user_assigned_identity_id":
					return test.identityChanged
				default:
					return true
				}
			}, pointer.To(state.BackupRetentionDays))
			require.Len(t, updates, test.count)
			first := updates[0]
			require.Equal(t, int64(14), pointer.From(first.Properties.Backup.RetentionDays))
			require.Equal(t, clusters.PublicNetworkAccessStateDisabled, pointer.From(first.Properties.Network.PublicAccess))
			require.Equal(t, clusters.AiModelManagementStatusDisabled, pointer.From(first.Properties.AiModelManagement))
			require.NotNil(t, first.Tags, "tag removal must not be omitted")
			for _, update := range updates {
				if encryption := update.Properties.DataEncryption; encryption != nil {
					require.False(t, encryption.PrimaryKeyUri != nil && encryption.PrimaryUserAssignedIdentityId != nil, "rotations must be in separate requests")
				}
				body, err := json.Marshal(update)
				require.NoError(t, err)
				for _, field := range []string{"postgreSqlVersion", "creationMode", "geoRedundancy", "readWritePool", "provisioningState"} {
					require.NotContains(t, string(body), field, "PATCH must not resubmit create-only or read-only fields")
				}
			}
		})
	}
}

func TestHorizondbClusterFlatten(t *testing.T) {
	resource := HorizondbClusterResource{}
	metadata := sdk.NewResourceMetaData(nil, resource)
	id := clusters.NewClusterID("00000000-0000-0000-0000-000000000000", "example-rg", "example")
	metadata.ResourceData.SetId(id.ID())
	state := HorizondbClusterResourceModel{Restore: []HorizondbClusterRestore{{PointInTimeUtc: "2026-10-01T00:00:00Z"}}}
	require.NoError(t, metadata.Encode(&state))
	sourceId := clusters.NewClusterID(id.SubscriptionId, "source-rg", "source").ID()
	model := clusters.Cluster{
		Location: "East US 2",
		Properties: &clusters.ClusterProperties{
			PostgreSqlVersion:               pointer.To(clusters.PostgreSqlVersion("18")),
			CreationMode:                    pointer.To(clusters.ClusterCreateModePointInTimeRestore),
			CreationSourceClusterResourceId: pointer.To(strings.ReplaceAll(sourceId, "resourceGroups", "resourcegroups")),
			CreationSourceClusterLocation:   pointer.To("West US 2"),
			Backup:                          &clusters.BackupProperties{RetentionDays: pointer.To(int64(7)), GeoRedundancy: clusters.GeoRedundancyStateDisabled},
			DataEncryption: &clusters.HorizonDbClusterDataEncryption{
				Type:                          pointer.To(clusters.HorizonDbClusterDataEncryptionTypeAzureKeyVault),
				PrimaryKeyUri:                 pointer.To("https://example.vault.azure.net/keys/example"),
				PrimaryUserAssignedIdentityId: pointer.To(strings.ReplaceAll(horizondbTestIdentityId, "resourceGroups", "resourcegroups")),
			},
			ReadWritePool: clusters.ReadWritePoolProperties{DesignatedPoolResourceId: pointer.To(id.ID() + "/pools/example-pool")},
		},
	}
	require.NoError(t, resource.flatten(metadata, &id, &model, nil))
	require.NoError(t, metadata.Decode(&state))
	require.Equal(t, "eastus2", state.Location)
	require.Equal(t, "PointInTimeRestore", state.CreateMode)
	require.Equal(t, sourceId, state.Restore[0].SourceHorizondbClusterId)
	require.Equal(t, "2026-10-01T00:00:00Z", state.Restore[0].PointInTimeUtc)
	require.Equal(t, horizondbTestIdentityId, state.CustomerManagedKey[0].UserAssignedIdentityId)
	require.Empty(t, state.Pool)
	model.Properties.ReadWritePool.DesignatedPoolResourceId = nil
	model.Properties.DataEncryption = nil
	model.Properties.CreationMode = pointer.To(clusters.ClusterCreateModeCreate)
	require.NoError(t, resource.flatten(metadata, &id, &model, nil))
	require.NoError(t, metadata.Decode(&state))
	require.Empty(t, state.Restore)
	require.Equal(t, "Create", state.CreateMode)
	require.Empty(t, state.CustomerManagedKey)
	require.Empty(t, state.Pool)
	require.Error(t, resource.flatten(metadata, &id, nil, nil))
	model.Properties.Backup = nil
	model.Properties.CreationMode = nil
	require.NoError(t, resource.flatten(metadata, &id, &model, nil))
	require.NoError(t, metadata.Decode(&state))
	require.Equal(t, int64(7), state.BackupRetentionDays)
	require.Equal(t, "Create", state.CreateMode)
	model.Properties.DataEncryption = &clusters.HorizonDbClusterDataEncryption{
		Type: pointer.To(clusters.HorizonDbClusterDataEncryptionTypeAzureKeyVault),
	}
	require.ErrorContains(t, resource.flatten(metadata, &id, &model, nil), "key URI is missing")
	model.Properties.DataEncryption.Type = pointer.To(clusters.HorizonDbClusterDataEncryptionType("Other"))
	require.ErrorContains(t, resource.flatten(metadata, &id, &model, nil), "unsupported encryption type")
}

func TestHorizondbClusterCreate(t *testing.T) {
	id := clusters.NewClusterID("00000000-0000-0000-0000-000000000000", "example-rg", "example")
	for _, test := range []struct {
		name                string
		overwrite           bool
		getStatus           int
		gets, puts, patches int32
		ai                  *bool
		patchStatus         int
		error               string
	}{
		{name: "requires import", getStatus: http.StatusOK, gets: 1, error: "needs to be imported"},
		{name: "create defaults AI disabled", getStatus: http.StatusNotFound, gets: 1, puts: 1, patches: 1, patchStatus: http.StatusOK},
		{name: "overwrite defaults AI disabled", overwrite: true, puts: 1, patches: 1, patchStatus: http.StatusOK},
		{name: "presence check failure", getStatus: http.StatusForbidden, gets: 1, error: "checking for presence"},
		{name: "explicit AI disabled", getStatus: http.StatusNotFound, gets: 1, puts: 1, patches: 1, ai: pointer.To(false), patchStatus: http.StatusOK},
		{name: "explicit AI enabled", getStatus: http.StatusNotFound, gets: 1, puts: 1, patches: 1, ai: pointer.To(true), patchStatus: http.StatusOK},
		{name: "AI failure retains identity", getStatus: http.StatusNotFound, gets: 1, puts: 1, patches: 1, ai: pointer.To(true), patchStatus: http.StatusBadRequest, error: "configuring AI model management"},
		{name: "default AI failure retains identity", getStatus: http.StatusNotFound, gets: 1, puts: 1, patches: 1, patchStatus: http.StatusBadRequest, error: "configuring AI model management"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var gets, puts, patches, creationPolls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.URL.Path == id.ID()+"/pools" {
					w.Header().Set("Content-Type", "application/json")
					assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{"value": []interface{}{}}))
					return
				}
				assert.Equal(t, id.ID(), request.URL.Path)
				assert.Equal(t, "2026-10-01-preview", request.URL.Query().Get("api-version"))
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set(azureclient.SkipPollingDelayHeader, "true")
				switch request.Method {
				case http.MethodGet:
					if puts.Load() == 0 {
						gets.Add(1)
						w.WriteHeader(test.getStatus)
					} else if patches.Load() == 0 {
						creationPolls.Add(1)
					}
				case http.MethodPut:
					puts.Add(1)
					var payload clusters.Cluster
					if !assert.NoError(t, json.NewDecoder(request.Body).Decode(&payload)) || !assert.NotNil(t, payload.Properties) {
						return
					}
					assert.Equal(t, "18", string(pointer.From(payload.Properties.PostgreSqlVersion)))
					assert.Nil(t, payload.Properties.AiModelManagement)
					w.WriteHeader(http.StatusOK)
				case http.MethodPatch:
					assert.Positive(t, creationPolls.Load(), "Cluster creation must finish polling before the AI PATCH")
					patches.Add(1)
					var payload clusters.ClusterForPatchUpdate
					if !assert.NoError(t, json.NewDecoder(request.Body).Decode(&payload)) || !assert.NotNil(t, payload.Properties) {
						return
					}
					assert.Equal(t, horizondbAiModelManagement(pointer.From(test.ai)), pointer.From(payload.Properties.AiModelManagement))
					w.WriteHeader(test.patchStatus)
				default:
					t.Errorf("unexpected method %s", request.Method)
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
				assert.NoError(t, json.NewEncoder(w).Encode(clusters.Cluster{
					Location:   "eastus2",
					Properties: &clusters.ClusterProperties{ProvisioningState: clusters.ProvisioningStateSucceeded},
				}))
			}))
			defer server.Close()
			client, err := clusters.NewClustersClientWithBaseURI(environments.ResourceManagerAPI(server.URL))
			require.NoError(t, err)
			client.Client.AuthorizeRequest = nil
			client.Client.DisableRetries = true
			poolClient, err := pools.NewPoolsClientWithBaseURI(environments.ResourceManagerAPI(server.URL))
			require.NoError(t, err)
			poolClient.Client.AuthorizeRequest = nil
			poolClient.Client.DisableRetries = true
			metadata := sdk.NewResourceMetaData(&clients.Client{
				Account:   &clients.ResourceManagerAccount{SubscriptionId: id.SubscriptionId},
				Features:  features.UserFeatures{SkipImportCheckOnCreateAndAllowOverwritingExistingResources: test.overwrite},
				HorizonDB: &horizondbclient.Client{ClustersClient: client, PoolsClient: poolClient},
			}, HorizondbClusterResource{})
			values := map[string]interface{}{
				"name": id.ClusterName, "resource_group_name": id.ResourceGroupName, "location": "eastus2",
				"postgresql_version": "18", "backup_retention_days": 7,
				"create_mode": (HorizondbClusterResource{}).Arguments()["create_mode"].Default,
			}
			if test.ai != nil {
				values["ai_model_management_enabled"] = *test.ai
			}
			wrapped := sdk.WrappedResource(HorizondbClusterResource{})
			config := horizondbClusterResourceConfig(t, values)
			diff, err := wrapped.Diff(t.Context(), nil, config, metadata.Client)
			require.NoError(t, err)
			diff.RawConfig = config.CtyValue
			data, err := schema.InternalMap(wrapped.Schema).Data(nil, diff)
			require.NoError(t, err)
			data.SetId("pending")
			state := data.State()
			state.ID = ""
			state.RawConfig = config.CtyValue
			metadata.ResourceData = wrapped.Data(state)
			metadata.ResourceData.SetId("")
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			err = (HorizondbClusterResource{}).Create().Func(ctx, metadata)
			if test.error != "" {
				require.ErrorContains(t, err, test.error)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.gets, gets.Load())
			require.Equal(t, test.puts, puts.Load())
			require.Equal(t, test.patches, patches.Load())
			if test.puts == 0 {
				require.Empty(t, metadata.ResourceData.Id())
			} else {
				require.Equal(t, id.ID(), metadata.ResourceData.Id())
				identityData, err := metadata.ResourceData.Identity()
				require.NoError(t, err)
				require.Equal(t, id.ClusterName, identityData.Get("name"))
				require.Equal(t, id.ResourceGroupName, identityData.Get("resource_group_name"))
				require.Equal(t, id.SubscriptionId, identityData.Get("subscription_id"))
			}
		})
	}
}

func TestHorizondbClusterAIRemovalDefaultsToDisabled(t *testing.T) {
	id := clusters.NewClusterID(horizondbTestSubscription, "example-rg", "example")
	var patches atomic.Int32
	client := newHorizondbTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, id.ID(), r.URL.Path)
		switch r.Method {
		case http.MethodPatch:
			patches.Add(1)
			var body map[string]interface{}
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, map[string]interface{}{
				"properties": map[string]interface{}{"aiModelManagement": "Disabled"},
			}, body)
		case http.MethodGet:
		default:
			t.Errorf("unexpected method %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		assert.NoError(t, json.NewEncoder(w).Encode(clusters.Cluster{
			Location: "eastus2",
			Properties: &clusters.ClusterProperties{
				ProvisioningState: clusters.ProvisioningStateSucceeded,
				AiModelManagement: pointer.To(clusters.AiModelManagementStatusDisabled),
			},
		}))
	})
	wrapped := sdk.WrappedResource(HorizondbClusterResource{})
	before := map[string]interface{}{
		"name": id.ClusterName, "resource_group_name": id.ResourceGroupName, "location": "eastus2",
		"postgresql_version": "18", "ai_model_management_enabled": true,
	}
	data := schema.TestResourceDataRaw(t, wrapped.Schema, before)
	data.SetId(id.ID())
	after := map[string]interface{}{
		"name": id.ClusterName, "resource_group_name": id.ResourceGroupName, "location": "eastus2",
		"postgresql_version": "18",
	}
	diff, err := wrapped.Diff(t.Context(), data.State(), terraform.NewResourceConfigRaw(after), client)
	require.NoError(t, err)
	require.False(t, diff.RequiresNew())
	updated, err := schema.InternalMap(wrapped.Schema).Data(data.State(), diff)
	require.NoError(t, err)
	require.True(t, updated.HasChange("ai_model_management_enabled"))
	old, new := updated.GetChange("ai_model_management_enabled")
	require.Equal(t, true, old)
	require.Equal(t, false, new)
	metadata := sdk.NewResourceMetaData(client, HorizondbClusterResource{})
	metadata.ResourceData = updated
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	require.NoError(t, (HorizondbClusterResource{}).Update().Func(ctx, metadata))
	require.Equal(t, int32(1), patches.Load())
}

func TestHorizondbClusterUpdateRequests(t *testing.T) {
	for _, failFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("first_request_fails_%t", failFirst), func(t *testing.T) {
			var requests, polls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set(azureclient.SkipPollingDelayHeader, "true")
				if request.Method == http.MethodGet {
					polls.Add(1)
					assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
						"properties": map[string]string{"provisioningState": "Succeeded"},
					}))
					return
				}
				assert.Equal(t, http.MethodPatch, request.Method)
				var payload clusters.ClusterForPatchUpdate
				if !assert.NoError(t, json.NewDecoder(request.Body).Decode(&payload)) || !assert.NotNil(t, payload.Properties) || !assert.NotNil(t, payload.Properties.DataEncryption) {
					return
				}
				index := requests.Add(1)
				assert.Equal(t, index-1, polls.Load(), "each rotation must finish polling before the next request")
				if index == 1 {
					assert.NotNil(t, payload.Properties.DataEncryption.PrimaryKeyUri)
					assert.Nil(t, payload.Properties.DataEncryption.PrimaryUserAssignedIdentityId)
					assert.Equal(t, map[string]string{"environment": "updated"}, pointer.From(payload.Tags))
				} else {
					assert.Nil(t, payload.Properties.DataEncryption.PrimaryKeyUri)
					assert.Equal(t, horizondbTestIdentityId, pointer.From(payload.Properties.DataEncryption.PrimaryUserAssignedIdentityId))
				}
				if failFirst {
					w.WriteHeader(http.StatusBadRequest)
				}
				assert.NoError(t, json.NewEncoder(w).Encode(clusters.Cluster{Location: "eastus2"}))
			}))
			defer server.Close()
			client, err := clusters.NewClustersClientWithBaseURI(environments.ResourceManagerAPI(server.URL))
			require.NoError(t, err)
			client.Client.AuthorizeRequest = nil
			client.Client.DisableRetries = true
			providerClient := &clients.Client{HorizonDB: &horizondbclient.Client{ClustersClient: client}}
			metadata := sdk.NewResourceMetaData(providerClient, HorizondbClusterResource{})
			wrapped := sdk.WrappedResource(HorizondbClusterResource{})
			before := map[string]interface{}{
				"name": "example", "resource_group_name": "example-rg", "location": "eastus2",
				"postgresql_version": "18", "backup_retention_days": 7,
				"tags": map[string]interface{}{"environment": "old"},
				"customer_managed_key": []interface{}{map[string]interface{}{
					"key_vault_key_id":          "https://example.vault.azure.net/keys/original",
					"user_assigned_identity_id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/example-rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/original",
				}},
			}
			data := schema.TestResourceDataRaw(t, wrapped.Schema, before)
			data.SetId(clusters.NewClusterID("00000000-0000-0000-0000-000000000000", "example-rg", "example").ID())
			after := map[string]interface{}{
				"name": "example", "resource_group_name": "example-rg", "location": "eastus2",
				"postgresql_version": "18", "backup_retention_days": 14,
				"tags": map[string]interface{}{"environment": "updated"},
				"customer_managed_key": []interface{}{map[string]interface{}{
					"key_vault_key_id":          "https://example.vault.azure.net/keys/rotated",
					"user_assigned_identity_id": horizondbTestIdentityId,
				}},
			}
			prior := data.State()
			config := horizondbClusterResourceConfig(t, after)
			prior.RawConfig = config.CtyValue
			diff, err := schema.InternalMap(wrapped.Schema).Diff(t.Context(), prior, config, nil, providerClient, true)
			require.NoError(t, err)
			diff.RawConfig = config.CtyValue
			metadata.ResourceData, err = schema.InternalMap(wrapped.Schema).Data(prior, diff)
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			err = (HorizondbClusterResource{}).Update().Func(ctx, metadata)
			if failFirst {
				require.ErrorContains(t, err, "updating")
				require.Equal(t, int32(1), requests.Load())
			} else {
				require.NoError(t, err)
				require.Equal(t, int32(2), requests.Load())
			}
		})
	}
}
