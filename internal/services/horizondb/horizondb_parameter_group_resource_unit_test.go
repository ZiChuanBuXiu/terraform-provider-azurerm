// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/clusters"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/defaultparametergroups"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/parametergroups"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/pools"
	azureclient "github.com/hashicorp/go-azure-sdk/sdk/client"
	"github.com/hashicorp/go-azure-sdk/sdk/environments"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	horizondbclient "github.com/hashicorp/terraform-provider-azurerm/internal/services/horizondb/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const horizondbParameterTestSubscription = "00000000-0000-0000-0000-000000000000"

func newHorizondbParameterTestClient(t *testing.T, handler http.HandlerFunc) *clients.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(azureclient.SkipPollingDelayHeader, "true")
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	groupClient, err := parametergroups.NewParameterGroupsClientWithBaseURI(environments.ResourceManagerAPI(server.URL))
	require.NoError(t, err)
	groupClient.Client.AuthorizeRequest = nil
	groupClient.Client.DisableRetries = true
	poolClient, err := pools.NewPoolsClientWithBaseURI(environments.ResourceManagerAPI(server.URL))
	require.NoError(t, err)
	poolClient.Client.AuthorizeRequest = nil
	poolClient.Client.DisableRetries = true
	clusterClient, err := clusters.NewClustersClientWithBaseURI(environments.ResourceManagerAPI(server.URL))
	require.NoError(t, err)
	clusterClient.Client.AuthorizeRequest = nil
	clusterClient.Client.DisableRetries = true
	defaultClient, err := defaultparametergroups.NewDefaultParameterGroupsClientWithBaseURI(environments.ResourceManagerAPI(server.URL))
	require.NoError(t, err)
	defaultClient.Client.AuthorizeRequest = nil
	defaultClient.Client.DisableRetries = true
	return &clients.Client{
		Account: &clients.ResourceManagerAccount{SubscriptionId: horizondbParameterTestSubscription},
		HorizonDB: &horizondbclient.Client{
			ParameterGroupsClient: groupClient, PoolsClient: poolClient, ClustersClient: clusterClient,
			DefaultParameterGroupsClient: defaultClient,
		},
	}
}

func TestHorizondbParameterGroupSchema(t *testing.T) {
	for _, resource := range []sdk.Resource{HorizondbParameterGroupResource{}, HorizondbPoolParameterGroupAssociationResource{}} {
		wrapped := sdk.WrappedResource(resource)
		require.NoError(t, wrapped.InternalValidate(nil, true))
		require.NotNil(t, wrapped.Identity)
	}
	name := HorizondbParameterGroupResource{}.Arguments()["name"]
	for _, value := range []string{"ab", "Test-1", strings.Repeat("x", 63)} {
		_, errs := name.ValidateFunc(value, "name")
		require.Empty(t, errs)
	}
	for _, value := range []string{"a", "-ab", "ab-", "a_b", strings.Repeat("x", 64)} {
		_, errs := name.ValidateFunc(value, "name")
		require.NotEmpty(t, errs)
	}
}

func TestHorizondbParameterOverrides(t *testing.T) {
	expanded := expandHorizondbParameters(map[string]string{"xmlbinary": "base64", "application_name": ""})
	require.Equal(t, "application_name", *expanded[0].Name)
	require.Equal(t, "", *expanded[0].Value)
	body, err := json.Marshal(expanded)
	require.NoError(t, err)
	require.JSONEq(t, `[{"name":"application_name","value":""},{"name":"xmlbinary","value":"base64"}]`, string(body))
	parameters := []parametergroups.ParameterProperties{
		{Name: pointer.To("application_name"), Value: pointer.To(""), DefaultValue: pointer.To(""), ValueSource: pointer.To(parametergroups.ParameterValueSourceUserOverride)},
		{Name: pointer.To("xmlbinary"), Value: pointer.To("base64"), DefaultValue: pointer.To("base64"), ValueSource: pointer.To(parametergroups.ParameterValueSourceUserOverride)},
		{Name: pointer.To("autovacuum"), Value: pointer.To("on"), ValueSource: pointer.To(parametergroups.ParameterValueSourceSystemDefault)},
	}
	overrides, err := flattenHorizondbParameterOverrides(parameters)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"application_name": "", "xmlbinary": "base64"}, overrides)
	for _, invalid := range []parametergroups.ParameterProperties{
		{},
		{Name: pointer.To("unknown")},
		{Name: pointer.To("unknown"), ValueSource: pointer.To(parametergroups.ParameterValueSource("Future"))},
		{Name: pointer.To("missing-value"), ValueSource: pointer.To(parametergroups.ParameterValueSourceUserOverride)},
	} {
		_, err := flattenHorizondbParameterOverrides([]parametergroups.ParameterProperties{invalid})
		require.Error(t, err)
	}
	_, err = flattenHorizondbParameterOverrides(append(parameters, parameters[0]))
	require.ErrorContains(t, err, "duplicate")
	require.NoError(t, validateHorizondbParameterChanges(map[string]interface{}{"a": "1"}, map[string]interface{}{"a": "2", "b": ""}))
	require.ErrorContains(t, validateHorizondbParameterChanges(map[string]interface{}{"a": "1"}, map[string]interface{}{}), "removing parameter overrides")
}

