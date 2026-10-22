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
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/clusters"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/defaultparametergroups"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/pools"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func horizondbClusterPoolTestConfig(name string) map[string]interface{} {
	config := map[string]interface{}{
		"name": "example", "resource_group_name": "example-rg", "location": "eastus2",
		"postgresql_version": "18", "backup_retention_days": 7,
	}
	if name != "" {
		config["pool"] = []interface{}{map[string]interface{}{
			"name": name, "node_count": 1, "high_availability_mode": "Disabled",
			"compute": []interface{}{map[string]interface{}{
				"type": "Provisioned", "processor": "AMD", "generation": 1, "vcores": 4,
			}},
		}}
	}
	return config
}

func horizondbClusterPoolTestModel(clusterId clusters.ClusterId, name string) pools.Pool {
	return pools.Pool{
		Id: pointer.To(pools.NewPoolID(clusterId.SubscriptionId, clusterId.ResourceGroupName, clusterId.ClusterName, name).ID()),
		Properties: &pools.PoolProperties{
			ComputeModel: pools.PoolComputeModel{
				Type: pools.PoolComputeModelTypeProvisioned, ComputeProcessor: pools.ComputeProcessorTypeAMD,
				ComputeGeneration: 1, VCores: 4,
			},
			NodeCount: 1, HighAvailability: pools.PoolHighAvailability{Mode: pools.PoolHighAvailabilityModeDisabled},
			ParameterGroup: pools.PoolParameterGroupReference{
				ParameterGroupResourceId: "/subscriptions/" + clusterId.SubscriptionId + "/resourceGroups/example-rg/providers/Microsoft.HorizonDb/parameterGroups/custom",
				ApplyMethod:              pools.ParameterApplyMethodPendingReboot,
			},
			ProvisioningState:    pointer.To(pools.ProvisioningStateSucceeded),
			ScheduledMaintenance: pools.SystemScheduledMaintenanceProperties{},
		},
	}
}

func TestHorizondbClusterPoolSchemaAndDiff(t *testing.T) {
	wrapped := sdk.WrappedResource(HorizondbClusterResource{})
	require.NoError(t, wrapped.InternalValidate(nil, true))
	require.NotContains(t, wrapped.Schema, "read_write_pool_id")
	require.True(t, wrapped.Schema["pool"].Optional)
	require.False(t, wrapped.Schema["pool"].Computed)
	require.Equal(t, 1, wrapped.Schema["pool"].MaxItems)
	fields := wrapped.Schema["pool"].Elem.(*schema.Resource).Schema
	require.True(t, fields["id"].Computed)
	require.False(t, fields["name"].ForceNew)
	require.NotContains(t, fields, "tags")
	for _, name := range []string{"a", "pool-1", strings.Repeat("p", 63)} {
		_, errs := fields["name"].ValidateFunc(name, "pool.name")
		require.Empty(t, errs)
	}
	for _, name := range []string{"", "Upper", "-pool", "pool-", strings.Repeat("p", 64)} {
		_, errs := fields["name"].ValidateFunc(name, "pool.name")
		require.NotEmpty(t, errs)
	}
	for _, nodes := range []int{0, 17} {
		_, errs := fields["node_count"].ValidateFunc(nodes, "pool.node_count")
		require.NotEmpty(t, errs)
	}
	compute := fields["compute"].Elem.(*schema.Resource).Schema
	for _, cores := range []int{1, 193} {
		_, errs := compute["vcores"].ValidateFunc(cores, "pool.compute.vcores")
		require.NotEmpty(t, errs)
	}
	for _, test := range []struct {
		name, old, new, mode, error string
		nodes                       int
	}{
		{name: "add", new: "pool", nodes: 1, mode: "Disabled"},
		{name: "remove", old: "pool"},
		{name: "scale", old: "pool", new: "pool", nodes: 2, mode: "ZoneRedundant"},
		{name: "rename", old: "pool", new: "other", nodes: 1, mode: "Disabled", error: "changing `pool.name`"},
		{name: "name collision", new: "example", nodes: 1, mode: "Disabled", error: "must differ"},
		{name: "single node HA", new: "pool", nodes: 1, mode: "AnyZone", error: "must be `Disabled`"},
		{name: "multi node no HA", new: "pool", nodes: 2, mode: "Disabled", error: "must be `Disabled`"},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := schema.TestResourceDataRaw(t, wrapped.Schema, horizondbClusterPoolTestConfig(test.old))
			data.SetId(clusters.NewClusterID(horizondbParameterTestSubscription, "example-rg", "example").ID())
			config := horizondbClusterPoolTestConfig(test.new)
			if test.new != "" {
				pool := config["pool"].([]interface{})[0].(map[string]interface{})
				pool["node_count"], pool["high_availability_mode"] = test.nodes, test.mode
			}
			prior := data.State()
			configData := schema.TestResourceDataRaw(t, wrapped.Schema, config)
			configData.SetId(data.Id())
			raw, err := configData.State().AttrsAsObjectValue(wrapped.CoreConfigSchema().ImpliedType())
			require.NoError(t, err)
			prior.RawConfig = raw
			diff, err := wrapped.Diff(t.Context(), prior, terraform.NewResourceConfigRaw(config), &clients.Client{})
			if test.error != "" {
				require.ErrorContains(t, err, test.error)
			} else {
				require.NoError(t, err)
				require.NotNil(t, diff)
				require.False(t, diff.RequiresNew(), "Pool changes must not replace the Cluster")
			}
		})
	}
}

