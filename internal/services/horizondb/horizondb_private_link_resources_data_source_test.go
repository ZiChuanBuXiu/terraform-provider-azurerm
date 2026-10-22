package horizondb

import (
	"reflect"
	"testing"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/privatelinkresources"
)

func TestFlattenHorizondbPrivateLinkResources(t *testing.T) {
	cluster := privatelinkresources.PrivateLinkResource{
		Id: pointer.To("/SUBSCRIPTIONS/00000000-0000-0000-0000-000000000000/RESOURCEGROUPS/example-rg/PROVIDERS/microsoft.horizondb/CLUSTERS/example-cluster/PRIVATELINKRESOURCES/example-cluster"),
		Properties: &privatelinkresources.PrivateLinkResourceProperties{
			GroupId:           pointer.To("example-cluster"),
			RequiredMembers:   pointer.To([]string{"member-b", "member-a"}),
			RequiredZoneNames: pointer.To([]string{"privatelink.horizondb.azure.com"}),
		},
	}
	pool := privatelinkresources.PrivateLinkResource{
		Id: pointer.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/example-rg/providers/Microsoft.HorizonDb/clusters/example-cluster/privateLinkResources/example-pool"),
		Properties: &privatelinkresources.PrivateLinkResourceProperties{
			GroupId: pointer.To("example-pool"),
		},
	}

	result, err := flattenHorizondbPrivateLinkResources([]privatelinkresources.PrivateLinkResource{pool, cluster})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected two resources, got %d", len(result))
	}
	expectedId := "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/example-rg/providers/Microsoft.HorizonDb/clusters/example-cluster/privateLinkResources/example-cluster"
	if result[0].Id != expectedId || result[0].GroupId != "example-cluster" {
		t.Fatalf("unexpected first resource: %#v", result[0])
	}
	if !reflect.DeepEqual(result[0].RequiredMembers, []string{"member-a", "member-b"}) {
		t.Fatalf("unexpected member order: %v", result[0].RequiredMembers)
	}
	if !reflect.DeepEqual(*cluster.Properties.RequiredMembers, []string{"member-b", "member-a"}) {
		t.Fatal("flatten modified the SDK response")
	}
	if !reflect.DeepEqual(result[0].RequiredZoneNames, []string{"privatelink.horizondb.azure.com"}) {
		t.Fatalf("unexpected DNS zones: %v", result[0].RequiredZoneNames)
	}
	if result[1].RequiredMembers == nil || result[1].RequiredZoneNames == nil {
		t.Fatal("absent members and zones must flatten to empty collections")
	}

	forward, err := flattenHorizondbPrivateLinkResources([]privatelinkresources.PrivateLinkResource{cluster, pool})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result, forward) {
		t.Fatal("API response order changed Terraform state")
	}
}

func TestFlattenHorizondbPrivateLinkResourcesInvalid(t *testing.T) {
	id := "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/example-rg/providers/Microsoft.HorizonDb/clusters/example-cluster/privateLinkResources/example-cluster"
	tests := map[string]privatelinkresources.PrivateLinkResource{
		"missingID":         {},
		"invalidID":         {Id: pointer.To("invalid")},
		"missingProperties": {Id: pointer.To(id)},
		"missingGroup":      {Id: pointer.To(id), Properties: &privatelinkresources.PrivateLinkResourceProperties{}},
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := flattenHorizondbPrivateLinkResources([]privatelinkresources.PrivateLinkResource{input}); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestFlattenHorizondbPrivateLinkResourcesEmpty(t *testing.T) {
	result, err := flattenHorizondbPrivateLinkResources(nil)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || len(result) != 0 {
		t.Fatalf("expected an empty collection, got %#v", result)
	}
}
