package kubernetes

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Validate(t *testing.T) {
	testCases := []struct {
		name      string
		db        KubernetesDatabase
		isInvalid bool
	}{
		{
			name:      "no resource types is invalid",
			db:        KubernetesDatabase{NamespaceScope: string(KubernetesNamespaceScopeAll)},
			isInvalid: true,
		},
		{
			name: "all-scope with one resource type is valid",
			db: KubernetesDatabase{
				ResourceTypes:  []string{string(KubernetesResourceTypeSecret)},
				NamespaceScope: string(KubernetesNamespaceScopeAll),
			},
			isInvalid: false,
		},
		{
			name: "specific-scope without namespaces is invalid",
			db: KubernetesDatabase{
				ResourceTypes:  []string{string(KubernetesResourceTypeConfigMap)},
				NamespaceScope: string(KubernetesNamespaceScopeSpecific),
			},
			isInvalid: true,
		},
		{
			name: "specific-scope with namespaces is valid",
			db: KubernetesDatabase{
				ResourceTypes:  []string{string(KubernetesResourceTypeConfigMap)},
				NamespaceScope: string(KubernetesNamespaceScopeSpecific),
				Namespaces:     []string{"prod"},
			},
			isInvalid: false,
		},
		{
			name: "unknown resource type is invalid",
			db: KubernetesDatabase{
				ResourceTypes:  []string{"PODS"},
				NamespaceScope: string(KubernetesNamespaceScopeAll),
			},
			isInvalid: true,
		},
		{
			name: "unknown namespace scope is invalid",
			db: KubernetesDatabase{
				ResourceTypes:  []string{string(KubernetesResourceTypeSecret)},
				NamespaceScope: "CLUSTER",
			},
			isInvalid: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.db.Validate()
			if tc.isInvalid {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_AfterFind_EmptyStringsYieldEmptySlices(t *testing.T) {
	loaded := KubernetesDatabase{}

	assert.NoError(t, loaded.AfterFind(nil))
	assert.Equal(t, []string{}, loaded.ResourceTypes)
	assert.Equal(t, []string{}, loaded.Namespaces)
	assert.Equal(t, []string{}, loaded.ObjectNames)
}

func Test_BeforeSaveAfterFind_RoundTripsListColumns(t *testing.T) {
	db := KubernetesDatabase{
		ResourceTypes: []string{"SECRET", "CONFIGMAP"},
		Namespaces:    []string{"prod", "staging"},
		ObjectNames:   []string{"app-config"},
	}

	assert.NoError(t, db.BeforeSave(nil))
	assert.Equal(t, "SECRET,CONFIGMAP", db.ResourceTypesString)
	assert.Equal(t, "prod,staging", db.NamespacesString)
	assert.Equal(t, "app-config", db.ObjectNamesString)

	loaded := KubernetesDatabase{
		ResourceTypesString: "SECRET,CONFIGMAP",
		NamespacesString:    "prod,staging",
		ObjectNamesString:   "app-config",
	}
	assert.NoError(t, loaded.AfterFind(nil))
	assert.Equal(t, []string{"SECRET", "CONFIGMAP"}, loaded.ResourceTypes)
	assert.Equal(t, []string{"prod", "staging"}, loaded.Namespaces)
	assert.Equal(t, []string{"app-config"}, loaded.ObjectNames)
}
