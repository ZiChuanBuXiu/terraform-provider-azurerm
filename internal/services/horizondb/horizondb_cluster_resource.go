// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonids"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/identity"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/location"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/resourceids"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/clusters"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/pools"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
)

//go:generate go run ../../tools/generator-tests resourceidentity
type HorizondbClusterResource struct{}

type HorizondbClusterResourceModel struct {
	Name                       string                               `tfschema:"name"`
	ResourceGroupName          string                               `tfschema:"resource_group_name"`
	Location                   string                               `tfschema:"location"`
	PostgresqlVersion          string                               `tfschema:"postgresql_version"`
	Pool                       []HorizondbClusterPool               `tfschema:"pool"`
	AiModelManagementEnabled   bool                                 `tfschema:"ai_model_management_enabled"`
	BackupRetentionDays        int64                                `tfschema:"backup_retention_days"`
	CreateMode                 string                               `tfschema:"create_mode"`
	CustomerManagedKey         []HorizondbClusterCustomerManagedKey `tfschema:"customer_managed_key"`
	GeoRedundantBackupEnabled  bool                                 `tfschema:"geo_redundant_backup_enabled"`
	Identity                   []identity.ModelUserAssigned         `tfschema:"identity"`
	PublicNetworkAccessEnabled bool                                 `tfschema:"public_network_access_enabled"`
	Restore                    []HorizondbClusterRestore            `tfschema:"restore"`
	Tags                       map[string]string                    `tfschema:"tags"`
	EarliestRestorePointUtc    string                               `tfschema:"earliest_restore_point_utc"`
}

type HorizondbClusterCustomerManagedKey struct {
	KeyVaultKeyId          string `tfschema:"key_vault_key_id"`
	UserAssignedIdentityId string `tfschema:"user_assigned_identity_id"`
}

type HorizondbClusterRestore struct {
	SourceHorizondbClusterId string `tfschema:"source_horizondb_cluster_id"`
	PointInTimeUtc           string `tfschema:"point_in_time_utc"`
	SourceLocation           string `tfschema:"source_location"`
}

var (
	_ sdk.ResourceWithUpdate        = HorizondbClusterResource{}
	_ sdk.ResourceWithIdentity      = HorizondbClusterResource{}
	_ sdk.ResourceWithCustomizeDiff = HorizondbClusterResource{}
)

func (HorizondbClusterResource) ModelObject() interface{} {
	return &HorizondbClusterResourceModel{}
}

func (HorizondbClusterResource) ResourceType() string {
	return "azurerm_horizondb_cluster"
}

func (HorizondbClusterResource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return clusters.ValidateClusterID
}

func (HorizondbClusterResource) Identity() resourceids.ResourceId {
	return &clusters.ClusterId{}
}

