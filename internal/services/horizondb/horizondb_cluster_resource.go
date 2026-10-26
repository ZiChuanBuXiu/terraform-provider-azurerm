// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/identity"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/location"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/resourceids"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/clusters"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/passwordauthentications"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/pools"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
)

//go:generate go run ../../tools/generator-tests resourceidentity
type HorizondbClusterResource struct{}

type HorizondbClusterResourceModel struct {
	Name                       string                       `tfschema:"name"`
	ResourceGroupName          string                       `tfschema:"resource_group_name"`
	Location                   string                       `tfschema:"location"`
	PostgresqlVersion          string                       `tfschema:"postgresql_version"`
	Pool                       []HorizondbClusterPool       `tfschema:"pool"`
	AdministratorLogin         string                       `tfschema:"administrator_login"`
	AdministratorPassword      string                       `tfschema:"administrator_password"`
	BackupRetentionDays        int64                        `tfschema:"backup_retention_days"`
	CreateMode                 string                       `tfschema:"create_mode"`
	GeoRedundantBackupEnabled  bool                         `tfschema:"geo_redundant_backup_enabled"`
	Identity                   []identity.ModelUserAssigned `tfschema:"identity"`
	PublicNetworkAccessEnabled bool                         `tfschema:"public_network_access_enabled"`
	Restore                    []HorizondbClusterRestore    `tfschema:"restore"`
	Tags                       map[string]string            `tfschema:"tags"`
	EarliestRestorePointUtc    string                       `tfschema:"earliest_restore_point_utc"`
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
		"administrator_login": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ForceNew:     true,
			ValidateFunc: validation.StringLenBetween(1, 63),
		},
		"administrator_password": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			Sensitive:    true,
			ValidateFunc: validation.StringLenBetween(8, 128),
		},
		"backup_retention_days": {
			Type:         pluginsdk.TypeInt,
			Optional:     true,
			Default:      7,
			ValidateFunc: validation.IntBetween(1, 35),
		},
		"create_mode": {
			Type:         pluginsdk.TypeString,
			Optional:     true,
			ForceNew:     true,
			Default:      string(clusters.ClusterCreateModeCreate),
			ValidateFunc: validation.StringInSlice(clusters.PossibleValuesForClusterCreateMode(), false),
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
	return nil
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
			if len(state.Pool) > 0 || metadata.Client.Features.SkipImportCheckOnCreateAndAllowOverwritingExistingResources {
				if err := reconcileHorizondbClusterPool(ctx, metadata, id, state.Pool); err != nil {
					return err
				}
			}
			return reconcileHorizondbClusterPasswordAuthentication(ctx, metadata, id, state)
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
				if err := reconcileHorizondbClusterPool(ctx, metadata, *id, state.Pool); err != nil {
					return err
				}
			}
			if metadata.ResourceData.HasChange("administrator_password") {
				authenticationId := passwordauthentications.NewClusterID(id.SubscriptionId, id.ResourceGroupName, id.ClusterName)
				payload := passwordauthentications.PasswordAuthenticationForPatchUpdate{
					Properties: &passwordauthentications.PasswordAuthenticationPropertiesForPatchUpdate{
						AdministratorLoginPassword: pointer.To(state.AdministratorPassword),
					},
				}
				if err := metadata.Client.HorizonDB.PasswordAuthenticationsClient.UpdateThenPoll(ctx, authenticationId, payload); err != nil {
					return fmt.Errorf("updating password authentication for %s: %+v", id, err)
				}
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

func reconcileHorizondbClusterPasswordAuthentication(ctx context.Context, metadata sdk.ResourceMetaData, id clusters.ClusterId, state HorizondbClusterResourceModel) error {
	properties := passwordauthentications.PasswordAuthenticationProperties{
		State:                      passwordauthentications.AuthenticationStateEnabled,
		AdministratorLogin:         pointer.To(state.AdministratorLogin),
		AdministratorLoginPassword: pointer.To(state.AdministratorPassword),
	}
	authenticationId := passwordauthentications.NewClusterID(id.SubscriptionId, id.ResourceGroupName, id.ClusterName)
	payload := passwordauthentications.PasswordAuthentication{Properties: &properties}
	if err := metadata.Client.HorizonDB.PasswordAuthenticationsClient.CreateOrReplaceThenPoll(ctx, authenticationId, payload); err != nil {
		return fmt.Errorf("configuring password authentication for %s: %+v", id, err)
	}
	return nil
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
	if payload.Tags != nil || payload.Properties.Backup != nil || payload.Properties.Network != nil {
		return []clusters.ClusterForPatchUpdate{payload}
	}
	return nil
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
		Name:                  id.ClusterName,
		ResourceGroupName:     id.ResourceGroupName,
		Location:              location.Normalize(model.Location),
		PostgresqlVersion:     pointer.FromEnum(props.PostgreSqlVersion),
		AdministratorLogin:    previous.AdministratorLogin,
		AdministratorPassword: previous.AdministratorPassword,
		BackupRetentionDays:   7,
		CreateMode:            pointer.FromEnum(props.CreationMode),
		Tags:                  pointer.From(model.Tags),
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
	if pointer.From(props.CreationMode) == clusters.ClusterCreateModePointInTimeRestore {
		restore := HorizondbClusterRestore{
			SourceHorizondbClusterId: pointer.From(props.CreationSourceClusterResourceId),
			SourceLocation:           location.Normalize(pointer.From(props.CreationSourceClusterLocation)),
			PointInTimeUtc:           pointer.From(props.PointInTimeUtc),
		}
		// Azure can omit creation-time restore metadata from GET responses.
		if len(previous.Restore) > 0 {
			if restore.SourceHorizondbClusterId == "" {
				restore.SourceHorizondbClusterId = previous.Restore[0].SourceHorizondbClusterId
			}
			if restore.SourceLocation == "" {
				restore.SourceLocation = location.Normalize(previous.Restore[0].SourceLocation)
			}
			if restore.PointInTimeUtc == "" {
				restore.PointInTimeUtc = previous.Restore[0].PointInTimeUtc
			}
		}
		if restore.SourceHorizondbClusterId != "" || len(previous.Restore) > 0 {
			sourceId, err := clusters.ParseClusterIDInsensitively(restore.SourceHorizondbClusterId)
			if err != nil {
				return err
			}
			restore.SourceHorizondbClusterId = sourceId.ID()
			state.Restore = []HorizondbClusterRestore{restore}
		}
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
