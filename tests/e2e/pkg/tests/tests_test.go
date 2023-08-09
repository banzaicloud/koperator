// Copyright © 2023 Cisco Systems, Inc. and/or its affiliates
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

package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Classifier_minimal(t *testing.T) {
	type fields struct {
		k8sClusterPool K8sClusterPool
		testCases      []TestCase
	}
	tests := []struct {
		name   string
		fields fields
		want   TestPool
	}{
		{
			name: "simpleCase",
			fields: fields{
				k8sClusterPool: K8sClusterPool{
					{
						clusterInfo: k8sClusterInfo{
							clusterID: "local1",
							version:   "1.24",
							provider:  "provider1",
						},
					},
					{
						clusterInfo: k8sClusterInfo{
							clusterID: "local2",
							version:   "1.25",
							provider:  "provider1",
						},
					},
				},
				testCases: []TestCase{
					{
						Name: "testCase1",
					},
					{
						Name: "testCase2",
					},
				},
			},
			want: []Test{
				{
					testCase: TestCase{
						Name: "testCase1",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local1",
							version:   "1.24",
							provider:  "provider1",
						},
					},
				},
				{
					testCase: TestCase{
						Name: "testCase2",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local2",
							version:   "1.25",
							provider:  "provider1",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := Classifier{
				k8sClusterPool: tt.fields.k8sClusterPool,
				testCases:      tt.fields.testCases,
			}

			got := tr.Minimal()
			if !got.Equal(tt.want) {
				t.Errorf("want: %v\ngot: %v", tt.want, got)
			}
		})
	}
}

func Test_Classifier_providerComplete(t *testing.T) {
	type fields struct {
		k8sClusterPool K8sClusterPool
		testCases      []TestCase
	}
	tests := []struct {
		name   string
		fields fields
		want   []Test
	}{
		{
			name: "simpleCase",
			fields: fields{
				k8sClusterPool: K8sClusterPool{
					{

						clusterInfo: k8sClusterInfo{
							clusterID: "local1",
							version:   "1.24",
							provider:  "provider1",
						},
					},
					{

						clusterInfo: k8sClusterInfo{
							clusterID: "local2",
							version:   "1.25",
							provider:  "provider2",
						},
					},
				},
				testCases: []TestCase{
					{
						Name: "testCase1",
					},
					{
						Name: "testCase2",
					},
				},
			},
			want: []Test{
				{
					testCase: TestCase{
						Name: "testCase1",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local1",
							version:   "1.24",
							provider:  "provider1",
						},
					},
				},
				{
					testCase: TestCase{
						Name: "testCase2",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local1",
							version:   "1.24",
							provider:  "provider1",
						},
					},
				},
				{
					testCase: TestCase{
						Name: "testCase1",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local2",
							version:   "1.25",
							provider:  "provider2",
						},
					},
				},
				{
					testCase: TestCase{
						Name: "testCase2",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local2",
							version:   "1.25",
							provider:  "provider2",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := Classifier{
				k8sClusterPool: tt.fields.k8sClusterPool,
				testCases:      tt.fields.testCases,
			}

			got := tr.ProviderComplete()
			if !got.Equal(tt.want) {
				t.Errorf("want: %v\ngot: %v", tt.want, got)
			}
		})
	}
}

func Test_Classifier_versionComplete(t *testing.T) {
	type fields struct {
		k8sClusterPool K8sClusterPool
		testCases      []TestCase
	}
	tests := []struct {
		name   string
		fields fields
		want   []Test
	}{
		{
			name: "simpleCase",
			fields: fields{
				k8sClusterPool: K8sClusterPool{
					{

						clusterInfo: k8sClusterInfo{
							clusterID: "local1",
							version:   "1.24",
							provider:  "provider1",
						},
					},
					{

						clusterInfo: k8sClusterInfo{
							clusterID: "local1",
							version:   "1.25",
							provider:  "provider1",
						},
					},
				},
				testCases: []TestCase{
					{
						Name: "testCase1",
					},
					{
						Name: "testCase2",
					},
				},
			},
			want: []Test{
				{
					testCase: TestCase{
						Name: "testCase1",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local1",
							version:   "1.24",
							provider:  "provider1",
						},
					},
				},
				{
					testCase: TestCase{
						Name: "testCase2",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local1",
							version:   "1.24",
							provider:  "provider1",
						},
					},
				},
				{
					testCase: TestCase{
						Name: "testCase1",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local1",
							version:   "1.24",
							provider:  "provider1",
						},
					},
				},
				{
					testCase: TestCase{
						Name: "testCase2",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local1",
							version:   "1.24",
							provider:  "provider1",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := Classifier{
				k8sClusterPool: tt.fields.k8sClusterPool,
				testCases:      tt.fields.testCases,
			}

			got := tr.VersionComplete()
			if !got.Equal(tt.want) {
				t.Errorf("want: %v\ngot: %v", tt.want, got)
			}
		})
	}
}

