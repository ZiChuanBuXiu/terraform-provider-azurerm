package horizondb

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/clusters"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/defaultparametergroups"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/pools"
	azureclient "github.com/hashicorp/go-azure-sdk/sdk/client"
	"github.com/hashicorp/go-azure-sdk/sdk/environments"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	horizondbclient "github.com/hashicorp/terraform-provider-azurerm/internal/services/horizondb/client"
	"github.com/stretchr/testify/require"
)

const horizondbTestSubscription = "00000000-0000-0000-0000-000000000000"

func newHorizondbTestClient(t *testing.T, handler http.HandlerFunc) *clients.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(azureclient.SkipPollingDelayHeader, "true")
		handler(w, r)
	}))
	t.Cleanup(server.Close)

	clusterClient, err := clusters.NewClustersClientWithBaseURI(environments.ResourceManagerAPI(server.URL))
	require.NoError(t, err)
	clusterClient.Client.AuthorizeRequest = nil
	clusterClient.Client.DisableRetries = true

	defaultClient, err := defaultparametergroups.NewDefaultParameterGroupsClientWithBaseURI(environments.ResourceManagerAPI(server.URL))
	require.NoError(t, err)
	defaultClient.Client.AuthorizeRequest = nil
	defaultClient.Client.DisableRetries = true

	poolClient, err := pools.NewPoolsClientWithBaseURI(environments.ResourceManagerAPI(server.URL))
	require.NoError(t, err)
	poolClient.Client.AuthorizeRequest = nil
	poolClient.Client.DisableRetries = true

	return &clients.Client{
		Account: &clients.ResourceManagerAccount{SubscriptionId: horizondbTestSubscription},
		HorizonDB: &horizondbclient.Client{
			ClustersClient:               clusterClient,
			DefaultParameterGroupsClient: defaultClient,
			PoolsClient:                  poolClient,
		},
	}
}
