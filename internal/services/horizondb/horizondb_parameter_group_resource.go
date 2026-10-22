// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package horizondb

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/location"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/resourceids"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/parametergroups"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
)

//go:generate go run ../../tools/generator-tests resourceidentity
type HorizondbParameterGroupResource struct{}

type HorizondbParameterGroupResourceModel struct {
	Name              string            `tfschema:"name"`
	ResourceGroupName string            `tfschema:"resource_group_name"`
	Location          string            `tfschema:"location"`
	PostgresqlVersion string            `tfschema:"postgresql_version"`
	Description       string            `tfschema:"description"`
	Parameters        map[string]string `tfschema:"parameters"`
	Tags              map[string]string `tfschema:"tags"`
}

var (
	_ sdk.ResourceWithUpdate        = HorizondbParameterGroupResource{}
	_ sdk.ResourceWithIdentity      = HorizondbParameterGroupResource{}
	_ sdk.ResourceWithCustomizeDiff = HorizondbParameterGroupResource{}
)

func (HorizondbParameterGroupResource) ModelObject() interface{} {
	return &HorizondbParameterGroupResourceModel{}
}

func (HorizondbParameterGroupResource) ResourceType() string {
	return "azurerm_horizondb_parameter_group"
}

func (HorizondbParameterGroupResource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return parametergroups.ValidateProviderParameterGroupID
}

func (HorizondbParameterGroupResource) Identity() resourceids.ResourceId {
	return &parametergroups.ProviderParameterGroupId{}
}

func (HorizondbParameterGroupResource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type: pluginsdk.TypeString, Required: true, ForceNew: true,
			ValidateFunc: validation.All(
				validation.StringLenBetween(2, 63),
				validation.StringMatch(regexp.MustCompile(`^[a-zA-Z0-9][-a-zA-Z0-9]*[a-zA-Z0-9]$`), "must contain only letters, numbers and hyphens and start and end with a letter or number"),
			),
		},
		"resource_group_name": commonschema.ResourceGroupName(),
		"location":            commonschema.Location(),
		"postgresql_version": {
			Type: pluginsdk.TypeString, Required: true, ForceNew: true,
			ValidateFunc: validation.StringInSlice(parametergroups.PossibleValuesForPostgreSqlVersion(), false),
		},
		"description": {
			Type: pluginsdk.TypeString, Optional: true,
			ValidateFunc: validation.StringLenBetween(0, 1024),
		},
		"parameters": {
			Type: pluginsdk.TypeMap, Optional: true,
			Elem: &pluginsdk.Schema{Type: pluginsdk.TypeString},
		},
		"tags": commonschema.Tags(),
	}
}

func (HorizondbParameterGroupResource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{}
}

func (HorizondbParameterGroupResource) CustomizeDiff() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			diff := metadata.ResourceDiff
			if diff.Id() == "" || diff.HasChanges("name", "resource_group_name", "location", "postgresql_version") || !diff.NewValueKnown("parameters") {
				return nil
			}
			old, new := diff.GetChange("parameters")
			return validateHorizondbParameterChanges(old.(map[string]interface{}), new.(map[string]interface{}))
		},
	}
}

func validateHorizondbParameterChanges(old, new map[string]interface{}) error {
	var removed []string
	for name := range old {
		if _, exists := new[name]; !exists {
			removed = append(removed, name)
		}
	}
	if len(removed) > 0 {
		slices.Sort(removed)
		return fmt.Errorf("removing parameter overrides (%s) is not supported because the HorizonDB reset API contract is unconfirmed; retain these keys in `parameters`", strings.Join(removed, ", "))
	}
	return nil
}

func (r HorizondbParameterGroupResource) Create() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 60 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			var state HorizondbParameterGroupResourceModel
			if err := metadata.Decode(&state); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}
			id := parametergroups.NewProviderParameterGroupID(metadata.Client.Account.SubscriptionId, state.ResourceGroupName, state.Name)
			client := metadata.Client.HorizonDB.ParameterGroupsClient
			if !metadata.Client.Features.SkipImportCheckOnCreateAndAllowOverwritingExistingResources {
				existing, err := client.Get(ctx, id)
				if err != nil && !response.WasNotFound(existing.HttpResponse) {
					return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
				}
				if !response.WasNotFound(existing.HttpResponse) {
					return metadata.ResourceRequiresImport(r.ResourceType(), id)
				}
			}
			payload := parametergroups.ParameterGroup{
				Location: location.Normalize(state.Location), Tags: pointer.To(state.Tags),
				Properties: &parametergroups.ParameterGroupProperties{
					PostgreSqlVersion: parametergroups.PostgreSqlVersion(state.PostgresqlVersion),
					Description:       pointer.To(state.Description), Parameters: pointer.To(expandHorizondbParameters(state.Parameters)),
				},
			}
			if err := client.CreateOrReplaceCallbackThenPoll(ctx, id, payload, metadata.SetIDAndIdentityCallback(&id)); err != nil {
				return fmt.Errorf("creating %s: %+v", id, err)
			}
			metadata.SetID(id)
			return pluginsdk.SetResourceIdentityData(metadata.ResourceData, &id)
		},
	}
}

