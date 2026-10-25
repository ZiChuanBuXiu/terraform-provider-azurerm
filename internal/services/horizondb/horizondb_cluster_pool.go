// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/location"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/clusters"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/defaultparametergroups"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/pools"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-provider-azurerm/internal/locks"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
)

type HorizondbClusterPool struct {
	Name                 string                        `tfschema:"name"`
	Id                   string                        `tfschema:"id"`
	Compute              []HorizondbClusterPoolCompute `tfschema:"compute"`
	NodeCount            int64                         `tfschema:"node_count"`
	HighAvailabilityMode string                        `tfschema:"high_availability_mode"`
}

type HorizondbClusterPoolCompute struct {
	Processor  string `tfschema:"processor"`
	Generation int64  `tfschema:"generation"`
	VCores     int64  `tfschema:"vcores"`
}

func horizondbClusterPoolSchema() *pluginsdk.Schema {
	return &pluginsdk.Schema{
		Type:     pluginsdk.TypeList,
		Optional: true,
		MinItems: 1,
		MaxItems: 1,
		Elem: &pluginsdk.Resource{Schema: map[string]*pluginsdk.Schema{
			"name": {
				Type:     pluginsdk.TypeString,
				Required: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(1, 63),
					validation.StringMatch(regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`), "must contain only lowercase letters, numbers and hyphens and start and end with a letter or number"),
				),
			},
			"compute": {
				Type:     pluginsdk.TypeList,
				Required: true,
				MinItems: 1,
				MaxItems: 1,
				Elem: &pluginsdk.Resource{Schema: map[string]*pluginsdk.Schema{
					"processor": {
						Type: pluginsdk.TypeString, Required: true,
						ValidateFunc: validation.StringInSlice(pools.PossibleValuesForComputeProcessorType(), false),
					},
					"generation": {
						Type: pluginsdk.TypeInt, Required: true,
						ValidateFunc: validation.IntBetween(1, 2147483647),
					},
					"vcores": {
						Type: pluginsdk.TypeInt, Required: true,
						ValidateFunc: validation.IntBetween(2, 192),
					},
				}},
			},
			"node_count": {
				Type:         pluginsdk.TypeInt,
				Required:     true,
				ValidateFunc: validation.IntBetween(1, 16),
			},
			"high_availability_mode": {
				Type:         pluginsdk.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice(pools.PossibleValuesForPoolHighAvailabilityMode(), false),
			},
			"id": {
				Type:     pluginsdk.TypeString,
				Computed: true,
			},
		}},
	}
}

func validateHorizondbClusterPoolConfig(config cty.Value) error {
	if !config.IsKnown() || config.IsNull() {
		return nil
	}
	block := config.GetAttr("pool")
	if !block.IsKnown() || block.IsNull() || block.LengthInt() == 0 {
		return nil
	}
	pool := block.AsValueSlice()[0]
	if !pool.IsKnown() || pool.IsNull() {
		return nil
	}
	name, clusterName := pool.GetAttr("name"), config.GetAttr("name")
	if name.IsKnown() && !name.IsNull() && clusterName.IsKnown() && !clusterName.IsNull() && strings.EqualFold(name.AsString(), clusterName.AsString()) {
		return errors.New("`pool.name` must differ from the cluster `name`")
	}
	count, mode := pool.GetAttr("node_count"), pool.GetAttr("high_availability_mode")
	if count.IsKnown() && !count.IsNull() && mode.IsKnown() && !mode.IsNull() {
		nodes, _ := count.AsBigFloat().Int64()
		if (nodes == 1) != (mode.AsString() == string(pools.PoolHighAvailabilityModeDisabled)) {
			return errors.New("`pool.high_availability_mode` must be `Disabled` for one node, and `AnyZone` or `ZoneRedundant` for two or more nodes")
		}
	}
	return nil
}

func validateHorizondbClusterPoolRename(old, new string) error {
	if old != "" && new != "" && old != new {
		return errors.New("changing `pool.name` in place is not supported; apply removal of the `pool` block before adding a pool with a new name")
	}
	return nil
}

func readHorizondbClusterPool(ctx context.Context, metadata sdk.ResourceMetaData, id clusters.ClusterId) (*pools.Pool, error) {
	result, err := metadata.Client.HorizonDB.PoolsClient.ListComplete(ctx, pools.NewClusterID(id.SubscriptionId, id.ResourceGroupName, id.ClusterName))
	if err != nil {
		return nil, fmt.Errorf("listing pools for %s: %+v", id, err)
	}
	if len(result.Items) > 1 {
		return nil, fmt.Errorf("retrieving %s: expected at most one pool, got %d", id, len(result.Items))
	}
	if len(result.Items) == 0 {
		return nil, nil
	}
	if _, err := flattenHorizondbClusterPool(id, &result.Items[0]); err != nil {
		return nil, err
	}
	return &result.Items[0], nil
}

func flattenHorizondbClusterPool(clusterId clusters.ClusterId, model *pools.Pool) ([]HorizondbClusterPool, error) {
	if model == nil {
		return nil, nil
	}
	id, err := pools.ParsePoolIDInsensitively(pointer.From(model.Id))
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(id.SubscriptionId, clusterId.SubscriptionId) || !strings.EqualFold(id.ResourceGroupName, clusterId.ResourceGroupName) || !strings.EqualFold(id.ClusterName, clusterId.ClusterName) {
		return nil, fmt.Errorf("pool %s does not belong to %s", id, clusterId)
	}
	props := model.Properties
	if props == nil || props.ComputeModel.ComputeProcessor == "" || props.ComputeModel.ComputeGeneration < 1 || props.ComputeModel.VCores < 2 || props.NodeCount < 1 || props.HighAvailability.Mode == "" {
		return nil, fmt.Errorf("retrieving %s: required pool configuration is missing", id)
	}
	if props.ComputeModel.Type != pools.PoolComputeModelTypeProvisioned {
		return nil, fmt.Errorf("retrieving %s: unsupported compute model type %q", id, props.ComputeModel.Type)
	}
	if props.ScheduledMaintenance != nil {
		maintenance := props.ScheduledMaintenance.ScheduledMaintenanceProperties().MaintenanceManagementType
		if maintenance != pools.ScheduledMaintenanceManagementTypeSystem {
			return nil, fmt.Errorf("retrieving %s: unsupported maintenance management type %q", id, maintenance)
		}
	}
	return []HorizondbClusterPool{{
		Name: id.PoolName, Id: id.ID(), NodeCount: props.NodeCount,
		HighAvailabilityMode: string(props.HighAvailability.Mode),
		Compute: []HorizondbClusterPoolCompute{{
			Processor: string(props.ComputeModel.ComputeProcessor), Generation: props.ComputeModel.ComputeGeneration,
			VCores: props.ComputeModel.VCores,
		}},
	}}, nil
}

func reconcileHorizondbClusterPool(ctx context.Context, metadata sdk.ResourceMetaData, clusterId clusters.ClusterId, desired []HorizondbClusterPool) error {
	oldName, _ := metadata.ResourceData.GetChange("pool.0.name")
	name := oldName.(string)
	if len(desired) > 0 {
		name = desired[0].Name
	}
	if name == "" {
		existing, err := readHorizondbClusterPool(ctx, metadata, clusterId)
		if err != nil {
			return err
		}
		if existing == nil {
			return nil
		}
		id, err := pools.ParsePoolIDInsensitively(pointer.From(existing.Id))
		if err != nil {
			return err
		}
		name = id.PoolName
	}
	id := pools.NewPoolID(clusterId.SubscriptionId, clusterId.ResourceGroupName, clusterId.ClusterName, name)
	lockId := strings.ToLower(id.ID())
	locks.ByID(lockId)
	defer locks.UnlockByID(lockId)

	existing, err := readHorizondbClusterPool(ctx, metadata, clusterId)
	if err != nil {
		return err
	}
	if existing != nil {
		existingId, err := pools.ParsePoolIDInsensitively(pointer.From(existing.Id))
		if err != nil {
			return err
		}
		if !strings.EqualFold(existingId.ID(), id.ID()) {
			return fmt.Errorf("reconciling %s: remote pool changed; refresh state before retrying", clusterId)
		}
	}
	client := metadata.Client.HorizonDB.PoolsClient
	if len(desired) == 0 {
		if existing == nil {
			return nil
		}
		if err := client.DeleteThenPoll(ctx, id); err != nil {
			return fmt.Errorf("deleting %s: %+v", id, err)
		}
		return nil
	}
	state := desired[0]
	if existing == nil {
		groupId, err := defaultHorizondbClusterPoolParameterGroup(ctx, metadata, clusterId)
		if err != nil {
			return err
		}
		payload := pools.Pool{
			Properties: &pools.PoolProperties{
				ComputeModel: expandHorizondbClusterPoolCompute(state.Compute[0]),
				NodeCount:    state.NodeCount, HighAvailability: pools.PoolHighAvailability{Mode: pools.PoolHighAvailabilityMode(state.HighAvailabilityMode)},
				ParameterGroup: pools.PoolParameterGroupReference{
					ParameterGroupResourceId: groupId, ApplyMethod: pools.ParameterApplyMethodImmediate,
				},
				ScheduledMaintenance: pools.SystemScheduledMaintenanceProperties{},
			},
		}
		if err := client.CreateOrReplaceThenPoll(ctx, id, payload); err != nil {
			return fmt.Errorf("creating %s: %+v", id, err)
		}
		return nil
	}
	payload := expandHorizondbClusterPoolUpdate(state)
	if err := client.UpdateThenPoll(ctx, id, payload); err != nil {
		return fmt.Errorf("updating %s: %+v", id, err)
	}
	return nil
}

func expandHorizondbClusterPoolCompute(input HorizondbClusterPoolCompute) pools.PoolComputeModel {
	return pools.PoolComputeModel{
		Type: pools.PoolComputeModelTypeProvisioned, 
		ComputeProcessor: pools.ComputeProcessorType(input.Processor),
		ComputeGeneration: input.Generation, VCores: input.VCores,
	}
}

func expandHorizondbClusterPoolUpdate(state HorizondbClusterPool) pools.PoolForPatchUpdate {
	compute := expandHorizondbClusterPoolCompute(state.Compute[0])
	return pools.PoolForPatchUpdate{
		Properties: &pools.PoolPropertiesForPatchUpdate{
			ComputeModel: &pools.PoolComputeModelForPatchUpdate{
				Type: pointer.To(compute.Type), ComputeProcessor: pointer.To(compute.ComputeProcessor),
				ComputeGeneration: pointer.To(compute.ComputeGeneration), VCores: pointer.To(compute.VCores),
			},
			NodeCount:        pointer.To(state.NodeCount),
			HighAvailability: &pools.PoolHighAvailabilityPatch{Mode: pointer.To(pools.PoolHighAvailabilityMode(state.HighAvailabilityMode))},
			ScheduledMaintenance: &pools.ScheduledMaintenancePropertiesForPatchUpdate{
				MaintenanceManagementType: pointer.To(pools.ScheduledMaintenanceManagementTypeSystem),
			},
		},
	}
}

func defaultHorizondbClusterPoolParameterGroup(ctx context.Context, metadata sdk.ResourceMetaData, id clusters.ClusterId) (string, error) {
	cluster, err := metadata.Client.HorizonDB.ClustersClient.Get(ctx, id)
	if err != nil {
		return "", fmt.Errorf("retrieving %s: %+v", id, err)
	}
	if cluster.Model == nil || cluster.Model.Properties == nil || pointer.From(cluster.Model.Properties.PostgreSqlVersion) == "" {
		return "", fmt.Errorf("retrieving %s: PostgreSQL version is missing", id)
	}
	if cluster.Model.Location == "" {
		return "", fmt.Errorf("retrieving %s: location is missing", id)
	}
	locationId := defaultparametergroups.NewLocationID(id.SubscriptionId, location.Normalize(cluster.Model.Location))
	result, err := metadata.Client.HorizonDB.DefaultParameterGroupsClient.ListBySubscriptionAndRegionComplete(ctx, locationId)
	if err != nil {
		return "", fmt.Errorf("listing default parameter groups for %s: %+v", id, err)
	}
	group, err := findHorizondbDefaultParameterGroup(result.Items, string(*cluster.Model.Properties.PostgreSqlVersion))
	if err != nil {
		return "", err
	}
	groupId, err := defaultparametergroups.ParseParameterGroupIDInsensitively(pointer.From(group.Id))
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(groupId.SubscriptionId, id.SubscriptionId) || location.Normalize(groupId.LocationName) != locationId.LocationName {
		return "", fmt.Errorf("default parameter group %q does not match the cluster subscription and location", groupId.ID())
	}
	return groupId.ID(), nil
}

func findHorizondbDefaultParameterGroup(groups []defaultparametergroups.DefaultParameterGroup, version string) (*defaultparametergroups.DefaultParameterGroup, error) {
	var result *defaultparametergroups.DefaultParameterGroup
	for index := range groups {
		group := &groups[index]
		if group.Properties == nil {
			return nil, errors.New("default parameter group response has no properties")
		}
		if string(group.Properties.PostgreSqlVersion) != version {
			continue
		}
		if result != nil {
			return nil, fmt.Errorf("multiple default parameter groups found for PostgreSQL version `%s`", version)
		}
		result = group
	}
	if result == nil {
		return nil, fmt.Errorf("no default parameter group found for PostgreSQL version `%s`", version)
	}
	return result, nil
}
