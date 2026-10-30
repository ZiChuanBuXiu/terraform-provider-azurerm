// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/clusters"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/pools"
	"github.com/hashicorp/go-azure-sdk/sdk/environments"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	horizondbclient "github.com/hashicorp/terraform-provider-azurerm/internal/services/horizondb/client"
)

func TestHorizondbClusterReadWritePoolSchema(t *testing.T) {
	arguments := (HorizondbClusterResource{}).Arguments()
	if _, ok := arguments["pool"]; ok {
		t.Error("legacy pool block must not be exposed")
	}
	block := arguments["read_write_pool"]
	if block == nil || !block.Required || block.MinItems != 1 || block.MaxItems != 1 {
		t.Fatal("read_write_pool must be a required singleton block")
	}
	if err := sdk.WrappedResource(HorizondbClusterResource{}).InternalValidate(nil, true); err != nil {
		t.Fatalf("invalid resource schema: %v", err)
	}
}

func TestReadHorizondbClusterPool(t *testing.T) {
	clusterId := clusters.NewClusterID("00000000-0000-0000-0000-000000000000", "test-rg", "test-cluster")
	poolId := pools.NewPoolID(clusterId.SubscriptionId, clusterId.ResourceGroupName, clusterId.ClusterName, "primary")
	foreignSubscription := pools.NewPoolID("11111111-1111-1111-1111-111111111111", clusterId.ResourceGroupName, clusterId.ClusterName, "primary")
	foreignGroup := pools.NewPoolID(clusterId.SubscriptionId, "other-rg", clusterId.ClusterName, "primary")
	foreignCluster := pools.NewPoolID(clusterId.SubscriptionId, clusterId.ResourceGroupName, "other-cluster", "primary")
	otherPool := pools.NewPoolID(clusterId.SubscriptionId, clusterId.ResourceGroupName, clusterId.ClusterName, "replica")

	testCases := []struct {
		name              string
		designation       *string
		missingModel      bool
		missingProperties bool
		status            int
		responseId        string
		emptyProperties   bool
		wantRequests      int32
		wantPool          bool
		errorContains     string
	}{
		{name: "designated pool without enumerating other pools", designation: pointer.To(poolId.ID()), wantRequests: 1, wantPool: true},
		{name: "case insensitive designated ID", designation: pointer.To(strings.ToUpper(poolId.ID())), wantRequests: 1, wantPool: true},
		{name: "missing designation on read", errorContains: "`readWritePool.designatedPoolResourceId` is missing"},
		{name: "empty designation on read", designation: pointer.To(""), errorContains: "`readWritePool.designatedPoolResourceId` is missing"},
		{name: "nil cluster", missingModel: true, errorContains: "cluster properties are missing"},
		{name: "nil cluster properties", missingProperties: true, errorContains: "cluster properties are missing"},
		{name: "malformed designation", designation: pointer.To("invalid"), errorContains: "parsing"},
		{name: "foreign subscription", designation: pointer.To(foreignSubscription.ID()), errorContains: "does not belong"},
		{name: "foreign resource group", designation: pointer.To(foreignGroup.ID()), errorContains: "does not belong"},
		{name: "foreign cluster", designation: pointer.To(foreignCluster.ID()), errorContains: "does not belong"},
		{name: "pool GET failure", designation: pointer.To(poolId.ID()), status: http.StatusForbidden, wantRequests: 1, errorContains: "retrieving designated read/write pool"},
		{name: "designated pool not found", designation: pointer.To(poolId.ID()), status: http.StatusNotFound, wantRequests: 1, errorContains: "retrieving designated read/write pool"},
		{name: "empty pool properties", designation: pointer.To(poolId.ID()), emptyProperties: true, wantRequests: 1, errorContains: "required pool configuration is missing"},
		{name: "different pool returned", designation: pointer.To(poolId.ID()), responseId: otherPool.ID(), wantRequests: 1, errorContains: "does not match"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var requests atomic.Int32
			metadata := horizondbPoolTestMetadata(t, func(w http.ResponseWriter, req *http.Request) {
				requests.Add(1)
				if req.Method != http.MethodGet || !strings.EqualFold(req.URL.Path, poolId.ID()) {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if testCase.status != 0 {
					w.WriteHeader(testCase.status)
				}
				model := horizondbTestPool(poolId)
				if testCase.responseId != "" {
					model.Id = pointer.To(testCase.responseId)
				}
				if testCase.emptyProperties {
					model.Properties = nil
				}
				if err := json.NewEncoder(w).Encode(model); err != nil {
					t.Errorf("encoding pool: %v", err)
				}
			})
			cluster := &clusters.Cluster{Properties: &clusters.ClusterProperties{
				ReadWritePool: clusters.ReadWritePoolProperties{DesignatedPoolResourceId: testCase.designation},
			}}
			if testCase.missingProperties {
				cluster.Properties = nil
			}
			if testCase.missingModel {
				cluster = nil
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			result, err := readHorizondbClusterPool(ctx, metadata, clusterId, cluster)
			if testCase.errorContains != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.errorContains) {
					t.Fatalf("expected error containing %q, got %v", testCase.errorContains, err)
				}
			} else if err != nil {
				t.Fatalf("reading designated pool: %v", err)
			}
			if (result != nil) != testCase.wantPool {
				t.Errorf("expected pool present=%t, got %+v", testCase.wantPool, result)
			}
			if result != nil {
				flattened, err := flattenHorizondbClusterPool(clusterId, result)
				if err != nil || len(flattened) != 1 || flattened[0].Id != poolId.ID() {
					t.Errorf("unexpected flattened pool: %+v, error: %v", flattened, err)
				}
			}
			if actual := requests.Load(); actual != testCase.wantRequests {
				t.Errorf("expected %d requests, got %d", testCase.wantRequests, actual)
			}
		})
	}
}

