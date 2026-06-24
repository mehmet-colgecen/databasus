package kubernetes

import (
	"context"
	"fmt"
	"io"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

// OpenExportStream builds the in-cluster client and returns a reader over the
// sanitized multi-document YAML export. A goroutine streams documents into a
// pipe; the returned Closer is the read side, whose closure unblocks the writer.
func (k *KubernetesDatabase) OpenExportStream(
	ctx context.Context,
) (io.Reader, io.Closer, error) {
	clientset, err := buildInClusterClientset()
	if err != nil {
		return nil, nil, err
	}

	reader, writer := io.Pipe()

	go func() {
		err := streamExport(ctx, clientset, k, writer)
		_ = writer.CloseWithError(err)
	}()

	return reader, reader, nil
}

// streamExport runs inside the OpenExportStream goroutine and returns any write
// error so the caller can propagate it to the pipe via CloseWithError. Documents
// are separated by "---" with no leading separator before the first one.
func streamExport(
	ctx context.Context,
	clientset kubernetes.Interface,
	db *KubernetesDatabase,
	writer io.Writer,
) error {
	namespaces, err := resolveNamespaces(ctx, clientset, db)
	if err != nil {
		return err
	}

	nameFilter := toNameFilter(db.ObjectNames)
	isFirstDocument := true

	for _, namespace := range namespaces {
		for _, resourceType := range db.ResourceTypes {
			objects, listErr := listObjects(ctx, clientset, KubernetesResourceType(resourceType), namespace)
			if listErr != nil {
				return listErr
			}

			for _, object := range objects {
				metaObject, ok := object.(metav1.Object)
				if !ok {
					continue
				}

				if nameFilter != nil {
					if _, isWanted := nameFilter[metaObject.GetName()]; !isWanted {
						continue
					}
				}

				sanitizeObjectMeta(metaObject)

				document, marshalErr := toYAMLDocument(object)
				if marshalErr != nil {
					return fmt.Errorf("failed to marshal %s/%s: %w", metaObject.GetNamespace(), metaObject.GetName(), marshalErr)
				}

				if !isFirstDocument {
					if _, writeErr := io.WriteString(writer, "---\n"); writeErr != nil {
						return writeErr
					}
				}
				isFirstDocument = false

				if _, writeErr := writer.Write(document); writeErr != nil {
					return writeErr
				}
			}
		}
	}

	return nil
}

func listObjects(
	ctx context.Context,
	clientset kubernetes.Interface,
	resourceType KubernetesResourceType,
	namespace string,
) ([]runtime.Object, error) {
	switch resourceType {
	case KubernetesResourceTypeSecret:
		list, err := clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list secrets (namespace %q): %w", namespace, err)
		}

		objects := make([]runtime.Object, 0, len(list.Items))
		for i := range list.Items {
			objects = append(objects, &list.Items[i])
		}

		return objects, nil

	case KubernetesResourceTypeConfigMap:
		list, err := clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list configmaps (namespace %q): %w", namespace, err)
		}

		objects := make([]runtime.Object, 0, len(list.Items))
		for i := range list.Items {
			objects = append(objects, &list.Items[i])
		}

		return objects, nil

	default:
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

// toNameFilter returns nil when no names are configured, meaning "no filter —
// pass all objects through"; the absence of a map is intentional, not a bug.
func toNameFilter(names []string) map[string]struct{} {
	if len(names) == 0 {
		return nil
	}

	filter := make(map[string]struct{}, len(names))
	for _, name := range names {
		filter[name] = struct{}{}
	}

	return filter
}
