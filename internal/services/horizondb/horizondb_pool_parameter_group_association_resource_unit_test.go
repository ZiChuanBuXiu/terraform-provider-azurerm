// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/defaultparametergroups"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/parametergroups"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/pools"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHorizondbPoolParameterGroupPayload(t *testing.T) {
	body, err := json.Marshal(expandHorizondbPoolParameterGroup("group-id", "PendingReboot"))
	require.NoError(t, err)
	require.JSONEq(t, `{"properties":{"parameterGroup":{"parameterGroupResourceId":"group-id","applyMethod":"PendingReboot"}}}`, string(body))
}

func TestHorizondbPoolParameterGroupFlatten(t *testing.T) {
	id := pools.NewPoolID(horizondbParameterTestSubscription, "example-rg", "example-cluster", "example-pool")
	group := parametergroups.NewProviderParameterGroupID(id.SubscriptionId, id.ResourceGroupName, "example-parameters")
	resource := HorizondbPoolParameterGroupAssociationResource{}
	metadata := sdk.NewResourceMetaData(nil, resource)
	model := &pools.Pool{Properties: &pools.PoolProperties{ParameterGroup: pools.PoolParameterGroupReference{
		ParameterGroupResourceId: strings.ReplaceAll(group.ID(), "resourceGroups", "resourcegroups"),
		ApplyMethod:              pools.ParameterApplyMethodPendingReboot, SyncStatus: pointer.To(pools.ParameterGroupSyncStatus("PendingReboot")),
	}}}
	require.NoError(t, resource.flatten(metadata, &id, model))
	require.Equal(t, group.ID(), metadata.ResourceData.Get("horizondb_parameter_group_id"))
	require.Equal(t, "PendingReboot", metadata.ResourceData.Get("sync_status"))
	identity, err := metadata.ResourceData.Identity()
	require.NoError(t, err)
	require.Equal(t, id.PoolName, identity.Get("name"))
	require.Equal(t, id.ClusterName, identity.Get("cluster_name"))
	model.Properties.ParameterGroup.ParameterGroupResourceId = defaultparametergroups.NewParameterGroupID(id.SubscriptionId, "eastus2", "default").ID()
	require.ErrorContains(t, resource.flatten(metadata, &id, model), "default parameter group")
	model.Properties.ParameterGroup.ParameterGroupResourceId = "invalid"
	require.Error(t, resource.flatten(metadata, &id, model))
}

func TestHorizondbPoolParameterGroupCreate(t *testing.T) {
	id := pools.NewPoolID(horizondbParameterTestSubscription, "example-rg", "example-cluster", "example-pool")
	group := parametergroups.NewProviderParameterGroupID(id.SubscriptionId, id.ResourceGroupName, "example-parameters")
	defaultGroup := defaultparametergroups.NewParameterGroupID(id.SubscriptionId, "eastus2", "default")
	for _, test := range []struct {
		name                       string
		custom, overwrite, missing bool
		patches                    int32
		error                      string
	}{
		{name: "attach", patches: 1},
		{name: "requires import", custom: true, error: "needs to be imported"},
		{name: "overwrite", custom: true, overwrite: true, patches: 1},
		{name: "missing pool", missing: true, error: "retrieving"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var patches atomic.Int32
			client := newHorizondbParameterTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, id.ID(), r.URL.Path)
				if test.missing {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if r.Method == http.MethodPatch {
					patches.Add(1)
					var body pools.PoolForPatchUpdate
					if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) || !assert.NotNil(t, body.Properties) {
						return
					}
					assert.Nil(t, body.Properties.ComputeModel)
					assert.Nil(t, body.Properties.HighAvailability)
					assert.Nil(t, body.Properties.NodeCount)
					assert.Nil(t, body.Tags)
					if assert.NotNil(t, body.Properties.ParameterGroup) {
						assert.Equal(t, group.ID(), pointer.From(body.Properties.ParameterGroup.ParameterGroupResourceId))
					}
				} else {
					assert.Equal(t, http.MethodGet, r.Method)
				}
				groupId := defaultGroup.ID()
				if test.custom || patches.Load() > 0 {
					groupId = group.ID()
				}
				assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{"properties": map[string]interface{}{
					"provisioningState": "Succeeded", "parameterGroup": map[string]string{"parameterGroupResourceId": groupId, "applyMethod": "Immediate"},
				}}))
			})
			client.Features.SkipImportCheckOnCreateAndAllowOverwritingExistingResources = test.overwrite
			resource := HorizondbPoolParameterGroupAssociationResource{}
			metadata := sdk.NewResourceMetaData(client, resource)
			require.NoError(t, metadata.Encode(&HorizondbPoolParameterGroupAssociationResourceModel{
				HorizondbPoolId: id.ID(), HorizondbParameterGroupId: group.ID(), ApplyMethod: "Immediate",
			}))
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			err := resource.Create().Func(ctx, metadata)
			if test.error != "" {
				require.ErrorContains(t, err, test.error)
			} else {
				require.NoError(t, err)
				require.Equal(t, id.ID(), metadata.ResourceData.Id())
				identity, err := metadata.ResourceData.Identity()
				require.NoError(t, err)
				require.Equal(t, id.PoolName, identity.Get("name"))
			}
			require.Equal(t, test.patches, patches.Load())
		})
	}
}

