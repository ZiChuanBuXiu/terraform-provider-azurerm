// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package cognitive

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/resourceids"
	"github.com/hashicorp/go-azure-sdk/resource-manager/cognitive/2026-03-01/projectconnectionresource"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/cognitive/validate"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
)

//go:generate go run ../../tools/generator-tests resourceidentity -resource-name cognitive_account_project_connection_project_managed_identity -properties "name" -compare-values "subscription_id:cognitive_account_project_id,resource_group_name:cognitive_account_project_id,account_name:cognitive_account_project_id,project_name:cognitive_account_project_id" -test-name "basic" -test-expect-non-empty

var (
	_ sdk.ResourceWithUpdate         = CognitiveAccountProjectConnectionProjectManagedIdentityResource{}
	_ sdk.ResourceWithIdentity       = CognitiveAccountProjectConnectionProjectManagedIdentityResource{}
	_ sdk.ResourceWithCustomImporter = CognitiveAccountProjectConnectionProjectManagedIdentityResource{}
)

type CognitiveAccountProjectConnectionProjectManagedIdentityResource struct{}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) CustomImporter() sdk.ResourceRunFunc {
	return cognitiveAccountProjectConnectionImporter(projectconnectionresource.ConnectionAuthTypeProjectManagedIdentity, r.ResourceType())
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) Identity() resourceids.ResourceId {
	return new(projectconnectionresource.ProjectConnectionId)
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) ResourceType() string {
	return "azurerm_cognitive_account_project_connection_project_managed_identity"
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) ModelObject() interface{} {
	return &CognitiveAccountProjectConnectionProjectManagedIdentityModel{}
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return projectconnectionresource.ValidateProjectConnectionID
}

