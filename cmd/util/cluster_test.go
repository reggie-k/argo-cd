package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clientcmdapiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func Test_newCluster(t *testing.T) {
	labels := map[string]string{"key1": "val1"}
	annotations := map[string]string{"key2": "val2"}
	clusterWithData := NewCluster("test-cluster", []string{"test-namespace"}, false, &rest.Config{
		TLSClientConfig: rest.TLSClientConfig{
			Insecure:   false,
			ServerName: "test-endpoint.example.com",
			CAData:     []byte("test-ca-data"),
			CertData:   []byte("test-cert-data"),
			KeyData:    []byte("test-key-data"),
		},
		Host: "test-endpoint.example.com",
	},
		"test-bearer-token",
		&v1alpha1.AWSAuthConfig{},
		&v1alpha1.ExecProviderConfig{}, labels, annotations)

	assert.Equal(t, "test-cert-data", string(clusterWithData.Config.CertData))
	assert.Equal(t, "test-key-data", string(clusterWithData.Config.KeyData))
	assert.Empty(t, clusterWithData.Config.BearerToken)
	assert.Equal(t, labels, clusterWithData.Labels)
	assert.Equal(t, annotations, clusterWithData.Annotations)
	assert.False(t, clusterWithData.Config.DisableCompression)

	clusterWithFiles := NewCluster("test-cluster", []string{"test-namespace"}, false, &rest.Config{
		TLSClientConfig: rest.TLSClientConfig{
			Insecure:   false,
			ServerName: "test-endpoint.example.com",
			CAData:     []byte("test-ca-data"),
			CertFile:   "./testdata/test.cert.pem",
			KeyFile:    "./testdata/test.key.pem",
		},
		Host: "test-endpoint.example.com",
	},
		"test-bearer-token",
		&v1alpha1.AWSAuthConfig{},
		&v1alpha1.ExecProviderConfig{}, labels, nil)

	assert.Contains(t, string(clusterWithFiles.Config.CertData), "test-cert-data")
	assert.Contains(t, string(clusterWithFiles.Config.KeyData), "test-key-data")
	assert.Empty(t, clusterWithFiles.Config.BearerToken)
	assert.Equal(t, labels, clusterWithFiles.Labels)
	assert.Nil(t, clusterWithFiles.Annotations)

	clusterWithBearerToken := NewCluster("test-cluster", []string{"test-namespace"}, false, &rest.Config{
		TLSClientConfig: rest.TLSClientConfig{
			Insecure:   false,
			ServerName: "test-endpoint.example.com",
			CAData:     []byte("test-ca-data"),
		},
		Host: "test-endpoint.example.com",
	},
		"test-bearer-token",
		&v1alpha1.AWSAuthConfig{},
		&v1alpha1.ExecProviderConfig{}, nil, nil)

	assert.Equal(t, "test-bearer-token", clusterWithBearerToken.Config.BearerToken)
	assert.Nil(t, clusterWithBearerToken.Labels)
	assert.Nil(t, clusterWithBearerToken.Annotations)

	clusterWithDisableCompression := NewCluster("test-cluster", []string{"test-namespace"}, false, &rest.Config{
		TLSClientConfig: rest.TLSClientConfig{
			Insecure:   false,
			ServerName: "test-endpoint.example.com",
			CAData:     []byte("test-ca-data"),
		},
		DisableCompression: true,
		Host:               "test-endpoint.example.com",
	}, "test-bearer-token",
		&v1alpha1.AWSAuthConfig{},
		&v1alpha1.ExecProviderConfig{}, labels, annotations)

	assert.True(t, clusterWithDisableCompression.Config.DisableCompression)
}

func TestGetKubePublicEndpoint(t *testing.T) {
	cases := []struct {
		name             string
		clusterInfo      *corev1.ConfigMap
		expectedEndpoint string
		expectedCAData   []byte
		expectError      bool
	}{
		{
			name: "has public endpoint and certificate authority data",
			clusterInfo: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "kube-public",
					Name:      "cluster-info",
				},
				Data: map[string]string{
					"kubeconfig": kubeconfigFixture("https://test-cluster:6443", []byte("test-ca-data")),
				},
			},
			expectedEndpoint: "https://test-cluster:6443",
			expectedCAData:   []byte("test-ca-data"),
		},
		{
			name: "has public endpoint",
			clusterInfo: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "kube-public",
					Name:      "cluster-info",
				},
				Data: map[string]string{
					"kubeconfig": kubeconfigFixture("https://test-cluster:6443", nil),
				},
			},
			expectedEndpoint: "https://test-cluster:6443",
			expectedCAData:   nil,
		},
		{
			name:        "no cluster-info",
			expectError: true,
		},
		{
			name: "no kubeconfig in cluster-info",
			clusterInfo: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "kube-public",
					Name:      "cluster-info",
				},
				Data: map[string]string{
					"argo": "the project, not the movie",
				},
			},
			expectError: true,
		},
		{
			name: "no clusters in cluster-info kubeconfig",
			clusterInfo: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "kube-public",
					Name:      "cluster-info",
				},
				Data: map[string]string{
					"kubeconfig": kubeconfigFixture("", nil),
				},
			},
			expectError: true,
		},
		{
			name: "can't parse kubeconfig",
			clusterInfo: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "kube-public",
					Name:      "cluster-info",
				},
				Data: map[string]string{
					"kubeconfig": "this is not valid YAML",
				},
			},
			expectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			objects := []runtime.Object{}
			if tc.clusterInfo != nil {
				objects = append(objects, tc.clusterInfo)
			}
			clientset := fake.NewClientset(objects...)
			endpoint, caData, err := GetKubePublicEndpoint(clientset)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equalf(t, tc.expectedEndpoint, endpoint, "expected endpoint %s, got %s", tc.expectedEndpoint, endpoint)
			require.Equalf(t, tc.expectedCAData, caData, "expected caData %s, got %s", tc.expectedCAData, caData)
		})
	}
}

func kubeconfigFixture(endpoint string, certificateAuthorityData []byte) string {
	kubeconfig := &clientcmdapiv1.Config{}
	if endpoint != "" {
		kubeconfig.Clusters = []clientcmdapiv1.NamedCluster{
			{
				Name: "test-kube",
				Cluster: clientcmdapiv1.Cluster{
					Server:                   endpoint,
					CertificateAuthorityData: certificateAuthorityData,
				},
			},
		}
	}
	configYAML, err := yaml.Marshal(kubeconfig)
	if err != nil {
		return ""
	}
	return string(configYAML)
}