func (HorizondbClusterResource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ForceNew: true,
			ValidateFunc: validation.All(
				validation.StringLenBetween(1, 63),
				validation.StringMatch(regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`), "must contain only lowercase letters, numbers and hyphens and start and end with a letter or number"),
			),
		},
		"resource_group_name": commonschema.ResourceGroupName(),
		"location":            commonschema.Location(),
		"ai_model_management_enabled": {
			Type:     pluginsdk.TypeBool,
			Optional: true,
			Default:  false,
		},
		"backup_retention_days": {
			Type:         pluginsdk.TypeInt,
			Optional:     true,
			Computed:     true,
			ValidateFunc: validation.IntBetween(1, 35),
		},
		"create_mode": {
			Type:         pluginsdk.TypeString,
			Optional:     true,
			ForceNew:     true,
			Default:      string(clusters.ClusterCreateModeCreate),
			ValidateFunc: validation.StringInSlice(clusters.PossibleValuesForClusterCreateMode(), false),
		},
		"customer_managed_key": {
			Type:     pluginsdk.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					"key_vault_key_id": {
						Type:     pluginsdk.TypeString,
						Required: true,
						ValidateFunc: validation.StringMatch(
							regexp.MustCompile(`^https://[A-Za-z0-9-]+[.]vault([.]azure[.]net|[.]usgovcloudapi[.]net|[.]azure[.]cn)/keys/[A-Za-z0-9-]+/?$`),
							"must be a versionless HTTPS Azure Key Vault key URI",
						),
					},
					"user_assigned_identity_id": commonschema.ResourceIDReferenceRequired(&commonids.UserAssignedIdentityId{}),
				},
			},
		},
		"geo_redundant_backup_enabled": {
			Type:     pluginsdk.TypeBool,
			Optional: true,
			ForceNew: true,
			Default:  false,
		},
		// The SDK identity map cannot encode null entries for individual PATCH removals.
		"identity": commonschema.UserAssignedIdentityOptionalForceNew(),
		"postgresql_version": {
			Type:         pluginsdk.TypeString,
			Optional:     true,
			Computed:     true,
			ForceNew:     true,
			ValidateFunc: validation.StringInSlice(clusters.PossibleValuesForPostgreSqlVersion(), false),
		},
		"pool": horizondbClusterPoolSchema(),
		"public_network_access_enabled": {
			Type:     pluginsdk.TypeBool,
			Optional: true,
			Default:  false,
		},
		"restore": {
			Type:     pluginsdk.TypeList,
			Optional: true,
			ForceNew: true,
			MaxItems: 1,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					"point_in_time_utc": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ValidateFunc: validation.IsRFC3339Time,
					},
					"source_horizondb_cluster_id": commonschema.ResourceIDReferenceRequired(&clusters.ClusterId{}),
					"source_location": {
						Type:         pluginsdk.TypeString,
						Optional:     true,
						StateFunc:    location.StateFunc,
						ValidateFunc: validation.StringIsNotEmpty,
					},
				},
			},
		},
		"tags": commonschema.Tags(),
	}
}

func (HorizondbClusterResource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"earliest_restore_point_utc": {
			Type:     pluginsdk.TypeString,
			Computed: true,
		},
	}
}

