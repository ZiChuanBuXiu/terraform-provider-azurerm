package client

import (
	"fmt"

	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/clusters"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/defaultparametergroups"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/endpoints"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/firewallrules"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/parametergroups"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/passwordauthentications"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/pools"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/privatelinkresources"
	"github.com/hashicorp/terraform-provider-azurerm/internal/common"
)

type Client struct {
	ClustersClient                *clusters.ClustersClient
	DefaultParameterGroupsClient  *defaultparametergroups.DefaultParameterGroupsClient
	EndpointsClient               *endpoints.EndpointsClient
	FirewallRulesClient           *firewallrules.FirewallRulesClient
	ParameterGroupsClient         *parametergroups.ParameterGroupsClient
	PasswordAuthenticationsClient *passwordauthentications.PasswordAuthenticationsClient
	PoolsClient                   *pools.PoolsClient
	PrivateLinkResourcesClient    *privatelinkresources.PrivateLinkResourcesClient
}

func NewClient(o *common.ClientOptions) (*Client, error) {
	clustersClient, err := clusters.NewClustersClientWithBaseURI(o.Environment.ResourceManager)
	if err != nil {
		return nil, fmt.Errorf("building Clusters client: %+v", err)
	}
	o.Configure(clustersClient.Client, o.Authorizers.ResourceManager)

	defaultParameterGroupsClient, err := defaultparametergroups.NewDefaultParameterGroupsClientWithBaseURI(o.Environment.ResourceManager)
	if err != nil {
		return nil, fmt.Errorf("building DefaultParameterGroups client: %+v", err)
	}
	o.Configure(defaultParameterGroupsClient.Client, o.Authorizers.ResourceManager)

	endpointsClient, err := endpoints.NewEndpointsClientWithBaseURI(o.Environment.ResourceManager)
	if err != nil {
		return nil, fmt.Errorf("building Endpoints client: %+v", err)
	}
	o.Configure(endpointsClient.Client, o.Authorizers.ResourceManager)

	firewallRulesClient, err := firewallrules.NewFirewallRulesClientWithBaseURI(o.Environment.ResourceManager)
	if err != nil {
		return nil, fmt.Errorf("building FirewallRules client: %+v", err)
	}
	o.Configure(firewallRulesClient.Client, o.Authorizers.ResourceManager)

	parameterGroupsClient, err := parametergroups.NewParameterGroupsClientWithBaseURI(o.Environment.ResourceManager)
	if err != nil {
		return nil, fmt.Errorf("building ParameterGroups client: %+v", err)
	}
	o.Configure(parameterGroupsClient.Client, o.Authorizers.ResourceManager)

	passwordAuthenticationsClient, err := passwordauthentications.NewPasswordAuthenticationsClientWithBaseURI(o.Environment.ResourceManager)
	if err != nil {
		return nil, fmt.Errorf("building PasswordAuthentications client: %+v", err)
	}
	o.Configure(passwordAuthenticationsClient.Client, o.Authorizers.ResourceManager)

	poolsClient, err := pools.NewPoolsClientWithBaseURI(o.Environment.ResourceManager)
	if err != nil {
		return nil, fmt.Errorf("building Pools client: %+v", err)
	}
	o.Configure(poolsClient.Client, o.Authorizers.ResourceManager)

	privateLinkResourcesClient, err := privatelinkresources.NewPrivateLinkResourcesClientWithBaseURI(o.Environment.ResourceManager)
	if err != nil {
		return nil, fmt.Errorf("building PrivateLinkResources client: %+v", err)
	}
	o.Configure(privateLinkResourcesClient.Client, o.Authorizers.ResourceManager)

	return &Client{
		ClustersClient:                clustersClient,
		DefaultParameterGroupsClient:  defaultParameterGroupsClient,
		EndpointsClient:               endpointsClient,
		FirewallRulesClient:           firewallRulesClient,
		ParameterGroupsClient:         parameterGroupsClient,
		PasswordAuthenticationsClient: passwordAuthenticationsClient,
		PoolsClient:                   poolsClient,
		PrivateLinkResourcesClient:    privateLinkResourcesClient,
	}, nil
}
