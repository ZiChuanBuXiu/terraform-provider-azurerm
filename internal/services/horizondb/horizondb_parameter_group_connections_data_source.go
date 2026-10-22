// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/parametergroups"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/pools"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

type HorizondbParameterGroupConnectionsDataSource struct{}

type HorizondbParameterGroupConnectionsDataSourceModel struct {
	HorizondbParameterGroupId string   `tfschema:"horizondb_parameter_group_id"`
	HorizondbPoolIds          []string `tfschema:"horizondb_pool_ids"`
}

var _ sdk.DataSource = HorizondbParameterGroupConnectionsDataSource{}

func (HorizondbParameterGroupConnectionsDataSource) ModelObject() interface{} {
	return &HorizondbParameterGroupConnectionsDataSourceModel{}
}
func (HorizondbParameterGroupConnectionsDataSource) ResourceType() string {
	return "azurerm_horizondb_parameter_group_connections"
}
func (HorizondbParameterGroupConnectionsDataSource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"horizondb_parameter_group_id": commonschema.ResourceIDReferenceRequired(&parametergroups.ProviderParameterGroupId{}),
	}
}
func (HorizondbParameterGroupConnectionsDataSource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"horizondb_pool_ids": {Type: pluginsdk.TypeSet, Computed: true, Elem: &pluginsdk.Schema{Type: pluginsdk.TypeString}},
	}
}
func (HorizondbParameterGroupConnectionsDataSource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			var state HorizondbParameterGroupConnectionsDataSourceModel
			if err := metadata.Decode(&state); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}
			id, err := parametergroups.ParseProviderParameterGroupID(state.HorizondbParameterGroupId)
			if err != nil {
				return err
			}
			resp, err := metadata.Client.HorizonDB.ParameterGroupsClient.ListConnectionsComplete(ctx, *id)
			if err != nil {
				return fmt.Errorf("listing connections for %s: %+v", id, err)
			}
			state.HorizondbPoolIds = make([]string, 0, len(resp.Items))
			for _, connection := range resp.Items {
				poolId, err := pools.ParsePoolIDInsensitively(connection.PoolResourceId)
				if err != nil {
					return fmt.Errorf("parsing connected pool ID for %s: %+v", id, err)
				}
				state.HorizondbPoolIds = append(state.HorizondbPoolIds, poolId.ID())
			}
			slices.Sort(state.HorizondbPoolIds)
			state.HorizondbParameterGroupId = id.ID()
			metadata.SetID(*id)
			return metadata.Encode(&state)
		},
	}
}