func (HorizondbClusterResource) CustomizeDiff() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			diff := metadata.ResourceDiff
			if err := validateHorizondbClusterConfig(diff.GetRawConfig()); err != nil {
				return err
			}
			if err := validateHorizondbClusterPoolConfig(diff.GetRawConfig()); err != nil {
				return err
			}
			if diff.Id() != "" && diff.NewValueKnown("customer_managed_key") {
				old, new := diff.GetChange("customer_managed_key")
				if len(old.([]interface{})) != len(new.([]interface{})) {
					return diff.ForceNew("customer_managed_key")
				}
			}
			if diff.Id() != "" && !diff.HasChanges("name", "resource_group_name", "location", "create_mode", "postgresql_version", "restore", "identity", "geo_redundant_backup_enabled") && diff.NewValueKnown("pool.0.name") {
				old, new := diff.GetChange("pool.0.name")
				if err := validateHorizondbClusterPoolRename(old.(string), new.(string)); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func validateHorizondbClusterConfig(config cty.Value) error {
	if !config.IsKnown() || config.IsNull() {
		return nil
	}
	restore := config.GetAttr("restore")
	version := config.GetAttr("postgresql_version")
	createMode := config.GetAttr("create_mode")
	if createMode.IsKnown() {
		mode := string(clusters.ClusterCreateModeCreate)
		if !createMode.IsNull() {
			mode = createMode.AsString()
		}
		hasRestore := restore.IsKnown() && !restore.IsNull() && restore.LengthInt() > 0
		switch mode {
		case string(clusters.ClusterCreateModeCreate):
			if restore.IsKnown() && hasRestore {
				return errors.New("`restore` can only be specified when `create_mode` is `PointInTimeRestore`")
			}
			if version.IsNull() {
				return errors.New("`postgresql_version` is required when `create_mode` is `Create`")
			}
		case string(clusters.ClusterCreateModePointInTimeRestore):
			if restore.IsKnown() && !hasRestore {
				return errors.New("`restore` is required when `create_mode` is `PointInTimeRestore`")
			}
			if !version.IsNull() {
				return errors.New("`postgresql_version` must be omitted when `create_mode` is `PointInTimeRestore`")
			}
		}
	}
	cmk := config.GetAttr("customer_managed_key")
	if !cmk.IsKnown() || cmk.IsNull() || cmk.LengthInt() == 0 {
		return nil
	}
	geoBackup := config.GetAttr("geo_redundant_backup_enabled")
	if geoBackup.IsKnown() && !geoBackup.IsNull() && geoBackup.True() {
		return errors.New("`geo_redundant_backup_enabled` cannot be true with `customer_managed_key`")
	}
	identities := config.GetAttr("identity")
	if !identities.IsKnown() {
		return nil
	}
	if identities.IsNull() || identities.LengthInt() == 0 {
		return errors.New("`identity` is required with `customer_managed_key`")
	}
	key := cmk.AsValueSlice()[0]
	assigned := identities.AsValueSlice()[0]
	if !key.IsKnown() || key.IsNull() || !assigned.IsKnown() || assigned.IsNull() {
		return nil
	}
	keyIdentity := key.GetAttr("user_assigned_identity_id")
	identityIds := assigned.GetAttr("identity_ids")
	if !keyIdentity.IsKnown() || keyIdentity.IsNull() || !identityIds.IsWhollyKnown() {
		return nil
	}
	if !identityIds.IsNull() {
		for _, id := range identityIds.AsValueSlice() {
			if !id.IsNull() && strings.EqualFold(id.AsString(), keyIdentity.AsString()) {
				return nil
			}
		}
	}
	return errors.New("`customer_managed_key.user_assigned_identity_id` must be included in `identity.identity_ids`")
}

func (r HorizondbClusterResource) Create() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 60 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			var state HorizondbClusterResourceModel
			if err := metadata.Decode(&state); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}
			id := clusters.NewClusterID(metadata.Client.Account.SubscriptionId, state.ResourceGroupName, state.Name)
			client := metadata.Client.HorizonDB.ClustersClient
			if !metadata.Client.Features.SkipImportCheckOnCreateAndAllowOverwritingExistingResources {
				existing, err := client.Get(ctx, id)
				if err != nil && !response.WasNotFound(existing.HttpResponse) {
					return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
				}
				if !response.WasNotFound(existing.HttpResponse) {
					return metadata.ResourceRequiresImport(r.ResourceType(), id)
				}
			}
			retention, err := configuredHorizondbBackupRetention(metadata.ResourceData.GetRawConfig(), state.BackupRetentionDays)
			if err != nil {
				return err
			}
			payload, err := expandHorizondbCluster(state, retention)
			if err != nil {
				return err
			}
			if err := client.CreateOrReplaceCallbackThenPoll(ctx, id, payload, metadata.SetIDAndIdentityCallback(&id)); err != nil {
				return fmt.Errorf("creating %s: %+v", id, err)
			}
			metadata.SetID(id)
			if err := pluginsdk.SetResourceIdentityData(metadata.ResourceData, &id); err != nil {
				return err
			}
			aiSettings := clusters.ClusterForPatchUpdate{Properties: &clusters.ClusterPropertiesForPatchUpdate{
				AiModelManagement: pointer.To(horizondbAiModelManagement(state.AiModelManagementEnabled)),
			}}
			if err := client.UpdateThenPoll(ctx, id, aiSettings); err != nil {
				return fmt.Errorf("configuring AI model management for %s: %+v", id, err)
			}
			if len(state.Pool) > 0 || metadata.Client.Features.SkipImportCheckOnCreateAndAllowOverwritingExistingResources {
				return reconcileHorizondbClusterPool(ctx, metadata, id, state.Pool)
			}
			return nil
		},
	}
}

func (r HorizondbClusterResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			id, err := clusters.ParseClusterIDInsensitively(metadata.ResourceData.Id())
			if err != nil {
				return err
			}
			resp, err := metadata.Client.HorizonDB.ClustersClient.Get(ctx, *id)
			if err != nil {
				if response.WasNotFound(resp.HttpResponse) {
					return metadata.MarkAsGone(*id)
				}
				return fmt.Errorf("retrieving %s: %+v", id, err)
			}
			pool, err := readHorizondbClusterPool(ctx, metadata, *id)
			if err != nil {
				return err
			}
			return r.flatten(metadata, id, resp.Model, pool)
		},
	}
}