func TestHorizondbClusterPoolUnknownConfig(t *testing.T) {
	poolType := cty.Object(map[string]cty.Type{
		"name": cty.String, "node_count": cty.Number, "high_availability_mode": cty.String,
	})
	for _, pool := range []cty.Value{
		cty.NullVal(cty.List(poolType)),
		cty.UnknownVal(cty.List(poolType)),
		cty.ListVal([]cty.Value{cty.UnknownVal(poolType)}),
		cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
			"name": cty.UnknownVal(cty.String), "node_count": cty.UnknownVal(cty.Number),
			"high_availability_mode": cty.StringVal("ZoneRedundant"),
		})}),
	} {
		require.NoError(t, validateHorizondbClusterPoolConfig(cty.ObjectVal(map[string]cty.Value{
			"name": cty.StringVal("example"), "pool": pool,
		})))
	}
}

func TestHorizondbClusterPoolRead(t *testing.T) {
	id := clusters.NewClusterID(horizondbParameterTestSubscription, "example-rg", "example")
	for _, scenario := range []string{"import", "missing pool", "multiple pools", "wrong parent", "missing properties", "malformed ID", "forbidden", "missing cluster"} {
		t.Run(scenario, func(t *testing.T) {
			model := horizondbClusterPoolTestModel(id, "pool")
			if scenario == "wrong parent" {
				model.Id = pointer.To(pools.NewPoolID(id.SubscriptionId, id.ResourceGroupName, "other", "pool").ID())
			}
			if scenario == "malformed ID" {
				model.Id = pointer.To("invalid")
			}
			if scenario == "missing properties" {
				model.Properties = nil
			}
			client := newHorizondbParameterTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				if r.URL.Path == id.ID() {
					if scenario == "missing cluster" {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					assert.NoError(t, json.NewEncoder(w).Encode(clusters.Cluster{
						Location: "eastus2", Properties: &clusters.ClusterProperties{PostgreSqlVersion: pointer.To(clusters.PostgreSqlVersion("18"))},
					}))
					return
				}
				assert.Equal(t, id.ID()+"/pools", r.URL.Path)
				items := []pools.Pool{model}
				switch scenario {
				case "missing pool":
					items = []pools.Pool{}
				case "multiple pools":
					items = append(items, model)
				case "forbidden":
					w.WriteHeader(http.StatusForbidden)
					return
				}
				assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{"value": items}))
			})
			metadata := sdk.NewResourceMetaData(client, HorizondbClusterResource{})
			metadata.ResourceData.SetId(id.ID())
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			err := (HorizondbClusterResource{}).Read().Func(ctx, metadata)
			switch scenario {
			case "import":
				require.NoError(t, err)
				var state HorizondbClusterResourceModel
				require.NoError(t, metadata.Decode(&state))
				require.Len(t, state.Pool, 1, "import must discover the Pool even without a designation or configured block")
				require.Equal(t, *model.Id, state.Pool[0].Id)
				require.Equal(t, int64(4), state.Pool[0].Compute[0].VCores)
			case "missing pool":
				require.NoError(t, err)
				require.Empty(t, metadata.ResourceData.Get("pool"))
				require.Equal(t, id.ID(), metadata.ResourceData.Id())
			case "missing cluster":
				require.NoError(t, err)
				require.Empty(t, metadata.ResourceData.Id())
			default:
				require.Error(t, err)
			}
		})
	}
}

