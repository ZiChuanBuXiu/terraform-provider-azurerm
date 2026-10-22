package horizondb

import (
	"strings"
	"testing"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-sdk/resource-manager/horizondb/2026-10-01-preview/defaultparametergroups"
)

func TestFindHorizondbDefaultParameterGroup(t *testing.T) {
	group17 := defaultparametergroups.DefaultParameterGroup{
		Id: pointer.To("/subscriptions/00000000-0000-0000-0000-000000000000/providers/Microsoft.HorizonDb/locations/westeurope/parameterGroups/default17"),
		Properties: &defaultparametergroups.DefaultParameterGroupProperties{
			PostgreSqlVersion: defaultparametergroups.PostgreSqlVersion("17"),
		},
	}
	group18 := defaultparametergroups.DefaultParameterGroup{
		Id: pointer.To("/subscriptions/00000000-0000-0000-0000-000000000000/providers/Microsoft.HorizonDb/locations/westeurope/parameterGroups/default18"),
		Properties: &defaultparametergroups.DefaultParameterGroupProperties{
			PostgreSqlVersion: defaultparametergroups.PostgreSqlVersion("18"),
		},
	}
	tests := []struct {
		name          string
		groups        []defaultparametergroups.DefaultParameterGroup
		version       string
		expectedId    string
		errorContains string
	}{
		{name: "version17", groups: []defaultparametergroups.DefaultParameterGroup{group18, group17}, version: "17", expectedId: *group17.Id},
		{name: "version18", groups: []defaultparametergroups.DefaultParameterGroup{group17, group18}, version: "18", expectedId: *group18.Id},
		{name: "empty", version: "17", errorContains: "no default parameter group"},
		{name: "missingVersion", groups: []defaultparametergroups.DefaultParameterGroup{group17}, version: "18", errorContains: "no default parameter group"},
		{name: "duplicateVersion", groups: []defaultparametergroups.DefaultParameterGroup{group17, group17}, version: "17", errorContains: "multiple default parameter groups"},
		{name: "missingProperties", groups: []defaultparametergroups.DefaultParameterGroup{{}}, version: "17", errorContains: "no properties"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := findHorizondbDefaultParameterGroup(test.groups, test.version)
			if test.errorContains != "" {
				if err == nil || !strings.Contains(err.Error(), test.errorContains) {
					t.Fatalf("expected error containing %q, got %v", test.errorContains, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if pointer.From(result.Id) != test.expectedId {
				t.Fatalf("expected ID %q, got %q", test.expectedId, pointer.From(result.Id))
			}
		})
	}
}
