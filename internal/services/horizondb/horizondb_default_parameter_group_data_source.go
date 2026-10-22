package horizondb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/location"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/defaultparametergroups"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
)

type HorizondbDefaultParameterGroupDataSource struct{}

type HorizondbDefaultParameterGroupDataSourceModel struct {
	Location          string            `tfschema:"location"`
	PostgresqlVersion string            `tfschema:"postgresql_version"`
	Name              string            `tfschema:"name"`
	Description       string            `tfschema:"description"`
	Parameters        map[string]string `tfschema:"parameters"`
}

var _ sdk.DataSource = HorizondbDefaultParameterGroupDataSource{}

func (HorizondbDefaultParameterGroupDataSource) ModelObject() interface{} {
	return &HorizondbDefaultParameterGroupDataSourceModel{}
}

func (HorizondbDefaultParameterGroupDataSource) ResourceType() string {
	return "azurerm_horizondb_default_parameter_group"
}

func (HorizondbDefaultParameterGroupDataSource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"location": commonschema.Location(),
		"postgresql_version": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ValidateFunc: validation.StringInSlice(defaultparametergroups.PossibleValuesForPostgreSqlVersion(), false),
		},
	}
}

func (HorizondbDefaultParameterGroupDataSource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name":        {Type: pluginsdk.TypeString, Computed: true},
		"description": {Type: pluginsdk.TypeString, Computed: true},
		"parameters": {
			Type:     pluginsdk.TypeMap,
			Computed: true,
			Elem:     &pluginsdk.Schema{Type: pluginsdk.TypeString},
		},
	}
}

func (HorizondbDefaultParameterGroupDataSource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			var state HorizondbDefaultParameterGroupDataSourceModel
			if err := metadata.Decode(&state); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			state.Location = location.Normalize(state.Location)
			id := defaultparametergroups.NewLocationID(metadata.Client.Account.SubscriptionId, state.Location)
			resp, err := metadata.Client.HorizonDB.DefaultParameterGroupsClient.ListBySubscriptionAndRegionComplete(ctx, id)
			if err != nil {
				return fmt.Errorf("listing default parameter groups for %s: %+v", id, err)
			}

			group, err := findHorizondbDefaultParameterGroup(resp.Items, state.PostgresqlVersion)
			if err != nil {
				return fmt.Errorf("retrieving a default parameter group in `%s`: %+v", state.Location, err)
			}
			groupId, err := defaultparametergroups.ParseParameterGroupIDInsensitively(pointer.From(group.Id))
			if err != nil {
				return err
			}

			state.Name = groupId.ParameterGroupName
			state.Description = pointer.From(group.Properties.Description)
			state.Parameters = make(map[string]string)
			for _, parameter := range group.Properties.Parameters {
				if parameter.Name != nil && parameter.Value != nil {
					state.Parameters[*parameter.Name] = *parameter.Value
				}
			}

			metadata.SetID(*groupId)
			return metadata.Encode(&state)
		},
	}
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
