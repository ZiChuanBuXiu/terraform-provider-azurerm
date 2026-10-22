// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb

import (
	"context"

	"github.com/hashicorp/go-azure-helpers/framework/typehelpers"
	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/pools"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

type HorizondbPoolParameterGroupAssociationListResource struct{}
type HorizondbPoolParameterGroupAssociationListModel struct {
	HorizondbClusterId types.String `tfsdk:"horizondb_cluster_id"`
}

var _ sdk.FrameworkListWrappedResource = HorizondbPoolParameterGroupAssociationListResource{}

func (HorizondbPoolParameterGroupAssociationListResource) ResourceFunc() *pluginsdk.Resource {
	return sdk.WrappedResource(HorizondbPoolParameterGroupAssociationResource{})
}
func (HorizondbPoolParameterGroupAssociationListResource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = HorizondbPoolParameterGroupAssociationResource{}.ResourceType()
}
func (HorizondbPoolParameterGroupAssociationListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, response *list.ListResourceSchemaResponse) {
	response.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"horizondb_cluster_id": schema.StringAttribute{
			Required: true, Validators: []validator.String{typehelpers.WrappedStringValidator{Func: pools.ValidateClusterID}},
		},
	}}
}
func (HorizondbPoolParameterGroupAssociationListResource) List(ctx context.Context, request list.ListRequest, stream *list.ListResultsStream, metadata sdk.ResourceMetadata) {
	var config HorizondbPoolParameterGroupAssociationListModel
	if diags := request.Config.Get(ctx, &config); diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	id, err := pools.ParseClusterID(config.HorizondbClusterId.ValueString())
	if err != nil {
		sdk.SetResponseErrorDiagnostic(stream, "parsing HorizonDB Cluster ID", err)
		return
	}
	resp, err := metadata.Client.HorizonDB.PoolsClient.ListComplete(ctx, *id)
	if err != nil {
		sdk.SetResponseErrorDiagnostic(stream, "listing HorizonDB Pool associations", err)
		return
	}
	stream.Results = func(push func(list.ListResult) bool) {
		for _, item := range resp.Items {
			result := request.NewListResult(ctx)
			_, custom, err := horizondbPoolParameterGroup(&item)
			if err != nil {
				sdk.SetErrorDiagnosticAndPushListResult(result, push, "reading HorizonDB Pool association", err)
				return
			}
			if !custom {
				continue
			}
			poolId, err := pools.ParsePoolIDInsensitively(pointer.From(item.Id))
			if err != nil {
				sdk.SetErrorDiagnosticAndPushListResult(result, push, "parsing HorizonDB Pool ID", err)
				return
			}
			result.DisplayName = poolId.PoolName
			resourceMetadata := sdk.NewResourceMetaData(metadata.Client, HorizondbPoolParameterGroupAssociationResource{})
			if err := (HorizondbPoolParameterGroupAssociationResource{}).flatten(resourceMetadata, poolId, &item); err != nil {
				sdk.SetErrorDiagnosticAndPushListResult(result, push, "encoding HorizonDB Pool association", err)
				return
			}
			sdk.EncodeListResult(ctx, resourceMetadata.ResourceData, &result)
			if !push(result) || result.Diagnostics.HasError() {
				return
			}
		}
	}
}