func (HorizondbClusterResource) Update() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 60 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			id, err := clusters.ParseClusterIDInsensitively(metadata.ResourceData.Id())
			if err != nil {
				return err
			}
			var state HorizondbClusterResourceModel
			if err := metadata.Decode(&state); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}
			oldName, newName := metadata.ResourceData.GetChange("pool.0.name")
			if err := validateHorizondbClusterPoolRename(oldName.(string), newName.(string)); err != nil {
				return err
			}
			retention, err := configuredHorizondbBackupRetention(metadata.ResourceData.GetRawConfig(), state.BackupRetentionDays)
			if err != nil {
				return err
			}
			for _, payload := range expandHorizondbClusterUpdates(state, metadata.ResourceData.HasChange, retention) {
				if err := metadata.Client.HorizonDB.ClustersClient.UpdateThenPoll(ctx, *id, payload); err != nil {
					return fmt.Errorf("updating %s: %+v", id, err)
				}
			}
			if metadata.ResourceData.HasChange("pool") {
				return reconcileHorizondbClusterPool(ctx, metadata, *id, state.Pool)
			}
			return nil
		},
	}
}

func (HorizondbClusterResource) Delete() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 60 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			id, err := clusters.ParseClusterIDInsensitively(metadata.ResourceData.Id())
			if err != nil {
				return err
			}
			if err := metadata.Client.HorizonDB.ClustersClient.DeleteThenPoll(ctx, *id); err != nil {
				return fmt.Errorf("deleting %s: %+v", id, err)
			}
			return nil
		},
	}
}

func configuredHorizondbBackupRetention(config cty.Value, retention int64) (*int64, error) {
	if config.IsNull() {
		return nil, nil
	}
	if !config.IsKnown() || !config.Type().IsObjectType() || !config.Type().HasAttribute("backup_retention_days") {
		return nil, errors.New("cannot determine whether `backup_retention_days` is configured")
	}
	value := config.GetAttr("backup_retention_days")
	if !value.IsKnown() {
		return nil, errors.New("`backup_retention_days` must be known when applying changes")
	}
	if value.IsNull() {
		return nil, nil
	}
	return pointer.To(retention), nil
}

func expandHorizondbCluster(state HorizondbClusterResourceModel, retention *int64) (clusters.Cluster, error) {
	geoRedundancy := clusters.GeoRedundancyStateDisabled
	if state.GeoRedundantBackupEnabled {
		geoRedundancy = clusters.GeoRedundancyStateEnabled
	}
	payload := clusters.Cluster{
		Location: location.Normalize(state.Location),
		Tags:     pointer.To(state.Tags),
		Properties: &clusters.ClusterProperties{
			CreationMode: pointer.To(clusters.ClusterCreateMode(state.CreateMode)),
			Network:      &clusters.Network{PublicAccess: pointer.To(horizondbPublicNetworkAccess(state.PublicNetworkAccessEnabled))},
			Backup: &clusters.BackupProperties{
				GeoRedundancy: geoRedundancy,
				RetentionDays: retention,
			},
		},
	}
	if len(state.Identity) > 0 {
		expanded, err := identity.ExpandUserAssignedMapFromModel(state.Identity)
		if err != nil {
			return payload, fmt.Errorf("expanding `identity`: %+v", err)
		}
		payload.Identity = expanded
	}
	if len(state.CustomerManagedKey) > 0 {
		key := state.CustomerManagedKey[0]
		payload.Properties.DataEncryption = &clusters.HorizonDbClusterDataEncryption{
			Type:                          pointer.To(clusters.HorizonDbClusterDataEncryptionTypeAzureKeyVault),
			PrimaryKeyUri:                 pointer.To(key.KeyVaultKeyId),
			PrimaryUserAssignedIdentityId: pointer.To(key.UserAssignedIdentityId),
		}
	}
	switch clusters.ClusterCreateMode(state.CreateMode) {
	case clusters.ClusterCreateModeCreate:
		if len(state.Restore) > 0 {
			return payload, errors.New("`restore` can only be specified when `create_mode` is `PointInTimeRestore`")
		}
		if state.PostgresqlVersion == "" {
			return payload, errors.New("`postgresql_version` is required when `create_mode` is `Create`")
		}
		payload.Properties.PostgreSqlVersion = pointer.To(clusters.PostgreSqlVersion(state.PostgresqlVersion))
	case clusters.ClusterCreateModePointInTimeRestore:
		if len(state.Restore) == 0 {
			return payload, errors.New("`restore` is required when `create_mode` is `PointInTimeRestore`")
		}
		restore := state.Restore[0]
		payload.Properties.CreationSourceClusterResourceId = pointer.To(restore.SourceHorizondbClusterId)
		payload.Properties.PointInTimeUtc = pointer.To(restore.PointInTimeUtc)
		if restore.SourceLocation != "" {
			payload.Properties.CreationSourceClusterLocation = pointer.To(location.Normalize(restore.SourceLocation))
		}
	default:
		return payload, fmt.Errorf("unsupported `create_mode` %q", state.CreateMode)
	}
	return payload, nil
}