func TestCreateOrReplaceHorizondbClusterPool(t *testing.T) {
	clusterId := clusters.NewClusterID("00000000-0000-0000-0000-000000000000", "test-rg", "test-cluster")
	poolId := pools.NewPoolID(clusterId.SubscriptionId, clusterId.ResourceGroupName, clusterId.ClusterName, "primary")
	otherPoolId := pools.NewPoolID(clusterId.SubscriptionId, clusterId.ResourceGroupName, clusterId.ClusterName, "replica")
	testCases := []struct {
		name              string
		designation       *string
		create            bool
		missingProperties bool
		clusterStatus     int
		poolStatus        int
		wantPoolGets      int32
		wantPut           int32
		errorContains     string
	}{
		{name: "initial creation with no designation", create: true, wantPoolGets: 1, wantPut: 1},
		{name: "initial creation with empty designation", create: true, designation: pointer.To(""), wantPoolGets: 1, wantPut: 1},
		{name: "update with no designation", errorContains: "`readWritePool.designatedPoolResourceId` is missing"},
		{name: "update with empty designation", designation: pointer.To(""), errorContains: "`readWritePool.designatedPoolResourceId` is missing"},
		{name: "update designated pool", designation: pointer.To(poolId.ID()), wantPoolGets: 2, wantPut: 1},
		{name: "create with existing designated pool", designation: pointer.To(poolId.ID()), create: true, wantPoolGets: 2, wantPut: 1},
		{name: "designation changed to another pool", designation: pointer.To(otherPoolId.ID()), wantPoolGets: 1, errorContains: "remote pool changed"},
		{name: "different designated pool during creation", create: true, designation: pointer.To(otherPoolId.ID()), wantPoolGets: 1, errorContains: "remote pool changed"},
		{name: "cluster GET failure during creation", create: true, clusterStatus: http.StatusForbidden, errorContains: "retrieving"},
		{name: "missing cluster properties during creation", create: true, missingProperties: true, errorContains: "cluster properties are missing"},
		{name: "malformed designation during creation", create: true, designation: pointer.To("invalid"), errorContains: "parsing"},
		{name: "foreign designation during creation", create: true, designation: pointer.To(pools.NewPoolID(clusterId.SubscriptionId, clusterId.ResourceGroupName, "other-cluster", "primary").ID()), errorContains: "does not belong"},
		{name: "designated pool not found during creation", create: true, designation: pointer.To(poolId.ID()), poolStatus: http.StatusNotFound, wantPoolGets: 1, errorContains: "retrieving designated read/write pool"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var poolGets, puts atomic.Int32
			metadata := horizondbPoolTestMetadata(t, func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if req.Method == http.MethodGet && req.URL.Path == clusterId.ID() {
					if testCase.clusterStatus != 0 {
						w.WriteHeader(testCase.clusterStatus)
					}
					model := clusters.Cluster{Properties: &clusters.ClusterProperties{
						ReadWritePool: clusters.ReadWritePoolProperties{DesignatedPoolResourceId: testCase.designation},
					}}
					if testCase.missingProperties {
						model.Properties = nil
					}
					if err := json.NewEncoder(w).Encode(model); err != nil {
						t.Errorf("encoding cluster: %v", err)
					}
					return
				}
				remoteId := poolId
				if pointer.From(testCase.designation) != "" {
					parsed, err := pools.ParsePoolIDInsensitively(*testCase.designation)
					if err != nil {
						t.Errorf("parsing test designation: %v", err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					remoteId = *parsed
				}
				if req.URL.Path != remoteId.ID() {
					t.Errorf("unexpected pool path %s", req.URL.Path)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				switch req.Method {
				case http.MethodGet:
					poolGets.Add(1)
					if testCase.poolStatus != 0 {
						w.WriteHeader(testCase.poolStatus)
					}
				case http.MethodPut:
					puts.Add(1)
					var payload pools.Pool
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Errorf("decoding pool payload: %v", err)
					} else if props := payload.Properties; props == nil {
						t.Error("pool payload properties are missing")
					} else {
						expected := horizondbTestPool(poolId).Properties
						if props.ComputeModel != expected.ComputeModel || props.NodeCount != expected.NodeCount || props.HighAvailability != expected.HighAvailability {
							t.Errorf("unexpected pool payload properties: %+v", props)
						}
						if props.ScheduledMaintenance == nil || props.ScheduledMaintenance.ScheduledMaintenanceProperties().MaintenanceManagementType != pools.ScheduledMaintenanceManagementTypeSystem {
							t.Error("pool payload must use system scheduled maintenance")
						}
					}
				default:
					t.Errorf("unexpected method %s", req.Method)
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				if err := json.NewEncoder(w).Encode(horizondbTestPool(remoteId)); err != nil {
					t.Errorf("encoding pool: %v", err)
				}
			})
			desired, err := flattenHorizondbClusterPool(clusterId, pointer.To(horizondbTestPool(poolId)))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			if testCase.create {
				metadata.ResourceData.MarkNewResource()
			}
			metadata.SetID(clusterId)
			err = createOrReplaceHorizondbClusterPool(ctx, metadata, clusterId, desired[0])
			if testCase.errorContains != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.errorContains) {
					t.Errorf("expected error containing %q, got %v", testCase.errorContains, err)
				}
			} else if err != nil {
				t.Errorf("reconciling pool: %v", err)
			}
			if poolGets.Load() != testCase.wantPoolGets || puts.Load() != testCase.wantPut {
				t.Errorf("expected GET/PUT %d/%d, got %d/%d", testCase.wantPoolGets, testCase.wantPut, poolGets.Load(), puts.Load())
			}
		})
	}
}

