package kubernetes

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"databasus-backend/internal/util/encryption"
)

type KubernetesDatabase struct {
	ID         uuid.UUID  `json:"id"         gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	DatabaseID *uuid.UUID `json:"databaseId" gorm:"type:uuid;column:database_id"`

	Version string `json:"version" gorm:"type:text;not null;default:''"`

	ResourceTypes       []string `json:"resourceTypes" gorm:"-"`
	ResourceTypesString string   `json:"-"             gorm:"column:resource_types;type:text;not null;default:''"`

	NamespaceScope string `json:"namespaceScope" gorm:"column:namespace_scope;type:text;not null;default:'ALL'"`

	Namespaces       []string `json:"namespaces" gorm:"-"`
	NamespacesString string   `json:"-"          gorm:"column:namespaces;type:text;not null;default:''"`

	ObjectNames       []string `json:"objectNames" gorm:"-"`
	ObjectNamesString string   `json:"-"           gorm:"column:object_names;type:text;not null;default:''"`
}

func (k *KubernetesDatabase) TableName() string {
	return "kubernetes_databases"
}

func (k *KubernetesDatabase) BeforeSave(_ *gorm.DB) error {
	k.ResourceTypesString = strings.Join(k.ResourceTypes, ",")
	k.NamespacesString = strings.Join(k.Namespaces, ",")
	k.ObjectNamesString = strings.Join(k.ObjectNames, ",")

	return nil
}

func (k *KubernetesDatabase) AfterFind(_ *gorm.DB) error {
	k.ResourceTypes = splitNonEmpty(k.ResourceTypesString)
	k.Namespaces = splitNonEmpty(k.NamespacesString)
	k.ObjectNames = splitNonEmpty(k.ObjectNamesString)

	return nil
}

func splitNonEmpty(value string) []string {
	if value == "" {
		return []string{}
	}

	return strings.Split(value, ",")
}

func (k *KubernetesDatabase) Validate() error {
	if len(k.ResourceTypes) == 0 {
		return errors.New("at least one resource type is required")
	}

	for _, resourceType := range k.ResourceTypes {
		switch KubernetesResourceType(resourceType) {
		case KubernetesResourceTypeSecret, KubernetesResourceTypeConfigMap:
		default:
			return errors.New("invalid resource type: " + resourceType)
		}
	}

	switch KubernetesNamespaceScope(k.NamespaceScope) {
	case KubernetesNamespaceScopeAll:
	case KubernetesNamespaceScopeSpecific:
		if len(k.Namespaces) == 0 {
			return errors.New("at least one namespace is required when namespace scope is SPECIFIC")
		}
	default:
		return errors.New("invalid namespace scope: " + k.NamespaceScope)
	}

	return nil
}

func (k *KubernetesDatabase) Update(incoming *KubernetesDatabase) {
	k.Version = incoming.Version
	k.ResourceTypes = incoming.ResourceTypes
	k.NamespaceScope = incoming.NamespaceScope
	k.Namespaces = incoming.Namespaces
	k.ObjectNames = incoming.ObjectNames
}

// HideSensitiveData is a no-op: the configuration holds no credentials
// (authentication is the backend's in-cluster ServiceAccount).
func (k *KubernetesDatabase) HideSensitiveData() {}

// EncryptSensitiveFields is a no-op for the same reason as HideSensitiveData.
func (k *KubernetesDatabase) EncryptSensitiveFields(_ encryption.FieldEncryptor) error {
	return nil
}

// GetRawDbSizeMb returns 0: Secrets/ConfigMaps are configuration, not a dataset.
// The real artifact size is recorded by the streaming counting writer.
func (k *KubernetesDatabase) GetRawDbSizeMb(
	_ context.Context,
	_ *slog.Logger,
	_ encryption.FieldEncryptor,
) (float64, error) {
	return 0, nil
}
