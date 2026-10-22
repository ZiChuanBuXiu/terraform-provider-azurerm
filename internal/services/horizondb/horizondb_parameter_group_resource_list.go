// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb

import (
	"context"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonids"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/parametergroups"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

type HorizondbParameterGroupListResource struct{}

var _ sdk.FrameworkListWrappedResource = HorizondbParameterGroupListResource{}

func (HorizondbParameterGroupListResource) ResourceFunc() *pluginsdk.Resource {
	return sdk.WrappedResource(HorizondbParameterGroupResource{})
}
func (HorizondbParameterGroupListResource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = HorizondbParameterGroupResource{}.ResourceType()
}
func (HorizondbParameterGroupListResource) List(ctx context.Context, request list.ListRequest, stream *list.ListResultsStream, metadata sdk.ResourceMetadata) {
	var config sdk.DefaultListModel
	if diags := request.Config.Get(ctx, &config); diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	subscriptionId := metadata.SubscriptionId
	if !config.SubscriptionId.IsNull() {
		subscriptionId = config.SubscriptionId.ValueString()
	}
	client := metadata.Client.HorizonDB.ParameterGroupsClient
	var items []parametergroups.ParameterGroup
	if !config.ResourceGroupName.IsNull() {
		resp, err := client.ListByResourceGroupComplete(ctx, commonids.NewResourceGroupID(subscriptionId, config.ResourceGroupName.ValueString()))
		if err != nil {
			sdk.SetResponseErrorDiagnostic(stream, "listing HorizonDB Parameter Groups", err)
			return
		}
		items = resp.Items
	} else {
		resp, err := client.ListBySubscriptionComplete(ctx, commonids.NewSubscriptionID(subscriptionId))
		if err != nil {
			sdk.SetResponseErrorDiagnostic(stream, "listing HorizonDB Parameter Groups", err)
			return
		}
		items = resp.Items
	}
	stream.Results = func(push func(list.ListResult) bool) {
		for _, item := range items {
			result := request.NewListResult(ctx)
			id, err := parametergroups.ParseProviderParameterGroupIDInsensitively(pointer.From(item.Id))
			if err != nil {
				sdk.SetErrorDiagnosticAndPushListResult(result, push, "parsing HorizonDB Parameter Group ID", err)
				return
			}
			result.DisplayName = id.ParameterGroupName
			resourceMetadata := sdk.NewResourceMetaData(metadata.Client, HorizondbParameterGroupResource{})
			if err := (HorizondbParameterGroupResource{}).flatten(resourceMetadata, id, &item); err != nil {
				sdk.SetErrorDiagnosticAndPushListResult(result, push, "encoding HorizonDB Parameter Group", err)
				return
			}
			sdk.EncodeListResult(ctx, resourceMetadata.ResourceData, &result)
			if !push(result) || result.Diagnostics.HasError() {
				return
			}
		}
	}
}