type CognitiveAccountProjectConnectionProjectManagedIdentityModel struct {
	Name                      string            `tfschema:"name"`
	CognitiveAccountProjectId string            `tfschema:"cognitive_account_project_id"`
	AuthType                  string            `tfschema:"authentication_type"`
	Category                  string            `tfschema:"category"`
	Metadata                  map[string]string `tfschema:"metadata"`
	Target                    string            `tfschema:"target"`
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ForceNew:     true,
			ValidateFunc: validate.AccountProjectConnectionName(),
		},

		"cognitive_account_project_id": commonschema.ResourceIDReferenceRequiredForceNew(&projectconnectionresource.ProjectId{}),

		"category": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ForceNew: true,
			ValidateFunc: validation.StringInSlice([]string{
				string(projectconnectionresource.ConnectionCategoryAzureOpenAI),
				string(projectconnectionresource.ConnectionCategoryAIServices),
				string(projectconnectionresource.ConnectionCategoryCognitiveService),
				string(projectconnectionresource.ConnectionCategoryCognitiveSearch),
				string(projectconnectionresource.ConnectionCategoryApiManagement),
				string(projectconnectionresource.ConnectionCategoryRemoteTool),
				string(projectconnectionresource.ConnectionCategoryRemoteATwoA),
				string(projectconnectionresource.ConnectionCategoryAzureContainerAppEnvironment),
				string(projectconnectionresource.ConnectionCategoryAzureStorageAccount),
				"PlaywrightWorkspace",
				"OpenAPI",
			}, false),
		},

		"metadata": {
			Type:     pluginsdk.TypeMap,
			Required: true,
			Elem: &pluginsdk.Schema{
				Type: pluginsdk.TypeString,
			},
		},

		"target": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ValidateFunc: validation.StringIsNotEmpty,
		},
	}
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"authentication_type": {
			Type:     pluginsdk.TypeString,
			Computed: true,
		},
	}
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) Create() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.Cognitive.ProjectConnectionResourceClient

			var model CognitiveAccountProjectConnectionProjectManagedIdentityModel
			if err := metadata.Decode(&model); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			projectId, err := projectconnectionresource.ParseProjectID(model.CognitiveAccountProjectId)
			if err != nil {
				return err
			}

			id := projectconnectionresource.NewProjectConnectionID(projectId.SubscriptionId, projectId.ResourceGroupName, projectId.AccountName, projectId.ProjectName, model.Name)
			if !metadata.Client.Features.SkipImportCheckOnCreateAndAllowOverwritingExistingResources {
				existing, err := client.ProjectConnectionsGet(ctx, id)
				if err != nil && !response.WasNotFound(existing.HttpResponse) {
					return fmt.Errorf("checking for existing %s: %+v", id, err)
				}

				if !response.WasNotFound(existing.HttpResponse) {
					return metadata.ResourceRequiresImport(r.ResourceType(), id)
				}
			}

			properties := projectconnectionresource.BaseConnectionPropertiesV2Impl{
				AuthType: projectconnectionresource.ConnectionAuthTypeProjectManagedIdentity,
				Category: pointer.ToEnum[projectconnectionresource.ConnectionCategory](model.Category),
				Metadata: pointer.To(model.Metadata),
				Target:   pointer.To(model.Target),
			}

			connection := projectconnectionresource.ConnectionPropertiesV2BasicResource{
				Properties: properties,
			}

			if _, err := client.ProjectConnectionsCreate(ctx, id, connection); err != nil {
				return fmt.Errorf("creating %s: %+v", id, err)
			}

			metadata.SetID(id)
			if err := pluginsdk.SetResourceIdentityData(metadata.ResourceData, &id); err != nil {
				return err
			}

			return nil
		},
	}
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.Cognitive.ProjectConnectionResourceClient

			id, err := projectconnectionresource.ParseProjectConnectionID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			resp, err := client.ProjectConnectionsGet(ctx, *id)
			if err != nil {
				if response.WasNotFound(resp.HttpResponse) {
					return metadata.MarkAsGone(id)
				}
				return fmt.Errorf("retrieving %s: %+v", *id, err)
			}

			if authType, ok := cognitiveAccountProjectConnectionAuthType(resp.Model); ok && authType != projectconnectionresource.ConnectionAuthTypeProjectManagedIdentity {
				return metadata.MarkAsGone(id)
			}

			var currentState CognitiveAccountProjectConnectionProjectManagedIdentityModel
			if err := metadata.Decode(&currentState); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			return r.flatten(metadata, id, resp.Model, currentState.Metadata)
		},
	}
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) Update() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.Cognitive.ProjectConnectionResourceClient

			id, err := projectconnectionresource.ParseProjectConnectionID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			resp, err := client.ProjectConnectionsGet(ctx, *id)
			if err != nil {
				return fmt.Errorf("retrieving %s: %+v", *id, err)
			}

			if resp.Model == nil {
				return fmt.Errorf("retrieving %s: `model` was nil", *id)
			}

			if resp.Model.Properties == nil {
				return fmt.Errorf("retrieving %s: `properties` were nil", *id)
			}

			properties := resp.Model.Properties.ConnectionPropertiesV2()
			if properties.AuthType != projectconnectionresource.ConnectionAuthTypeProjectManagedIdentity {
				return fmt.Errorf("unexpected authentication type `%s` for %s", properties.AuthType, *id)
			}

			var model CognitiveAccountProjectConnectionProjectManagedIdentityModel
			if err := metadata.Decode(&model); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			if metadata.ResourceData.HasChange("metadata") {
				properties.Metadata = pointer.To(model.Metadata)
			}

			if metadata.ResourceData.HasChange("target") {
				properties.Target = pointer.To(model.Target)
			}

			connection := projectconnectionresource.ConnectionPropertiesV2BasicResource{
				Properties: properties,
			}

			if _, err := client.ProjectConnectionsCreate(ctx, *id, connection); err != nil {
				return fmt.Errorf("updating %s: %+v", *id, err)
			}

			return nil
		},
	}
}

func (r CognitiveAccountProjectConnectionProjectManagedIdentityResource) Delete() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.Cognitive.ProjectConnectionResourceClient

			id, err := projectconnectionresource.ParseProjectConnectionID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			if _, err := client.ProjectConnectionsDelete(ctx, *id); err != nil {
				return fmt.Errorf("deleting %s: %+v", *id, err)
			}

			return nil
		},
	}
}

func (CognitiveAccountProjectConnectionProjectManagedIdentityResource) flatten(metadata sdk.ResourceMetaData, id *projectconnectionresource.ProjectConnectionId, model *projectconnectionresource.ConnectionPropertiesV2BasicResource, priorMetadata map[string]string) error {
	state := CognitiveAccountProjectConnectionProjectManagedIdentityModel{
		CognitiveAccountProjectId: projectconnectionresource.NewProjectID(id.SubscriptionId, id.ResourceGroupName, id.AccountName, id.ProjectName).ID(),
		Name:                      id.ConnectionName,
	}

	if err := pluginsdk.SetResourceIdentityData(metadata.ResourceData, id); err != nil {
		return err
	}

	if model != nil && model.Properties != nil {
		base := model.Properties.ConnectionPropertiesV2()
		state.AuthType = string(base.AuthType)
		state.Category = pointer.FromEnum(base.Category)
		state.Metadata = flattenProjectConnectionMetadata(priorMetadata, base.Metadata)
		state.Target = pointer.From(base.Target)
	}

	return metadata.Encode(&state)
}
