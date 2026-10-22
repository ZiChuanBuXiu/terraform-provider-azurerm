// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/resourceids"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/clusters"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/defaultparametergroups"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/parametergroups"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/pools"
	"github.com/hashicorp/terraform-provider-azurerm/internal/locks"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
)

//go:generate go run ../../tools/generator-tests resourceidentity -parent-id "horizondb_pool_id" -test-env-vars "ARM_TEST_HORIZONDB_POOL_ID"
type HorizondbPoolParameterGroupAssociationResource struct{}

type HorizondbPoolParameterGroupAssociationResourceModel struct {
	HorizondbPoolId           string `tfschema:"horizondb_pool_id"`
	HorizondbParameterGroupId string `tfschema:"horizondb_parameter_group_id"`
	ApplyMethod               string `tfschema:"apply_method"`
	SyncStatus                string `tfschema:"sync_status"`
}

var (
	_ sdk.ResourceWithUpdate   = HorizondbPoolParameterGroupAssociationResource{}
	_ sdk.ResourceWithIdentity = HorizondbPoolParameterGroupAssociationResource{}
)

func (HorizondbPoolParameterGroupAssociationResource) ModelObject() interface{} {
	return &HorizondbPoolParameterGroupAssociationResourceModel{}
}

func (HorizondbPoolParameterGroupAssociationResource) ResourceType() string {
	return "azurerm_horizondb_pool_parameter_group_association"
}

func (HorizondbPoolParameterGroupAssociationResource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return pools.ValidatePoolID
}

func (HorizondbPoolParameterGroupAssociationResource) Identity() resourceids.ResourceId {
	return &pools.PoolId{}
}

func (HorizondbPoolParameterGroupAssociationResource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"horizondb_pool_id":            commonschema.ResourceIDReferenceRequiredForceNew(&pools.PoolId{}),
		"horizondb_parameter_group_id": commonschema.ResourceIDReferenceRequired(&parametergroups.ProviderParameterGroupId{}),
		"apply_method": {
			Type: pluginsdk.TypeString, Required: true,
			ValidateFunc: validation.StringInSlice(pools.PossibleValuesForParameterApplyMethod(), false),
		},
	}
}

func (HorizondbPoolParameterGroupAssociationResource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{"sync_status": {Type: pluginsdk.TypeString, Computed: true}}
}

func (r HorizondbPoolParameterGroupAssociationResource) Create() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 60 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			return r.setAssociation(ctx, metadata, true)
		},
	}
}

func (r HorizondbPoolParameterGroupAssociationResource) Update() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 60 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			return r.setAssociation(ctx, metadata, false)
		},
	}
}

func (r HorizondbPoolParameterGroupAssociationResource) setAssociation(ctx context.Context, metadata sdk.ResourceMetaData, creating bool) error {
	var state HorizondbPoolParameterGroupAssociationResourceModel
	if err := metadata.Decode(&state); err != nil {
		return fmt.Errorf("decoding: %+v", err)
	}
	id, err := pools.ParsePoolID(state.HorizondbPoolId)
	if err != nil {
		return err
	}
	lockId := strings.ToLower(id.ID())
	locks.ByID(lockId)
	defer locks.UnlockByID(lockId)
	client := metadata.Client.HorizonDB.PoolsClient
	existing, err := client.Get(ctx, *id)
	if err != nil {
		return fmt.Errorf("retrieving %s: %+v", id, err)
	}
	groupId, custom, err := horizondbPoolParameterGroup(existing.Model)
	if err != nil {
		return fmt.Errorf("retrieving association for %s: %+v", id, err)
	}
	if creating {
		if custom && !metadata.Client.Features.SkipImportCheckOnCreateAndAllowOverwritingExistingResources {
			return metadata.ResourceRequiresImport(r.ResourceType(), *id)
		}
	} else {
		old, _ := metadata.ResourceData.GetChange("horizondb_parameter_group_id")
		if !custom || !strings.EqualFold(groupId, old.(string)) {
			return fmt.Errorf("updating association for %s: remote parameter group changed; refresh state before retrying", id)
		}
	}
	payload := expandHorizondbPoolParameterGroup(state.HorizondbParameterGroupId, state.ApplyMethod)
	var callback func() error
	if creating {
		callback = metadata.SetIDAndIdentityCallback(id)
	}
	if err := client.UpdateCallbackThenPoll(ctx, *id, payload, callback); err != nil {
		return fmt.Errorf("updating parameter group association for %s: %+v", id, err)
	}
	metadata.SetID(*id)
	return pluginsdk.SetResourceIdentityData(metadata.ResourceData, id)
}

