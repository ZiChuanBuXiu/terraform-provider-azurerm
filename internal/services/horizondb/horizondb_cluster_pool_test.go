// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/clusters"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/pools"
	"github.com/hashicorp/go-azure-sdk/sdk/environments"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	horizondbclient "github.com/hashicorp/terraform-provider-azurerm/internal/services/horizondb/client"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/stretchr/testify/require"
)

func TestDefaultHorizondbClusterPoolParameterGroup(t *testing.T) {
	id := clusters.NewClusterID("00000000-0000-0000-0000-000000000000", "test", "cluster")
	for _, version := range []string{"17", "18"} {
		t.Run(version, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet || req.URL.Path != id.ID() {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				err := json.NewEncoder(w).Encode(clusters.Cluster{
					Location: "East US 2",
					Properties: &clusters.ClusterProperties{
						PostgreSqlVersion: pointer.To(clusters.PostgreSqlVersion(version)),
					},
				})
				if err != nil {
					t.Errorf("encoding cluster response: %v", err)
				}
			}))
			defer server.Close()

			client, err := clusters.NewClustersClientWithBaseURI(environments.ResourceManagerAPI(server.URL))
			require.NoError(t, err)
			client.Client.AuthorizeRequest = nil
			client.Client.DisableRetries = true
			metadata := sdk.ResourceMetaData{
				Client: &clients.Client{
					HorizonDB: &horizondbclient.Client{ClustersClient: client},
				},
			}

			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			groupId, err := defaultHorizondbClusterPoolParameterGroup(ctx, metadata, id)
			require.NoError(t, err)
			require.Equal(t, "/subscriptions/"+id.SubscriptionId+"/providers/Microsoft.HorizonDb/locations/eastus2/parameterGroups/default_pg"+version, groupId)
		})
	}
}

func TestHorizondbClusterPoolComputeType_Schema(t *testing.T) {
	poolSchema := horizondbClusterPoolSchema()
	computeSchema := poolSchema.Elem.(*pluginsdk.Resource).Schema["compute"]
	typeSchema := computeSchema.Elem.(*pluginsdk.Resource).Schema["type"]
	require.True(t, typeSchema.Optional)
	require.False(t, typeSchema.Required)
	require.False(t, typeSchema.Computed)
	require.False(t, typeSchema.ForceNew)
	require.False(t, computeSchema.ForceNew)
	require.False(t, poolSchema.ForceNew)
	require.Equal(t, "Provisioned", typeSchema.Default)

	for _, value := range pools.PossibleValuesForPoolComputeModelType() {
		_, errs := typeSchema.ValidateFunc(value, "pool.0.compute.0.type")
		require.Empty(t, errs)
	}
	for _, value := range []string{"", "provisioned", "Unsupported"} {
		_, errs := typeSchema.ValidateFunc(value, "pool.0.compute.0.type")
		require.NotEmpty(t, errs)
	}
}

func TestHorizondbClusterPoolComputeType_DecodeAndExpand(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		name := "default"
		if explicit {
			name = "explicit"
		}
		t.Run(name, func(t *testing.T) {
			compute := map[string]interface{}{
				"processor": "AMD", "generation": 1, "vcores": 4,
			}
			if explicit {
				compute["type"] = "Provisioned"
			}
			metadata := sdk.NewResourceMetaData(nil, HorizondbClusterResource{})
			metadata.ResourceData = schema.TestResourceDataRaw(t, HorizondbClusterResource{}.Arguments(), map[string]interface{}{
				"pool": []interface{}{map[string]interface{}{
					"name": "primary", "node_count": 1, "high_availability_mode": "Disabled",
					"compute": []interface{}{compute},
				}},
			})
			var state HorizondbClusterResourceModel
			require.NoError(t, metadata.Decode(&state))
			require.Len(t, state.Pool, 1)
			require.Len(t, state.Pool[0].Compute, 1)
			require.Equal(t, HorizondbClusterPoolCompute{
				Type: "Provisioned", Processor: "AMD", Generation: 1, VCores: 4,
			}, state.Pool[0].Compute[0])

			expanded := expandHorizondbClusterPoolCompute(state.Pool[0].Compute[0])
			putJSON, err := json.Marshal(expanded)
			require.NoError(t, err)
			require.JSONEq(t, `{"type":"Provisioned","computeProcessor":"AMD","computeGeneration":1,"vCores":4}`, string(putJSON))

			patch := expandHorizondbClusterPoolUpdate(state.Pool[0])
			patchJSON, err := json.Marshal(patch.Properties.ComputeModel)
			require.NoError(t, err)
			require.JSONEq(t, string(putJSON), string(patchJSON))
			require.Equal(t, int64(1), pointer.From(patch.Properties.NodeCount))
			require.Equal(t, pools.PoolHighAvailabilityModeDisabled, pointer.From(patch.Properties.HighAvailability.Mode))
		})
	}
}

func TestHorizondbClusterPoolComputeType_Flatten(t *testing.T) {
	clusterId := clusters.NewClusterID("00000000-0000-0000-0000-000000000000", "test", "cluster")
	poolId := pools.NewPoolID(clusterId.SubscriptionId, clusterId.ResourceGroupName, clusterId.ClusterName, "primary")
	for _, computeType := range []pools.PoolComputeModelType{pools.PoolComputeModelTypeProvisioned, "", "Unsupported"} {
		t.Run(string(computeType), func(t *testing.T) {
			model := pools.Pool{
				Id: pointer.To(poolId.ID()),
				Properties: &pools.PoolProperties{
					ComputeModel: pools.PoolComputeModel{
						Type: computeType, ComputeProcessor: pools.ComputeProcessorType("AMD"),
						ComputeGeneration: 1, VCores: 4,
					},
					NodeCount:        1,
					HighAvailability: pools.PoolHighAvailability{Mode: pools.PoolHighAvailabilityModeDisabled},
				},
			}
			flattened, err := flattenHorizondbClusterPool(clusterId, &model)
			if computeType != pools.PoolComputeModelTypeProvisioned {
				require.ErrorContains(t, err, "unsupported compute model type")
				return
			}
			require.NoError(t, err)
			require.Equal(t, []HorizondbClusterPool{{
				Name: "primary", Id: poolId.ID(), NodeCount: 1, HighAvailabilityMode: "Disabled",
				Compute: []HorizondbClusterPoolCompute{{
					Type: "Provisioned", Processor: "AMD", Generation: 1, VCores: 4,
				}},
			}}, flattened)

			metadata := sdk.NewResourceMetaData(nil, HorizondbClusterResource{})
			require.NoError(t, metadata.Encode(&HorizondbClusterResourceModel{Pool: flattened}))
			require.Equal(t, "Provisioned", metadata.ResourceData.Get("pool.0.compute.0.type"))
			var state HorizondbClusterResourceModel
			require.NoError(t, metadata.Decode(&state))
			require.Equal(t, flattened, state.Pool)
		})
	}
}