func TestHorizondbClusterPoolUpdate(t *testing.T) {
	id := clusters.NewClusterID(horizondbParameterTestSubscription, "example-rg", "example")
	for _, scenario := range []string{"add", "scale and cluster tags", "remove", "already removed", "rename guard", "remote changed", "patch failure"} {
		t.Run(scenario, func(t *testing.T) {
			var poolPuts, poolPatches, poolDeletes, clusterPatches atomic.Int32
			model := horizondbClusterPoolTestModel(id, "pool")
			if scenario == "remote changed" {
				model = horizondbClusterPoolTestModel(id, "other")
			}
			client := newHorizondbParameterTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "2026-10-01-preview", r.URL.Query().Get("api-version"))
				switch {
				case r.URL.Path == id.ID():
					if r.Method == http.MethodPatch {
						clusterPatches.Add(1)
						var payload map[string]interface{}
						assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
						assert.Equal(t, map[string]interface{}{"environment": "updated"}, payload["tags"])
						assert.NotContains(t, payload["properties"], "pool")
					}
					assert.NoError(t, json.NewEncoder(w).Encode(clusters.Cluster{
						Location: "eastus2", Properties: &clusters.ClusterProperties{
							PostgreSqlVersion: pointer.To(clusters.PostgreSqlVersion("18")), ProvisioningState: clusters.ProvisioningStateSucceeded,
						},
					}))
				case strings.Contains(r.URL.Path, "/locations/"):
					groupId := defaultparametergroups.NewParameterGroupID(id.SubscriptionId, "eastus2", "default18")
					assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{"value": []interface{}{
						map[string]interface{}{"id": groupId.ID(), "properties": map[string]interface{}{"postgreSqlVersion": "18"}},
					}}))
				case r.URL.Path == id.ID()+"/pools":
					items := []pools.Pool{model}
					if scenario == "add" || scenario == "already removed" {
						items = []pools.Pool{}
					}
					assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{"value": items}))
				case r.URL.Path == id.ID()+"/pools/pool":
					switch r.Method {
					case http.MethodPut:
						poolPuts.Add(1)
						var body pools.Pool
						assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
						if assert.NotNil(t, body.Properties) {
							assert.Equal(t, pools.ParameterApplyMethodImmediate, body.Properties.ParameterGroup.ApplyMethod)
							assert.Contains(t, body.Properties.ParameterGroup.ParameterGroupResourceId, "/locations/eastus2/parameterGroups/default18")
							assert.Equal(t, int64(1), body.Properties.NodeCount)
							assert.Equal(t, pools.PoolHighAvailabilityModeDisabled, body.Properties.HighAvailability.Mode)
							assert.Equal(t, pools.ScheduledMaintenanceManagementTypeSystem, body.Properties.ScheduledMaintenance.ScheduledMaintenanceProperties().MaintenanceManagementType)
						}
					case http.MethodPatch:
						poolPatches.Add(1)
						var body map[string]interface{}
						assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
						properties := body["properties"].(map[string]interface{})
						assert.NotContains(t, properties, "parameterGroup")
						assert.NotContains(t, properties, "provisioningState")
						assert.NotContains(t, properties, "state")
						assert.Equal(t, float64(2), properties["nodeCount"])
						assert.Equal(t, map[string]interface{}{"mode": "ZoneRedundant"}, properties["highAvailability"])
						assert.NotContains(t, body, "tags")
						if scenario == "scale and cluster tags" {
							assert.Equal(t, int32(1), clusterPatches.Load())
						}
						if scenario == "patch failure" {
							w.WriteHeader(http.StatusBadRequest)
							return
						}
					case http.MethodDelete:
						poolDeletes.Add(1)
						w.WriteHeader(http.StatusNoContent)
						return
					case http.MethodGet:
						if poolDeletes.Load() > 0 {
							w.WriteHeader(http.StatusNotFound)
							return
						}
					default:
						t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					}
					assert.NoError(t, json.NewEncoder(w).Encode(model))
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			})
			wrapped := sdk.WrappedResource(HorizondbClusterResource{})
			before, after := horizondbClusterPoolTestConfig("pool"), horizondbClusterPoolTestConfig("pool")
			switch scenario {
			case "add":
				before = horizondbClusterPoolTestConfig("")
			case "remove", "already removed", "remote changed":
				after = horizondbClusterPoolTestConfig("")
			case "rename guard":
				after = horizondbClusterPoolTestConfig("new")
				after["tags"] = map[string]interface{}{"environment": "updated"}
			default:
				pool := after["pool"].([]interface{})[0].(map[string]interface{})
				pool["node_count"], pool["high_availability_mode"] = 2, "ZoneRedundant"
				if scenario == "scale and cluster tags" {
					after["tags"] = map[string]interface{}{"environment": "updated"}
				}
			}
			data := schema.TestResourceDataRaw(t, wrapped.Schema, before)
			data.SetId(id.ID())
			wrapped.CustomizeDiff = nil
			diff, err := wrapped.Diff(t.Context(), data.State(), terraform.NewResourceConfigRaw(after), client)
			require.NoError(t, err)
			updated, err := schema.InternalMap(wrapped.Schema).Data(data.State(), diff)
			require.NoError(t, err)
			metadata := sdk.NewResourceMetaData(client, HorizondbClusterResource{})
			metadata.ResourceData = updated
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			err = (HorizondbClusterResource{}).Update().Func(ctx, metadata)
			switch scenario {
			case "rename guard":
				require.ErrorContains(t, err, "changing `pool.name`")
				require.Zero(t, clusterPatches.Load())
			case "remote changed":
				require.ErrorContains(t, err, "remote pool changed")
				require.Zero(t, poolDeletes.Load())
			case "patch failure":
				require.ErrorContains(t, err, "updating")
			default:
				require.NoError(t, err)
			}
			if scenario == "add" {
				require.Equal(t, int32(1), poolPuts.Load())
			} else {
				require.Zero(t, poolPuts.Load())
			}
			if scenario == "scale and cluster tags" || scenario == "patch failure" {
				require.Equal(t, int32(1), poolPatches.Load())
			} else {
				require.Zero(t, poolPatches.Load())
			}
			if scenario == "remove" {
				require.Equal(t, int32(1), poolDeletes.Load())
			}
			require.Equal(t, id.ID(), metadata.ResourceData.Id())
		})
	}
}