func horizondbParameterTestConfig(parameters map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"name": "example-parameters", "resource_group_name": "example-rg", "location": "eastus2",
		"postgresql_version": "18", "parameters": parameters,
	}
}

func TestHorizondbParameterGroupRemovalDiff(t *testing.T) {
	wrapped := sdk.WrappedResource(HorizondbParameterGroupResource{})
	for _, test := range []struct {
		name           string
		parameters     map[string]interface{}
		replace, fails bool
	}{
		{name: "change value", parameters: map[string]interface{}{"application_name": "new"}},
		{name: "add override", parameters: map[string]interface{}{"application_name": "old", "xmlbinary": "base64"}},
		{name: "remove override", parameters: map[string]interface{}{}, fails: true},
		{name: "replacement may omit override", parameters: map[string]interface{}{}, replace: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := schema.TestResourceDataRaw(t, wrapped.Schema, horizondbParameterTestConfig(map[string]interface{}{"application_name": "old"}))
			data.SetId(parametergroups.NewProviderParameterGroupID(horizondbParameterTestSubscription, "example-rg", "example-parameters").ID())
			config := horizondbParameterTestConfig(test.parameters)
			if test.replace {
				config["name"] = "other-parameters"
			}
			diff, err := wrapped.Diff(t.Context(), data.State(), terraform.NewResourceConfigRaw(config), &clients.Client{})
			if test.fails {
				require.ErrorContains(t, err, "removing parameter overrides")
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.replace, diff.RequiresNew())
		})
	}
}

func TestHorizondbParameterGroupCreate(t *testing.T) {
	for _, test := range []struct {
		name      string
		status    int
		overwrite bool
		puts      int32
		error     string
	}{
		{name: "create", status: http.StatusNotFound, puts: 1},
		{name: "requires import", status: http.StatusOK, error: "needs to be imported"},
		{name: "overwrite", status: http.StatusOK, overwrite: true, puts: 1},
		{name: "forbidden", status: http.StatusForbidden, error: "checking for presence"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var puts atomic.Int32
			client := newHorizondbParameterTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					if puts.Load() == 0 {
						w.WriteHeader(test.status)
					}
				case http.MethodPut:
					puts.Add(1)
					var body map[string]interface{}
					if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
						return
					}
					properties := body["properties"].(map[string]interface{})
					assert.NotContains(t, properties, "provisioningState")
					assert.Equal(t, "18", properties["postgreSqlVersion"])
				default:
					t.Errorf("unexpected method %s", r.Method)
				}
				assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{"properties": map[string]string{"provisioningState": "Succeeded"}}))
			})
			client.Features.SkipImportCheckOnCreateAndAllowOverwritingExistingResources = test.overwrite
			metadata := sdk.NewResourceMetaData(client, HorizondbParameterGroupResource{})
			require.NoError(t, metadata.Encode(&HorizondbParameterGroupResourceModel{
				Name: "example-parameters", ResourceGroupName: "example-rg", Location: "eastus2",
				PostgresqlVersion: "18", Parameters: map[string]string{"application_name": ""},
			}))
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			err := (HorizondbParameterGroupResource{}).Create().Func(ctx, metadata)
			if test.error != "" {
				require.ErrorContains(t, err, test.error)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, metadata.ResourceData.Id())
				identity, err := metadata.ResourceData.Identity()
				require.NoError(t, err)
				require.Equal(t, "example-parameters", identity.Get("name"))
			}
			require.Equal(t, test.puts, puts.Load())
		})
	}
}

