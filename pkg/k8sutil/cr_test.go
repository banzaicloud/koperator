// Copyright © 2019 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sutil

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/banzaicloud/koperator/api/v1beta1"
)

func Test_rackAwarenessLabelsToReadonlyConfig(t *testing.T) {
	type args struct {
		pod              *corev1.Pod
		cr               *v1beta1.KafkaCluster
		rackConfigValues []string
	}
	tests := []struct {
		name  string
		args  args
		want  v1beta1.RackAwarenessState
		want1 []v1beta1.Broker
	}{
		{
			name: "rack awareness state not set",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"brokerId": "0",
						},
					},
				},
				cr: &v1beta1.KafkaCluster{
					Spec: v1beta1.KafkaClusterSpec{
						Brokers: []v1beta1.Broker{
							{
								Id: 0,
							},
						},
					},
					Status: v1beta1.KafkaClusterStatus{},
				},
				rackConfigValues: []string{"us-east-2", "us-east-2a"},
			},
			want: "broker.rack=us-east-2,us-east-2a\n",
			want1: []v1beta1.Broker{
				{
					Id:             0,
					ReadOnlyConfig: "broker.rack=us-east-2,us-east-2a\n",
				},
			},
		},
		{
			name: "rack awareness state not set, broker.rack is set in broker read only config",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"brokerId": "0",
						},
					},
				},
				cr: &v1beta1.KafkaCluster{
					Spec: v1beta1.KafkaClusterSpec{
						Brokers: []v1beta1.Broker{
							{
								Id:             0,
								ReadOnlyConfig: "fake-readonly-config\n",
							},
						},
					},
					Status: v1beta1.KafkaClusterStatus{},
				},
				rackConfigValues: []string{"us-east-2", "us-east-2a"},
			},
			want: "broker.rack=us-east-2,us-east-2a\n",
			want1: []v1beta1.Broker{
				{
					Id:             0,
					ReadOnlyConfig: "fake-readonly-config\nbroker.rack=us-east-2,us-east-2a\n",
				},
			},
		},
		{
			name: "rack awareness state is set, broker radonly config not set",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"brokerId": "0",
						},
					},
				},
				cr: &v1beta1.KafkaCluster{
					Spec: v1beta1.KafkaClusterSpec{
						Brokers: []v1beta1.Broker{
							{
								Id: 0,
							},
						},
					},
					Status: v1beta1.KafkaClusterStatus{
						BrokersState: map[string]v1beta1.BrokerState{
							"0": {
								RackAwarenessState: "broker.rack=us-east-2,us-east-2a\n",
							},
						},
					},
				},
				rackConfigValues: []string{},
			},
			want: "broker.rack=us-east-2,us-east-2a\n",
			want1: []v1beta1.Broker{
				{
					Id:             0,
					ReadOnlyConfig: "broker.rack=us-east-2,us-east-2a\n",
				},
			},
		},
		{
			name: "rack awareness state is set, broker readonly config is set",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"brokerId": "0",
						},
					},
				},
				cr: &v1beta1.KafkaCluster{
					Spec: v1beta1.KafkaClusterSpec{
						Brokers: []v1beta1.Broker{
							{
								Id:             0,
								ReadOnlyConfig: "fake-readonly-config\nbroker.rack=us-east-2,us-east-2a\n",
							},
						},
					},
					Status: v1beta1.KafkaClusterStatus{
						BrokersState: map[string]v1beta1.BrokerState{
							"0": {
								RackAwarenessState: "broker.rack=us-east-2,us-east-2a\n",
							},
						},
					},
				},
				rackConfigValues: []string{},
			},
			want: "broker.rack=us-east-2,us-east-2a\n",
			want1: []v1beta1.Broker{
				{
					Id:             0,
					ReadOnlyConfig: "fake-readonly-config\nbroker.rack=us-east-2,us-east-2a\n",
				},
			},
		},
		{
			name: "rack awareness state is set, broker readonly config is set but broker.rack is missing",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"brokerId": "0",
						},
					},
				},
				cr: &v1beta1.KafkaCluster{
					Spec: v1beta1.KafkaClusterSpec{
						Brokers: []v1beta1.Broker{
							{
								Id:             0,
								ReadOnlyConfig: "fake-readonly-config\n",
							},
						},
					},
					Status: v1beta1.KafkaClusterStatus{
						BrokersState: map[string]v1beta1.BrokerState{
							"0": {
								RackAwarenessState: "broker.rack=us-east-2,us-east-2a\n",
							},
						},
					},
				},
				rackConfigValues: []string{},
			},
			want: "broker.rack=us-east-2,us-east-2a\n",
			want1: []v1beta1.Broker{
				{
					Id:             0,
					ReadOnlyConfig: "fake-readonly-config\nbroker.rack=us-east-2,us-east-2a\n",
				},
			},
		},
	}

	t.Parallel()

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			got, got1 := rackAwarenessLabelsToReadonlyConfig(tt.args.pod, tt.args.cr, tt.args.rackConfigValues)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rackAwarenessLabelsToReadonlyConfig() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("rackAwarenessLabelsToReadonlyConfig() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