func (r HorizondbPoolParameterGroupAssociationResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			id, err := pools.ParsePoolIDInsensitively(metadata.ResourceData.Id())
			if err != nil {
				return err
			}
			resp, err := metadata.Client.HorizonDB.PoolsClient.Get(ctx, *id)
			if err != nil {
				if response.WasNotFound(resp.HttpResponse) {
					return metadata.MarkAsGone(*id)
				}
				return fmt.Errorf("retrieving %s: %+v", id, err)
			}
			_, custom, err := horizondbPoolParameterGroup(resp.Model)
			if err != nil {
				return fmt.Errorf("retrieving association for %s: %+v", id, err)
			}
			if !custom {
				return metadata.MarkAsGone(*id)
			}
			return r.flatten(metadata, id, resp.Model)
		},
	}
}

func (HorizondbPoolParameterGroupAssociationResource) Delete() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 60 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			id, err := pools.ParsePoolIDInsensitively(metadata.ResourceData.Id())
			if err != nil {
				return err
			}
			lockId := strings.ToLower(id.ID())
			locks.ByID(lockId)
			defer locks.UnlockByID(lockId)
			client := metadata.Client.HorizonDB
			existing, err := client.PoolsClient.Get(ctx, *id)
			if err != nil {
				if response.WasNotFound(existing.HttpResponse) {
					return nil
				}
				return fmt.Errorf("retrieving %s: %+v", id, err)
			}
			groupId, custom, err := horizondbPoolParameterGroup(existing.Model)
			if err != nil {
				return fmt.Errorf("retrieving association for %s: %+v", id, err)
			}
			if !custom {
				return nil
			}
			var state HorizondbPoolParameterGroupAssociationResourceModel
			if err := metadata.Decode(&state); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}
			if !strings.EqualFold(groupId, state.HorizondbParameterGroupId) {
				return fmt.Errorf("deleting association for %s: remote parameter group changed; refusing to detach another association", id)
			}
			clusterId := clusters.NewClusterID(id.SubscriptionId, id.ResourceGroupName, id.ClusterName)
			defaultId, err := defaultHorizondbClusterPoolParameterGroup(ctx, metadata, clusterId)
			if err != nil {
				return fmt.Errorf("finding default parameter group for %s: %+v", id, err)
			}
			if err := client.PoolsClient.UpdateThenPoll(ctx, *id, expandHorizondbPoolParameterGroup(defaultId, state.ApplyMethod)); err != nil {
				return fmt.Errorf("restoring default parameter group for %s: %+v", id, err)
			}
			return nil
		},
	}
}

func expandHorizondbPoolParameterGroup(groupId, applyMethod string) pools.PoolForPatchUpdate {
	return pools.PoolForPatchUpdate{Properties: &pools.PoolPropertiesForPatchUpdate{
		ParameterGroup: &pools.PoolParameterGroupReferenceForPatchUpdate{
			ParameterGroupResourceId: pointer.To(groupId), ApplyMethod: pointer.To(pools.ParameterApplyMethod(applyMethod)),
		},
	}}
}

func horizondbPoolParameterGroup(model *pools.Pool) (string, bool, error) {
	if model == nil || model.Properties == nil {
		return "", false, errors.New("pool properties are missing")
	}
	rawId := model.Properties.ParameterGroup.ParameterGroupResourceId
	if id, err := parametergroups.ParseProviderParameterGroupIDInsensitively(rawId); err == nil {
		return id.ID(), true, nil
	}
	if id, err := defaultparametergroups.ParseParameterGroupIDInsensitively(rawId); err == nil {
		return id.ID(), false, nil
	}
	return "", false, fmt.Errorf("invalid parameter group ID %q returned by pool", rawId)
}

func (HorizondbPoolParameterGroupAssociationResource) flatten(metadata sdk.ResourceMetaData, id *pools.PoolId, model *pools.Pool) error {
	groupId, custom, err := horizondbPoolParameterGroup(model)
	if err != nil {
		return err
	}
	if !custom {
		return fmt.Errorf("pool %s is associated with a default parameter group, not a customer-created group", id)
	}
	if model.Properties.ParameterGroup.ApplyMethod == "" {
		return fmt.Errorf("pool %s has no parameter group apply method", id)
	}
	state := HorizondbPoolParameterGroupAssociationResourceModel{
		HorizondbPoolId: id.ID(), HorizondbParameterGroupId: groupId,
		ApplyMethod: string(model.Properties.ParameterGroup.ApplyMethod), SyncStatus: string(pointer.From(model.Properties.ParameterGroup.SyncStatus)),
	}
	metadata.SetID(*id)
	if err := pluginsdk.SetResourceIdentityData(metadata.ResourceData, id); err != nil {
		return err
	}
	return metadata.Encode(&state)
}