func TestHorizondbParameterGroupUpdate(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(fmt.Sprintf("remove_%t", remove), func(t *testing.T) {
			var patches atomic.Int32
			client := newHorizondbParameterTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPatch {
					patches.Add(1)
					var body map[string]interface{}
					if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
						return
					}
					properties := body["properties"].(map[string]interface{})
					assert.Equal(t, "", properties["description"])
					assert.NotContains(t, properties, "postgreSqlVersion")
					assert.NotContains(t, properties, "provisioningState")
					assert.Contains(t, body, "tags")
				}
				assert.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{"properties": map[string]string{"provisioningState": "Succeeded"}}))
			})
			wrapped := sdk.WrappedResource(HorizondbParameterGroupResource{})
			before := horizondbParameterTestConfig(map[string]interface{}{"application_name": "before"})
			before["description"] = "remove me"
			before["tags"] = map[string]interface{}{"environment": "test"}
			data := schema.TestResourceDataRaw(t, wrapped.Schema, before)
			data.SetId(parametergroups.NewProviderParameterGroupID(horizondbParameterTestSubscription, "example-rg", "example-parameters").ID())
			after := horizondbParameterTestConfig(map[string]interface{}{"application_name": "after"})
			if remove {
				after["parameters"] = map[string]interface{}{}
			}
			// Exercise the apply-time guard independently of CustomizeDiff.
			wrapped.CustomizeDiff = nil
			diff, err := wrapped.Diff(t.Context(), data.State(), terraform.NewResourceConfigRaw(after), client)
			require.NoError(t, err)
			updated, err := schema.InternalMap(wrapped.Schema).Data(data.State(), diff)
			require.NoError(t, err)
			metadata := sdk.NewResourceMetaData(client, HorizondbParameterGroupResource{})
			metadata.ResourceData = updated
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			err = (HorizondbParameterGroupResource{}).Update().Func(ctx, metadata)
			if remove {
				require.ErrorContains(t, err, "removing parameter overrides")
				require.Zero(t, patches.Load(), "invalid removals must not partially apply other changes")
			} else {
				require.NoError(t, err)
				require.Equal(t, int32(1), patches.Load())
			}
		})
	}
}

func TestHorizondbParameterGroupConnections(t *testing.T) {
	id := parametergroups.NewProviderParameterGroupID(horizondbParameterTestSubscription, "example-rg", "example-parameters")
	for _, test := range []struct {
		name                      string
		empty, invalid, forbidden bool
		count                     int
	}{
		{name: "pages", count: 2}, {name: "empty", empty: true}, {name: "invalid ID", invalid: true}, {name: "forbidden", forbidden: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := newHorizondbParameterTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if test.forbidden {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				values := []map[string]string{}
				body := map[string]interface{}{"value": values}
				if !test.empty {
					name := "pool-one"
					if r.URL.Query().Get("page") == "2" {
						name = "pool-two"
					} else {
						body["nextLink"] = "http://" + r.Host + r.URL.Path + "?api-version=2026-10-01-preview&page=2"
					}
					poolId := pools.NewPoolID(horizondbParameterTestSubscription, "example-rg", "example-cluster", name).ID()
					if test.invalid {
						poolId = "invalid"
					}
					body["value"] = []map[string]string{{"poolResourceId": strings.ReplaceAll(poolId, "resourceGroups", "resourcegroups")}}
				}
				assert.NoError(t, json.NewEncoder(w).Encode(body))
			})
			source := HorizondbParameterGroupConnectionsDataSource{}
			fields := source.Arguments()
			for k, v := range source.Attributes() {
				fields[k] = v
			}
			metadata := sdk.NewResourceMetaData(client, HorizondbParameterGroupResource{})
			metadata.ResourceData = schema.TestResourceDataRaw(t, fields, map[string]interface{}{"horizondb_parameter_group_id": id.ID()})
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			err := source.Read().Func(ctx, metadata)
			if test.invalid || test.forbidden {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.count, metadata.ResourceData.Get("horizondb_pool_ids").(*schema.Set).Len())
			require.Equal(t, id.ID(), metadata.ResourceData.Id())
		})
	}
}