func expandHorizondbClusterUpdates(state HorizondbClusterResourceModel, hasChange func(string) bool, retention *int64) []clusters.ClusterForPatchUpdate {
	payload := clusters.ClusterForPatchUpdate{Properties: &clusters.ClusterPropertiesForPatchUpdate{}}
	if hasChange("tags") {
		payload.Tags = pointer.To(state.Tags)
	}
	if retention != nil && hasChange("backup_retention_days") {
		payload.Properties.Backup = &clusters.BackupPropertiesForPatchUpdate{RetentionDays: retention}
	}
	if hasChange("public_network_access_enabled") {
		payload.Properties.Network = &clusters.NetworkForPatchUpdate{PublicAccess: pointer.To(horizondbPublicNetworkAccess(state.PublicNetworkAccessEnabled))}
	}
	if hasChange("ai_model_management_enabled") {
		payload.Properties.AiModelManagement = pointer.To(horizondbAiModelManagement(state.AiModelManagementEnabled))
	}
	updates := make([]clusters.ClusterForPatchUpdate, 0, 2)
	if len(state.CustomerManagedKey) > 0 {
		key := state.CustomerManagedKey[0]
		keyChanged := hasChange("customer_managed_key.0.key_vault_key_id")
		identityChanged := hasChange("customer_managed_key.0.user_assigned_identity_id")
		if keyChanged {
			payload.Properties.DataEncryption = &clusters.HorizonDbClusterDataEncryptionForPatchUpdate{PrimaryKeyUri: pointer.To(key.KeyVaultKeyId)}
		}
		if identityChanged {
			if keyChanged {
				// HorizonDB requires key and encryption-identity rotations to complete separately.
				updates = append(updates, payload)
				payload = clusters.ClusterForPatchUpdate{Properties: &clusters.ClusterPropertiesForPatchUpdate{}}
			}
			payload.Properties.DataEncryption = &clusters.HorizonDbClusterDataEncryptionForPatchUpdate{PrimaryUserAssignedIdentityId: pointer.To(key.UserAssignedIdentityId)}
		}
	}
	if payload.Tags != nil || payload.Properties.Backup != nil || payload.Properties.Network != nil || payload.Properties.AiModelManagement != nil || payload.Properties.DataEncryption != nil {
		updates = append(updates, payload)
	}
	return updates
}

