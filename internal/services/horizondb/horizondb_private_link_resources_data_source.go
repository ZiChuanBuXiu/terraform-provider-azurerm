package horizondb

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/privatelinkresources"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

type HorizondbPrivateLinkResourcesDataSource struct{}

type HorizondbPrivateLinkResourcesDataSourceModel struct {
	HorizondbClusterId   string                              `tfschema:"horizondb_cluster_id"`
	PrivateLinkResources []HorizondbPrivateLinkResourceModel `tfschema:"private_link_resources"`
}

type HorizondbPrivateLinkResourceModel struct {
	Id                string   `tfschema:"id"`
	GroupId           string   `tfschema:"group_id"`
	RequiredMembers   []string `tfschema:"required_members"`
	RequiredZoneNames []string `tfschema:"required_zone_names"`
}

var _ sdk.DataSource = HorizondbPrivateLinkResourcesDataSource{}

func (HorizondbPrivateLinkResourcesDataSource) ModelObject() interface{} {
	return &HorizondbPrivateLinkResourcesDataSourceModel{}
}

func (HorizondbPrivateLinkResourcesDataSource) ResourceType() string {
	return "azurerm_horizondb_private_link_resources"
}

func (HorizondbPrivateLinkResourcesDataSource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"horizondb_cluster_id": commonschema.ResourceIDReferenceRequired(&privatelinkresources.ClusterId{}),
	}
}

func (HorizondbPrivateLinkResourcesDataSource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"private_link_resources": {
			Type:     pluginsdk.TypeList,
			Computed: true,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					"id":       {Type: pluginsdk.TypeString, Computed: true},
					"group_id": {Type: pluginsdk.TypeString, Computed: true},
					"required_members": {
						Type: pluginsdk.TypeList, Computed: true,
						Elem: &pluginsdk.Schema{Type: pluginsdk.TypeString},
					},
					"required_zone_names": {
						Type: pluginsdk.TypeList, Computed: true,
						Elem: &pluginsdk.Schema{Type: pluginsdk.TypeString},
					},
				},
			},
		},
	}
}

func (HorizondbPrivateLinkResourcesDataSource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			var state HorizondbPrivateLinkResourcesDataSourceModel
			if err := metadata.Decode(&state); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}
			id, err := privatelinkresources.ParseClusterID(state.HorizondbClusterId)
			if err != nil {
				return err
			}

			resp, err := metadata.Client.HorizonDB.PrivateLinkResourcesClient.ListComplete(ctx, *id)
			if err != nil {
				return fmt.Errorf("listing private link resources for %s: %+v", id, err)
			}
			state.PrivateLinkResources, err = flattenHorizondbPrivateLinkResources(resp.Items)
			if err != nil {
				return err
			}
			state.HorizondbClusterId = id.ID()
			metadata.SetID(*id)
			return metadata.Encode(&state)
		},
	}
}

func flattenHorizondbPrivateLinkResources(input []privatelinkresources.PrivateLinkResource) ([]HorizondbPrivateLinkResourceModel, error) {
	result := make([]HorizondbPrivateLinkResourceModel, 0, len(input))
	for _, resource := range input {
		id, err := privatelinkresources.ParsePrivateLinkResourceIDInsensitively(pointer.From(resource.Id))
		if err != nil {
			return nil, err
		}
		if resource.Properties == nil || resource.Properties.GroupId == nil {
			return nil, fmt.Errorf("retrieving %s: private link resource group is missing", id)
		}

		members := append([]string{}, pointer.From(resource.Properties.RequiredMembers)...)
		zones := append([]string{}, pointer.From(resource.Properties.RequiredZoneNames)...)
		sort.Strings(members)
		sort.Strings(zones)
		result = append(result, HorizondbPrivateLinkResourceModel{
			Id:                id.ID(),
			GroupId:           *resource.Properties.GroupId,
			RequiredMembers:   members,
			RequiredZoneNames: zones,
		})
	}
	sort.Slice(result, func(first, second int) bool {
		return result[first].Id < result[second].Id
	})
	return result, nil
}
