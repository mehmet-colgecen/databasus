package kubernetes

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func buildInClusterClientset() (kubernetes.Interface, error) {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build in-cluster config (is Databasus running inside Kubernetes?): %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build Kubernetes client: %w", err)
	}

	return clientset, nil
}

// detectServerVersion ignores the context because Discovery().ServerVersion()
// does not accept one; the parameter exists for signature parity with the other
// client helpers and for future use should the discovery client gain context support.
func detectServerVersion(_ context.Context, clientset kubernetes.Interface) (string, error) {
	versionInfo, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to read Kubernetes server version: %w", err)
	}

	return versionInfo.GitVersion, nil
}

// verifyReadAccess confirms the ServiceAccount can list each selected resource
// type within the configured namespace scope by issuing a 1-item list.
func verifyReadAccess(
	ctx context.Context,
	clientset kubernetes.Interface,
	db *KubernetesDatabase,
) error {
	namespaces, err := resolveNamespaces(ctx, clientset, db)
	if err != nil {
		return err
	}

	listOptions := metav1.ListOptions{Limit: 1}

	for _, namespace := range namespaces {
		for _, resourceType := range db.ResourceTypes {
			switch KubernetesResourceType(resourceType) {
			case KubernetesResourceTypeSecret:
				if _, err := clientset.CoreV1().Secrets(namespace).List(ctx, listOptions); err != nil {
					return fmt.Errorf("cannot list secrets (namespace %q): %w", namespace, err)
				}
			case KubernetesResourceTypeConfigMap:
				if _, err := clientset.CoreV1().ConfigMaps(namespace).List(ctx, listOptions); err != nil {
					return fmt.Errorf("cannot list configmaps (namespace %q): %w", namespace, err)
				}
			default:
				return fmt.Errorf("unsupported resource type %q: cannot verify read access", resourceType)
			}
		}
	}

	return nil
}

// resolveNamespaces returns the namespaces to operate on. For ALL scope it
// returns a single empty string, which client-go treats as "all namespaces"
// (requires the cluster-wide ClusterRole). For SPECIFIC scope it returns the
// configured list.
func resolveNamespaces(
	_ context.Context,
	_ kubernetes.Interface,
	db *KubernetesDatabase,
) ([]string, error) {
	switch KubernetesNamespaceScope(db.NamespaceScope) {
	case KubernetesNamespaceScopeAll:
		return []string{metav1.NamespaceAll}, nil
	case KubernetesNamespaceScopeSpecific:
		if len(db.Namespaces) == 0 {
			return nil, errors.New("no namespaces configured for SPECIFIC scope")
		}

		return db.Namespaces, nil
	default:
		return nil, errors.New("invalid namespace scope: " + db.NamespaceScope)
	}
}