func TestHorizondbClusterPoolCreateAndRestore(t *testing.T) {
	id := clusters.NewClusterID(horizondbParameterTestSubscription, "example-rg", "example")
	for _, scenario := range []string{"create", "restore", "pool failure", "overwrite", "overwrite without pool"} {
		t.Run(scenario, func(t *testing.T) {
			var clusterPuts, poolPuts, poolPatches, poolDeletes atomic.Int32
			pool := horizondbClusterPoolTestModel(id, "pool")
			client := newHorizondbParameterTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == id.ID():
					if r.Method == http.MethodPut {
						clusterPuts.Add(1)
						var body clusters.Cluster
						assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
						if scenario == "restore" {
							assert.Equal(t, clusters.ClusterCreateModePointInTimeRestore, pointer.From(body.Properties.CreationMode))
							assert.Nil(t, body.Properties.PostgreSqlVersion)
						} else {
							assert.Equal(t, clusters.ClusterCreateModeCreate, pointer.From(body.Properties.CreationMode))
						}
					}
					if clusterPuts.Load() == 0 {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					assert.NoError(t, json.NewEncoder(w).Encode(clusters.Cluster{
						Location: "eastus2", Properties: &clusters.ClusterProperties{
							PostgreSqlVersion: pointer.To(clusters.PostgreSqlVersion("17")), ProvisioningState: clusters.ProvisioningStateSucceeded,
						},
					}))
				case r.URL.Path == id.ID()+"/pools":
					items := []pools.Pool{}
					if strings.HasPrefix(scenario, "overwrite") {
						items = append(items, pool)
					}
					assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{"value": items}))
				case strings.Contains(r.URL.Path, "/locations/"):
					assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{"value": []interface{}{
						map[string]interface{}{
							"id":         defaultparametergroups.NewParameterGroupID(id.SubscriptionId, "eastus2", "default17").ID(),
							"properties": map[string]interface{}{"postgreSqlVersion": "17"},
						},
					}}))
				case r.URL.Path == *pool.Id:
					assert.Equal(t, int32(1), clusterPuts.Load())
					switch r.Method {
					case http.MethodPut:
						poolPuts.Add(1)
						var body pools.Pool
						assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
						assert.Nil(t, body.Tags)
						assert.Contains(t, body.Properties.ParameterGroup.ParameterGroupResourceId, "default17")
						if scenario == "pool failure" {
							w.WriteHeader(http.StatusBadRequest)
							return
						}
					case http.MethodPatch:
						poolPatches.Add(1)
						var body map[string]interface{}
						assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
						assert.NotContains(t, body["properties"], "parameterGroup", "overwrite must not reset an existing custom association")
					case http.MethodGet:
						if poolDeletes.Load() > 0 {
							w.WriteHeader(http.StatusNotFound)
							return
						}
					case http.MethodDelete:
						poolDeletes.Add(1)
						w.WriteHeader(http.StatusNoContent)
						return
					default:
						t.Errorf("unexpected method %s", r.Method)
					}
					assert.NoError(t, json.NewEncoder(w).Encode(pool))
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			})
			client.Features.SkipImportCheckOnCreateAndAllowOverwritingExistingResources = strings.HasPrefix(scenario, "overwrite")
			metadata := sdk.NewResourceMetaData(client, HorizondbClusterResource{})
			require.NoError(t, metadata.ResourceData.Set("create_mode", (HorizondbClusterResource{}).Arguments()["create_mode"].Default))
			config := horizondbClusterPoolTestConfig("pool")
			if scenario == "overwrite without pool" {
				config = horizondbClusterPoolTestConfig("")
			}
			if scenario == "restore" {
				delete(config, "postgresql_version")
				config["create_mode"] = "PointInTimeRestore"
				config["restore"] = []interface{}{map[string]interface{}{
					"source_horizondb_cluster_id": clusters.NewClusterID(id.SubscriptionId, "example-rg", "source").ID(),
					"point_in_time_utc":           "2026-10-01T00:00:00Z",
				}}
			}
			for key, value := range config {
				require.NoError(t, metadata.ResourceData.Set(key, value))
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			err := (HorizondbClusterResource{}).Create().Func(ctx, metadata)
			if scenario == "pool failure" {
				require.ErrorContains(t, err, "creating")
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, id.ID(), metadata.ResourceData.Id(), "Pool failure must not lose the already-created Cluster")
			identity, err := metadata.ResourceData.Identity()
			require.NoError(t, err)
			require.Equal(t, id.ClusterName, identity.Get("name"))
			if scenario == "overwrite" {
				require.Zero(t, poolPuts.Load())
				require.Equal(t, int32(1), poolPatches.Load())
			} else if scenario == "overwrite without pool" {
				require.Zero(t, poolPuts.Load())
				require.Equal(t, int32(1), poolDeletes.Load())
			} else {
				require.Equal(t, int32(1), poolPuts.Load())
			}
		})
	}
}