func horizondbTestPool(id pools.PoolId) pools.Pool {
	return pools.Pool{
		Id: pointer.To(id.ID()),
		Properties: &pools.PoolProperties{
			ComputeModel: pools.PoolComputeModel{
				Type: pools.PoolComputeModelTypeProvisioned, ComputeProcessor: pools.ComputeProcessorTypeAMD,
				ComputeGeneration: 1, VCores: 4,
			},
			NodeCount:            1,
			HighAvailability:     pools.PoolHighAvailability{Mode: pools.PoolHighAvailabilityModeDisabled},
			ScheduledMaintenance: pools.SystemScheduledMaintenanceProperties{MaintenanceManagementType: pools.ScheduledMaintenanceManagementTypeSystem},
			ProvisioningState:    pointer.To(pools.ProvisioningStateSucceeded),
		},
	}
}

func horizondbPoolTestMetadata(t *testing.T, handler http.HandlerFunc) sdk.ResourceMetaData {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	poolClient, err := pools.NewPoolsClientWithBaseURI(environments.ResourceManagerAPI(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	poolClient.Client.AuthorizeRequest = nil
	poolClient.Client.DisableRetries = true
	clusterClient, err := clusters.NewClustersClientWithBaseURI(environments.ResourceManagerAPI(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	clusterClient.Client.AuthorizeRequest = nil
	clusterClient.Client.DisableRetries = true
	return sdk.NewResourceMetaData(&clients.Client{
		HorizonDB: &horizondbclient.Client{PoolsClient: poolClient, ClustersClient: clusterClient},
	}, HorizondbClusterResource{})
}
