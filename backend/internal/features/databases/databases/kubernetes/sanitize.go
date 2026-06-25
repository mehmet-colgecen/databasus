package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

const lastAppliedConfigAnnotation = "kubectl.kubernetes.io/last-applied-configuration"

// sanitizeObjectMeta removes server-managed metadata so the exported manifest
// is portable and re-appliable via `kubectl apply -f`.
func sanitizeObjectMeta(obj metav1.Object) {
	obj.SetResourceVersion("")
	obj.SetUID("")
	obj.SetGeneration(0)
	obj.SetCreationTimestamp(metav1.Time{})
	obj.SetDeletionTimestamp(nil)
	obj.SetManagedFields(nil)
	obj.SetOwnerReferences(nil)
	obj.SetSelfLink("")
	obj.SetGenerateName("")

	annotations := obj.GetAnnotations()
	delete(annotations, lastAppliedConfigAnnotation)
	if len(annotations) == 0 {
		annotations = nil
	}
	obj.SetAnnotations(annotations)
}

// toYAMLDocument sets the TypeMeta (List responses omit it) and marshals one
// object to YAML via sigs.k8s.io/yaml, which honours the JSON field tags.
func toYAMLDocument(obj runtime.Object) ([]byte, error) {
	switch typed := obj.(type) {
	case *corev1.Secret:
		typed.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"}
	case *corev1.ConfigMap:
		typed.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"}
	}

	return yaml.Marshal(obj)
}
