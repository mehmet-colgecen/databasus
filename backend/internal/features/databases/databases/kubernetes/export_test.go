package kubernetes

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_StreamExport_AllNamespacesBothTypes(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "prod", ResourceVersion: "9"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "staging"}},
	)
	db := &KubernetesDatabase{
		ResourceTypes:  []string{"SECRET", "CONFIGMAP"},
		NamespaceScope: "ALL",
	}

	var buf bytes.Buffer
	err := streamExport(context.Background(), clientset, db, &buf)
	assert.NoError(t, err)

	out := buf.String()
	assert.True(t, strings.Contains(out, "kind: Secret"))
	assert.True(t, strings.Contains(out, "name: s1"))
	assert.True(t, strings.Contains(out, "kind: ConfigMap"))
	assert.True(t, strings.Contains(out, "name: c1"))
	assert.True(t, strings.Contains(out, "---"))
	assert.False(t, strings.Contains(out, "resourceVersion: \"9\""))
}

func Test_StreamExport_ObjectNameFilter(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "keep", Namespace: "prod"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "drop", Namespace: "prod"}},
	)
	db := &KubernetesDatabase{
		ResourceTypes:  []string{"SECRET"},
		NamespaceScope: "SPECIFIC",
		Namespaces:     []string{"prod"},
		ObjectNames:    []string{"keep"},
	}

	var buf bytes.Buffer
	err := streamExport(context.Background(), clientset, db, &buf)
	assert.NoError(t, err)

	out := buf.String()
	assert.True(t, strings.Contains(out, "name: keep"))
	assert.False(t, strings.Contains(out, "name: drop"))
}
