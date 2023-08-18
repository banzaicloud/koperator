package tests

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_providerIdentifier(t *testing.T) {
	type args struct {
		node corev1.Node
	}
	tests := []struct {
		name    string
		args    args
		want    k8sProvider
		wantErr error
	}{
		{
			name: "Unrecognized provider",
			args: args{corev1.Node{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "almafa",
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: " v1.24.14",
					},
				},
			}},
			wantErr: errors.New("provider could not been identified"),
		},
		{
			name: "GKE provider",
			args: args{corev1.Node{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "gke-name-pool1-a9e92295-f4ml",
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: "v1.24.14-gke.2700",
					},
				},
			}},
			want:    "GKE",
			wantErr: nil,
		},
		{
			name: "AKS provider",
			args: args{corev1.Node{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "aks-name-pool1-a9e92295-f4ml",
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: "v1.24.14",
					},
				},
			}},
			want:    "AKS",
			wantErr: nil,
		},
		{
			name: "EKS provider",
			args: args{corev1.Node{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "ip-192-168-64-60.eu-west-1.compute.internal",
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: "v1.23.9-eks-ba74326",
					},
				},
			}},
			want:    "EKS",
			wantErr: nil,
		},
		{
			name: "Kind provider",
			args: args{corev1.Node{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kind.node1",
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: "v1.24.14",
					},
				},
			}},
			want:    "Kind",
			wantErr: nil,
		},
		{
			name: "PKE provider",
			args: args{corev1.Node{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "ip-192-168-64-60.eu-west-1.compute.internal",
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: "v1.24.14",
					},
				},
			}},
			want:    "PKE",
			wantErr: nil,
		},
		{
			name: "AKS and EKS provider",
			args: args{corev1.Node{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "aks-name-pool1-a9e92295-f4ml",
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: "v1.23.9-eks-ba74326",
					},
				},
			}},
			want:    "",
			wantErr: errors.New("K8s cluster provider name matched for multiple patterns: 'EKS' and 'AKS'"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := providerIdentifier(tt.args.node)
			if err != nil && tt.wantErr == nil {
				require.NoError(t, err)
			}
			if tt.wantErr != nil {
				require.Error(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_versionIdentifier(t *testing.T) {
	type args struct {
		version string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr error
	}{
		{
			name:    "Unrecognized version",
			args:    args{"123"},
			want:    "",
			wantErr: errors.New("K8s cluster version could not be recognized: '123'"),
		},
		{
			name:    "Standard version format",
			args:    args{"v1.24.14"},
			want:    "v1.24.14",
			wantErr: nil,
		},
		{
			name:    "Subversion format",
			args:    args{"v1.24.14-gke.270"},
			want:    "v1.24.14",
			wantErr: nil,
		},
		{
			name:    "Edge case version format",
			args:    args{"v1.3.0"},
			want:    "v1.3.0",
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := versionIdentifier(tt.args.version)
			if err != nil && tt.wantErr == nil {
				require.NoError(t, err)
			}
			if tt.wantErr != nil {
				require.EqualError(t, err, tt.wantErr.Error())
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