func (r HorizondbParameterGroupResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			id, err := parametergroups.ParseProviderParameterGroupIDInsensitively(metadata.ResourceData.Id())
			if err != nil {
				return err
			}
			resp, err := metadata.Client.HorizonDB.ParameterGroupsClient.Get(ctx, *id)
			if err != nil {
				if response.WasNotFound(resp.HttpResponse) {
					return metadata.MarkAsGone(*id)
				}
				return fmt.Errorf("retrieving %s: %+v", id, err)
			}
			return r.flatten(metadata, id, resp.Model)
		},
	}
}

func (HorizondbParameterGroupResource) Update() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 60 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			id, err := parametergroups.ParseProviderParameterGroupIDInsensitively(metadata.ResourceData.Id())
			if err != nil {
				return err
			}
			old, new := metadata.ResourceData.GetChange("parameters")
			if err := validateHorizondbParameterChanges(old.(map[string]interface{}), new.(map[string]interface{})); err != nil {
				return err
			}
			var state HorizondbParameterGroupResourceModel
			if err := metadata.Decode(&state); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}
			payload := parametergroups.ParameterGroupForPatchUpdate{Properties: &parametergroups.ParameterGroupPropertiesForPatchUpdate{}}
			if metadata.ResourceData.HasChange("description") {
				payload.Properties.Description = pointer.To(state.Description)
			}
			if metadata.ResourceData.HasChange("parameters") {
				payload.Properties.Parameters = pointer.To(expandHorizondbParameters(state.Parameters))
			}
			if metadata.ResourceData.HasChange("tags") {
				payload.Tags = pointer.To(state.Tags)
			}
			if err := metadata.Client.HorizonDB.ParameterGroupsClient.UpdateThenPoll(ctx, *id, payload); err != nil {
				return fmt.Errorf("updating %s: %+v", id, err)
			}
			return nil
		},
	}
}

func (HorizondbParameterGroupResource) Delete() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 60 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			id, err := parametergroups.ParseProviderParameterGroupIDInsensitively(metadata.ResourceData.Id())
			if err != nil {
				return err
			}
			if err := metadata.Client.HorizonDB.ParameterGroupsClient.DeleteThenPoll(ctx, *id); err != nil {
				return fmt.Errorf("deleting %s: %+v", id, err)
			}
			return nil
		},
	}
}

func expandHorizondbParameters(input map[string]string) []parametergroups.ParameterProperties {
	names := make([]string, 0, len(input))
	for name := range input {
		names = append(names, name)
	}
	slices.Sort(names)
	result := make([]parametergroups.ParameterProperties, 0, len(names))
	for _, name := range names {
		result = append(result, parametergroups.ParameterProperties{Name: pointer.To(name), Value: pointer.To(input[name])})
	}
	return result
}

func flattenHorizondbParameterOverrides(input []parametergroups.ParameterProperties) (map[string]string, error) {
	result := make(map[string]string)
	seen := make(map[string]struct{})
	for _, parameter := range input {
		name := pointer.From(parameter.Name)
		if name == "" {
			return nil, errors.New("parameter response contains an empty name")
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("parameter response contains duplicate name %q", name)
		}
		seen[name] = struct{}{}
		switch pointer.From(parameter.ValueSource) {
		case parametergroups.ParameterValueSourceSystemDefault:
		case parametergroups.ParameterValueSourceUserOverride:
			if parameter.Value == nil {
				return nil, fmt.Errorf("parameter %q has no override value", name)
			}
			result[name] = *parameter.Value
		default:
			return nil, fmt.Errorf("parameter %q has missing or unsupported value source %q", name, pointer.From(parameter.ValueSource))
		}
	}
	return result, nil
}

func (HorizondbParameterGroupResource) flatten(metadata sdk.ResourceMetaData, id *parametergroups.ProviderParameterGroupId, model *parametergroups.ParameterGroup) error {
	if model == nil || model.Properties == nil {
		return fmt.Errorf("retrieving %s: parameter group properties are missing", id)
	}
	parameters, err := flattenHorizondbParameterOverrides(pointer.From(model.Properties.Parameters))
	if err != nil {
		return fmt.Errorf("flattening parameters for %s: %+v", id, err)
	}
	state := HorizondbParameterGroupResourceModel{
		Name: id.ParameterGroupName, ResourceGroupName: id.ResourceGroupName,
		Location: location.Normalize(model.Location), PostgresqlVersion: string(model.Properties.PostgreSqlVersion),
		Description: pointer.From(model.Properties.Description), Parameters: parameters, Tags: pointer.From(model.Tags),
	}
	metadata.SetID(*id)
	if err := pluginsdk.SetResourceIdentityData(metadata.ResourceData, id); err != nil {
		return err
	}
	return metadata.Encode(&state)
}
