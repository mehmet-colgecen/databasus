package kubernetes

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_SanitizeObjectMeta_StripsServerFields(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "tls-cert",
			Namespace:         "prod",
			ResourceVersion:   "12345",
			UID:               "abc-uid",
			Generation:        7,
			CreationTimestamp: metav1.NewTime(time.Unix(1000, 0)),
			ManagedFields:     []metav1.ManagedFieldsEntry{{Manager: "kubectl"}},
			OwnerReferences:   []metav1.OwnerReference{{Name: "owner"}},
			SelfLink:          "/api/v1/...",
			Labels:            map[string]string{"app": "web"},
			Annotations: map[string]string{
				"kubectl.kubernetes.io/last-applied-configuration": "{...}",
				"team": "payments",
			},
		},
	}

	sanitizeObjectMeta(secret)

	assert.Equal(t, "tls-cert", secret.Name)
	assert.Equal(t, "prod", secret.Namespace)
	assert.Equal(t, map[string]string{"app": "web"}, secret.Labels)
	assert.Equal(t, map[string]string{"team": "payments"}, secret.Annotations)
	assert.Empty(t, secret.ResourceVersion)
	assert.Empty(t, secret.UID)
	assert.Zero(t, secret.Generation)
	assert.True(t, secret.CreationTimestamp.IsZero())
	assert.Nil(t, secret.ManagedFields)
	assert.Nil(t, secret.OwnerReferences)
	assert.Empty(t, secret.SelfLink)
}

func Test_ToYAMLDocument_SetsTypeMetaForSecret(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "tls-cert", Namespace: "prod"},
		Data:       map[string][]byte{"tls.crt": []byte("xyz")},
		Type:       corev1.SecretTypeTLS,
	}

	doc, err := toYAMLDocument(secret)
	assert.NoError(t, err)

	text := string(doc)
	assert.True(t, strings.Contains(text, "apiVersion: v1"))
	assert.True(t, strings.Contains(text, "kind: Secret"))
	assert.True(t, strings.Contains(text, "name: tls-cert"))
}

func Test_ToYAMLDocument_SetsTypeMetaForConfigMap(t *testing.T) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "app-config", Namespace: "prod"},
		Data:       map[string]string{"key": "value"},
	}

	doc, err := toYAMLDocument(configMap)
	assert.NoError(t, err)

	text := string(doc)
	assert.True(t, strings.Contains(text, "kind: ConfigMap"))
	assert.True(t, strings.Contains(text, "name: app-config"))
}