func TestHorizondbPoolParameterGroupDelete(t *testing.T) {
	id := pools.NewPoolID(horizondbParameterTestSubscription, "example-rg", "example-cluster", "example-pool")
	group := parametergroups.NewProviderParameterGroupID(id.SubscriptionId, id.ResourceGroupName, "example-parameters")
	defaultGroup := defaultparametergroups.NewParameterGroupID(id.SubscriptionId, "eastus2", "default-18")
	for _, test := range []struct {
		name                                                  string
		missing, detached, drift, noDefault, duplicateDefault bool
		patches                                               int32
		error                                                 string
	}{
		{name: "detach", patches: 1},
		{name: "missing pool", missing: true},
		{name: "already default", detached: true},
		{name: "ownership changed", drift: true, error: "refusing to detach another"},
		{name: "no default", noDefault: true, error: "no default parameter group"},
		{name: "ambiguous default", duplicateDefault: true, error: "multiple default parameter groups"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var patches atomic.Int32
			client := newHorizondbParameterTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPatch {
					assert.Equal(t, id.ID(), r.URL.Path)
					patches.Add(1)
					var body pools.PoolForPatchUpdate
					if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) || !assert.NotNil(t, body.Properties) || !assert.NotNil(t, body.Properties.ParameterGroup) {
						return
					}
					assert.Equal(t, defaultGroup.ID(), pointer.From(body.Properties.ParameterGroup.ParameterGroupResourceId))
					assert.Equal(t, pools.ParameterApplyMethodPendingReboot, pointer.From(body.Properties.ParameterGroup.ApplyMethod))
					assert.Nil(t, body.Properties.ComputeModel)
					assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{"properties": map[string]string{"provisioningState": "Succeeded"}}))
					return
				}
				assert.Equal(t, http.MethodGet, r.Method, "detach must never delete or restart a pool")
				switch {
				case r.URL.Path == id.ID():
					if test.missing {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					current := group.ID()
					if test.detached || patches.Load() > 0 {
						current = defaultGroup.ID()
					}
					if test.drift {
						current = parametergroups.NewProviderParameterGroupID(id.SubscriptionId, id.ResourceGroupName, "another").ID()
					}
					assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{"properties": map[string]interface{}{
						"provisioningState": "Succeeded", "parameterGroup": map[string]string{"parameterGroupResourceId": current, "applyMethod": "PendingReboot"},
					}}))
				case strings.HasSuffix(r.URL.Path, "/clusters/"+id.ClusterName):
					assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{"location": "East US 2", "properties": map[string]string{"postgreSqlVersion": "18"}}))
				case strings.HasSuffix(r.URL.Path, "/locations/eastus2/parameterGroups"):
					values := []map[string]interface{}{}
					if !test.noDefault {
						values = append(values, map[string]interface{}{"id": defaultGroup.ID(), "properties": map[string]string{"postgreSqlVersion": "18"}})
					}
					if test.duplicateDefault {
						values = append(values, values[0])
					}
					assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{"value": values}))
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			})
			resource := HorizondbPoolParameterGroupAssociationResource{}
			metadata := sdk.NewResourceMetaData(client, resource)
			require.NoError(t, metadata.Encode(&HorizondbPoolParameterGroupAssociationResourceModel{
				HorizondbPoolId: id.ID(), HorizondbParameterGroupId: group.ID(), ApplyMethod: "PendingReboot",
			}))
			metadata.SetID(id)
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			err := resource.Delete().Func(ctx, metadata)
			if test.error != "" {
				require.ErrorContains(t, err, test.error)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.patches, patches.Load())
		})
	}
}
