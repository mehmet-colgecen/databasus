package kubernetes

type KubernetesResourceType string

const (
	KubernetesResourceTypeSecret    KubernetesResourceType = "SECRET"
	KubernetesResourceTypeConfigMap KubernetesResourceType = "CONFIGMAP"
)

type KubernetesNamespaceScope string

const (
	KubernetesNamespaceScopeAll      KubernetesNamespaceScope = "ALL"
	KubernetesNamespaceScopeSpecific KubernetesNamespaceScope = "SPECIFIC"
)