func Test_Classifier_complete(t *testing.T) {
	type fields struct {
		k8sClusterPool K8sClusterPool
		testCases      []TestCase
	}
	tests := []struct {
		name   string
		fields fields
		want   []Test
	}{
		{
			name: "simpleCase",
			fields: fields{
				k8sClusterPool: K8sClusterPool{
					{

						clusterInfo: k8sClusterInfo{
							clusterID: "local1",
							version:   "1.24",
							provider:  "provider1",
						},
					},
					{

						clusterInfo: k8sClusterInfo{
							clusterID: "local2",
							version:   "1.25",
							provider:  "provider2",
						},
					},
				},
				testCases: []TestCase{
					{
						Name: "testCase1",
					},
					{
						Name: "testCase2",
					},
				},
			},
			want: []Test{
				{
					testCase: TestCase{
						Name: "testCase1",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local1",
							version:   "1.24",
							provider:  "provider1",
						},
					},
				},
				{
					testCase: TestCase{
						Name: "testCase2",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local1",
							version:   "1.24",
							provider:  "provider1",
						},
					},
				},
				{
					testCase: TestCase{
						Name: "testCase1",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local2",
							version:   "1.25",
							provider:  "provider2",
						},
					},
				},
				{
					testCase: TestCase{
						Name: "testCase2",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local2",
							version:   "1.25",
							provider:  "provider2",
						},
					},
				},
			},
		},
		{
			name: "complexCase",
			fields: fields{
				k8sClusterPool: K8sClusterPool{
					{

						clusterInfo: k8sClusterInfo{
							clusterID: "local1",
							version:   "1.24",
							provider:  "provider1",
						},
					},
					{

						clusterInfo: k8sClusterInfo{
							clusterID: "local2",
							version:   "1.25",
							provider:  "provider2",
						},
					},
					{

						clusterInfo: k8sClusterInfo{
							clusterID: "local3",
							version:   "1.25",
							provider:  "provider3",
						},
					},
					{

						clusterInfo: k8sClusterInfo{
							clusterID: "local4",
							version:   "1.25",
							provider:  "provider3",
						},
					},
				},
				testCases: []TestCase{
					{
						Name: "testCase1",
					},
					{
						Name: "testCase2",
					},
				},
			},
			want: []Test{
				{
					testCase: TestCase{
						Name: "testCase1",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local1",
							version:   "1.24",
							provider:  "provider1",
						},
					},
				},
				{
					testCase: TestCase{
						Name: "testCase2",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local1",
							version:   "1.24",
							provider:  "provider1",
						},
					},
				},
				{
					testCase: TestCase{
						Name: "testCase1",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local2",
							version:   "1.25",
							provider:  "provider2",
						},
					},
				},
				{
					testCase: TestCase{
						Name: "testCase2",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local2",
							version:   "1.25",
							provider:  "provider2",
						},
					},
				},
				{
					testCase: TestCase{
						Name: "testCase1",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local3",
							version:   "1.25",
							provider:  "provider3",
						},
					},
				},
				{
					testCase: TestCase{
						Name: "testCase2",
					},
					k8sCluster: K8sCluster{

						clusterInfo: k8sClusterInfo{
							clusterID: "local3",
							version:   "1.25",
							provider:  "provider3",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := Classifier{
				k8sClusterPool: tt.fields.k8sClusterPool,
				testCases:      tt.fields.testCases,
			}

			got := tr.Complete()
			if !got.Equal(tt.want) {
				t.Errorf("want: %v\ngot: %v", tt.want, got)
			}
		})
	}
}

func TestTestPool_GetTestSuiteDurationParallel(t *testing.T) {
	tests := []struct {
		name  string
		tests TestPool
		want  time.Duration
	}{
		{
			name:  "MockTestsProvider",
			tests: MockTestsProvider().ProviderComplete(),
			want:  5 * time.Second,
		},
		{
			name:  "MockTestsProviderMoreTestsThenProvider",
			tests: MockTestsProviderMoreTestsThenProvider().ProviderComplete(),
			want:  9 * time.Second,
		},

		{
			name:  "MockTestsVersionOne",
			tests: MockTestsVersionOne().VersionComplete(),
			want:  3 * time.Second,
		},
		{
			name:  "MockTestsComplete",
			tests: MockTestsComplete().Complete(),
			want:  5 * time.Second,
		},
		{
			name:  "MockTestsVersion",
			tests: MockTestsVersion().VersionComplete(),
			want:  5 * time.Second,
		},
		{
			name:  "MockTestsMinimal",
			tests: MockTestsMinimal().Minimal(),
			want:  3 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tests.GetTestSuiteDurationParallel()
			assert.Equal(t, tt.want, got)
		})
	}
}