func (HorizondbClusterResource) flatten(metadata sdk.ResourceMetaData, id *clusters.ClusterId, model *clusters.Cluster, pool *pools.Pool) error {
	if model == nil || model.Properties == nil {
		return fmt.Errorf("retrieving %s: cluster properties are missing", id)
	}
	var previous HorizondbClusterResourceModel
	if err := metadata.Decode(&previous); err != nil {
		return fmt.Errorf("decoding: %+v", err)
	}
	props := model.Properties
	state := HorizondbClusterResourceModel{
		Name:                     id.ClusterName,
		ResourceGroupName:        id.ResourceGroupName,
		Location:                 location.Normalize(model.Location),
		PostgresqlVersion:        pointer.FromEnum(props.PostgreSqlVersion),
		AiModelManagementEnabled: pointer.From(props.AiModelManagement) == clusters.AiModelManagementStatusEnabled,
		BackupRetentionDays:      7,
		CreateMode:               pointer.FromEnum(props.CreationMode),
		Tags:                     pointer.From(model.Tags),
	}
	if state.CreateMode == "" {
		state.CreateMode = string(clusters.ClusterCreateModeCreate)
	}
	if props.Network != nil {
		state.PublicNetworkAccessEnabled = pointer.From(props.Network.PublicAccess) == clusters.PublicNetworkAccessStateEnabled
	}
	if props.Backup != nil {
		if props.Backup.RetentionDays != nil {
			state.BackupRetentionDays = *props.Backup.RetentionDays
		}
		state.GeoRedundantBackupEnabled = props.Backup.GeoRedundancy == clusters.GeoRedundancyStateEnabled
		state.EarliestRestorePointUtc = pointer.From(props.Backup.EarliestRestorePointUtc)
	}
	flattenedIdentity, err := identity.FlattenUserAssignedMapToModel(model.Identity)
	if err != nil {
		return fmt.Errorf("flattening `identity`: %+v", err)
	}
	state.Identity = pointer.From(flattenedIdentity)
	if encryption := props.DataEncryption; encryption != nil {
		switch pointer.From(encryption.Type) {
		case "", clusters.HorizonDbClusterDataEncryptionTypeSystemManaged:
		case clusters.HorizonDbClusterDataEncryptionTypeAzureKeyVault:
			if pointer.From(encryption.PrimaryKeyUri) == "" {
				return fmt.Errorf("retrieving %s: customer-managed encryption key URI is missing", id)
			}
			identityId, err := commonids.ParseUserAssignedIdentityIDInsensitively(pointer.From(encryption.PrimaryUserAssignedIdentityId))
			if err != nil {
				return err
			}
			state.CustomerManagedKey = []HorizondbClusterCustomerManagedKey{{
				KeyVaultKeyId:          pointer.From(encryption.PrimaryKeyUri),
				UserAssignedIdentityId: identityId.ID(),
			}}
		default:
			return fmt.Errorf("retrieving %s: unsupported encryption type %q", id, *encryption.Type)
		}
	}
	if pointer.From(props.CreationMode) == clusters.ClusterCreateModePointInTimeRestore {
		sourceId, err := clusters.ParseClusterIDInsensitively(pointer.From(props.CreationSourceClusterResourceId))
		if err != nil {
			return err
		}
		restore := HorizondbClusterRestore{
			SourceHorizondbClusterId: sourceId.ID(),
			SourceLocation:           location.Normalize(pointer.From(props.CreationSourceClusterLocation)),
			PointInTimeUtc:           pointer.From(props.PointInTimeUtc),
		}
		// Cross-region restores do not return the requested timestamp.
		if props.PointInTimeUtc == nil && len(previous.Restore) > 0 {
			restore.PointInTimeUtc = previous.Restore[0].PointInTimeUtc
		}
		state.Restore = []HorizondbClusterRestore{restore}
	}
	state.Pool, err = flattenHorizondbClusterPool(*id, pool)
	if err != nil {
		return err
	}
	if err := pluginsdk.SetResourceIdentityData(metadata.ResourceData, id); err != nil {
		return err
	}
	return metadata.Encode(&state)
}

func horizondbPublicNetworkAccess(enabled bool) clusters.PublicNetworkAccessState {
	if enabled {
		return clusters.PublicNetworkAccessStateEnabled
	}
	return clusters.PublicNetworkAccessStateDisabled
}

func horizondbAiModelManagement(enabled bool) clusters.AiModelManagementStatus {
	if enabled {
		return clusters.AiModelManagementStatusEnabled
	}
	return clusters.AiModelManagementStatusDisabled
}
