/*
 * Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License").
 * You may not use this file except in compliance with the License.
 * A copy of the License is located at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * or in the "license" file accompanying this file. This file is distributed
 * on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
 * express or implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

package controller

import (
	"aws-signingproxy-admissioncontroller/controller/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestWebhookServer_describeNamespace(t *testing.T) {
	mockKubernetesClient := &mocks.KubernetesNamespaceClient{}
	labels := map[string]string{"Key": "Value"}

	mockKubernetesClient.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(
		&corev1.Namespace{TypeMeta: metav1.TypeMeta{}, ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
		}}, nil)

	t.Run("TestNamespaceLabelsMatch", func(t *testing.T) {
		whsvr := &WebhookServer{
			server:          nil,
			namespaceClient: mockKubernetesClient,
		}
		l, err := whsvr.describeNamespace(nil, "testNamespace")
		assert.Nil(t, err, "Should succeed")
		assert.Equal(t, l, labels, "Labels should match")
	})

	wrongLabels := map[string]string{"Key": "WrongValue"}

	t.Run("TestNamespaceLabelsDoNotMatch", func(t *testing.T) {
		whsvr := &WebhookServer{
			server:          nil,
			namespaceClient: mockKubernetesClient,
		}
		l, err := whsvr.describeNamespace(nil, "testNamespace")
		assert.Nil(t, err, "Should succeed")
		assert.NotEqual(t, l, wrongLabels, "Labels should not match")
	})

	emptyKubernetesClient := &mocks.KubernetesNamespaceClient{}
	emptyLabels := map[string]string{}

	emptyKubernetesClient.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(
		&corev1.Namespace{TypeMeta: metav1.TypeMeta{}, ObjectMeta: metav1.ObjectMeta{
			Labels: emptyLabels,
		}}, nil)

	t.Run("TestNamespaceLabelsEmpty", func(t *testing.T) {
		whsvr := &WebhookServer{
			server:          nil,
			namespaceClient: emptyKubernetesClient,
		}
		l, err := whsvr.describeNamespace(nil, "testNamespace")
		assert.Nil(t, err, "Should succeed")
		assert.Empty(t, l, "Labels should be empty")
	})
}

func TestWebhookServer_shouldMutate(t *testing.T) {
	var positiveTestCases = []struct {
		name          string
		podObjectMeta *metav1.ObjectMeta
		labels        map[string]string
		errorMessage  string
	} {
		{
			name: "TestSidecarInjectCorrectAnnotation",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{signingProxyWebhookAnnotationInjectKey: "true", signingProxyWebhookAnnotationHostKey: "random"},
			},
			labels: map[string]string{"Key": "Value"},
			errorMessage: "Should inject sidecar - correct annotation",
		},
		{
			name: "TestSidecarInjectMatchLabels",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{signingProxyWebhookAnnotationHostKey: "random"},
			},
			labels: map[string]string{"sidecar-inject": "true"},
			errorMessage: "Should inject sidecar - matching labels",
		},
		{
			name: "TestSidecarInjectAnnotationMatchLabels",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{signingProxyWebhookAnnotationInjectKey: "true", signingProxyWebhookAnnotationHostKey: "random"},
			},
			labels: map[string]string{"sidecar-inject": "true"},
			errorMessage: "Should inject sidecar - annotation and matching namespace label",
		},
		{
			name: "TestSidecarInjectWithHostLabelAndNoHostAnnotation",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{signingProxyWebhookAnnotationInjectKey: "true"},
			},
			labels: map[string]string{"sidecar-host": "random-host"},
			errorMessage: "Should inject sidecar - there is a host label but no host annotation",
		},
	}

	var negativeTestCases = []struct {
		name          string
		podObjectMeta *metav1.ObjectMeta
		labels        map[string]string
		errorMessage  string
	} {
		{
			name: "TestSidecarInjectIncorrectAnnotation",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{"hello": "world", signingProxyWebhookAnnotationHostKey: "random"},
			},
			labels: map[string]string{"Key": "Value"},
			errorMessage: "Should not inject sidecar - incorrect annotation",
		},
		{
			name: "TestSidecarInjectMismatchLabels",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{signingProxyWebhookAnnotationHostKey: "random"},
			},
			labels: map[string]string{"Key": "Value"},
			errorMessage: "Should not inject sidecar - mismatching labels",
		},
		{
			name: "TestSidecarInjectAnnotationRejection",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{signingProxyWebhookAnnotationInjectKey: "false", signingProxyWebhookAnnotationHostKey: "random"},
			},
			labels: map[string]string{"sidecar-inject": "true"},
			errorMessage: "Should not inject sidecar - annotation rejection",
		},
		{
			name: "TestSidecarInjectNoHostAnnotationOrLabel",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{signingProxyWebhookAnnotationInjectKey: "true"},
			},
			labels: map[string]string{"sidecar-inject": "true"},
			errorMessage: "Should not inject sidecar - no host annotation or label",
		},
	}

	for _, tc := range positiveTestCases {
		t.Run(tc.name, func(t *testing.T) {
			whsvr := &WebhookServer{
				server:          nil,
				namespaceClient: nil,
			}

			b := whsvr.shouldMutate(tc.labels, tc.podObjectMeta)
			assert.True(t, b, tc.errorMessage)
		})
	}

	for _, tc := range negativeTestCases {
		t.Run(tc.name, func(t *testing.T) {
			whsvr := &WebhookServer{
				server:          nil,
				namespaceClient: nil,
			}

			b := whsvr.shouldMutate(tc.labels, tc.podObjectMeta)
			assert.False(t, b, tc.errorMessage)
		})
	}
}

func TestWebhookServer_getUpstreamEndpointParameters(t *testing.T) {
	var testCases = []struct {
		name          string
		podObjectMeta *metav1.ObjectMeta
		labels        map[string]string
		expected      []string
		errorMessages []string
	} {
		{
			name: "TestSidecarAllAnnotationsPresent",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					signingProxyWebhookAnnotationHostKey: "annotation.us-west-2.amazonaws.com",
					signingProxyWebhookAnnotationNameKey: "annotationName",
					signingProxyWebhookAnnotationRegionKey: "us-west-2-region",
				},
			},
			labels: map[string]string{},
			expected: []string{"annotation.us-west-2.amazonaws.com", "annotationName", "us-west-2-region"},
			errorMessages: []string{"Should return host annotation value", "Should return name annotation value", "Should return region annotation value"},
		},
		{
			name: "TestSidecarRegionAnnotationNotPresent",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					signingProxyWebhookAnnotationHostKey: "annotation.us-west-2.amazonaws.com",
					signingProxyWebhookAnnotationNameKey: "annotationName",
				},
			},
			labels: map[string]string{},
			expected: []string{"annotation.us-west-2.amazonaws.com", "annotationName", "us-west-2"},
			errorMessages: []string{"Should return host annotation value", "Should return name annotation value", "Should return region from host annotation"},
		},
		{
			name: "TestSidecarNameAnnotationNotPresent",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					signingProxyWebhookAnnotationHostKey: "annotation.us-west-2.amazonaws.com",
					signingProxyWebhookAnnotationRegionKey: "us-west-2-region",
				},
			},
			labels: map[string]string{},
			expected: []string{"annotation.us-west-2.amazonaws.com", "annotation", "us-west-2-region"},
			errorMessages: []string{"Should return host annotation value", "Should return name from host annotation", "Should return region annotation value"},
		},
		{
			name: "TestSidecarNameRegionAnnotationsNotPresent",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					signingProxyWebhookAnnotationHostKey: "annotation.us-west-2.amazonaws.com",
				},
			},
			labels: map[string]string{},
			expected: []string{"annotation.us-west-2.amazonaws.com", "annotation", "us-west-2"},
			errorMessages: []string{"Should return host annotation value", "Should return name from host annotation", "Should return region from host annotation"},
		},
		{
			name: "TestSidecarAllAnnotationsAndLabelsPresent",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					signingProxyWebhookAnnotationHostKey: "annotation.us-west-2.amazonaws.com",
					signingProxyWebhookAnnotationNameKey: "annotationName",
					signingProxyWebhookAnnotationRegionKey: "us-west-2-region",
				},
			},
			labels: map[string]string{
				signingProxyWebhookLabelHostKey: "label.us-east-2.amazonaws.com",
				signingProxyWebhookLabelNameKey: "labelName",
				signingProxyWebhookLabelRegionKey: "us-east-2-region",
			},
			expected: []string{"annotation.us-west-2.amazonaws.com", "annotationName", "us-west-2-region"},
			errorMessages: []string{"Should return host annotation value", "Should return name annotation value", "Should return region annotation value"},
		},
		{
			name: "TestSidecarAllLabelsPresent",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{},
			},
			labels: map[string]string{
				signingProxyWebhookLabelHostKey: "label.us-east-2.amazonaws.com",
				signingProxyWebhookLabelNameKey: "labelName",
				signingProxyWebhookLabelRegionKey: "us-east-2-region",
			},
			expected: []string{"label.us-east-2.amazonaws.com", "labelName", "us-east-2-region"},
			errorMessages: []string{"Should return host label value", "Should return name label value", "Should return region label value"},
		},
		{
			name: "TestSidecarOnlyHostLabelsPresent",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{},
			},
			labels: map[string]string{
				signingProxyWebhookLabelHostKey: "label.us-east-2.amazonaws.com",
			},
			expected: []string{"label.us-east-2.amazonaws.com", "label", "us-east-2"},
			errorMessages: []string{"Should return host label value", "Should return name from host label", "Should return region from host label"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			whsvr := &WebhookServer{
				server:          nil,
				namespaceClient: nil,
			}

			a, b, c := whsvr.getUpstreamEndpointParameters(tc.labels, tc.podObjectMeta)
			assert.Equal(t, tc.expected[0], a, tc.errorMessages[0])
			assert.Equal(t, tc.expected[1], b, tc.errorMessages[1])
			assert.Equal(t, tc.expected[2], c, tc.errorMessages[2])
		})
	}
}

func TestWebhookServer_getRoleArn(t *testing.T) {
	var testCases = []struct {
		name          string
		podObjectMeta *metav1.ObjectMeta
		labels        map[string]string
		expected      string
		errorMessage  string
	} {
		{
			name: "TestSidecarRoleArnAnnotationPresent",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					signingProxyWebhookAnnotationRoleArnKey: "arn:aws:iam::123456789:annotation/assume-role-test",
				},
			},
			labels: map[string]string{},
			expected: "arn:aws:iam::123456789:annotation/assume-role-test",
			errorMessage: "Should return role-arn annotation value",
		},
		{
			name: "TestSidecarRoleArnLabelPresent",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{},
			},
			labels: map[string]string{
				signingProxyWebhookLabelRoleArnKey: "arn:aws:iam::123456789:label/assume-role-test",
			},
			expected: "arn:aws:iam::123456789:label/assume-role-test",
			errorMessage: "Should return role-arn label value",
		},
		{
			name: "TestSidecarNoRoleArnAnnotationPresent",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{},
			},
			labels: map[string]string{},
			expected: "",
			errorMessage: "Should return empty role-arn since there is no annotation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			whsvr := &WebhookServer{
				server:          nil,
				namespaceClient: nil,
			}

			r := whsvr.getRoleArn(tc.labels, tc.podObjectMeta)
			assert.Equal(t, tc.expected, r, tc.errorMessage)
		})
	}
}

func TestWebhookServer_getResourceRequirements(t *testing.T) {
	var testCases = []struct {
		name          string
		podObjectMeta *metav1.ObjectMeta
		labels        map[string]string
		expected      *corev1.ResourceRequirements
		errorMessage  string
	}{
		{
			name: "TestSidecarResourceAnnotationPresent",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					signingProxyWebhookAnnotationCPURequestKey: "200m",
					signingProxyWebhookAnnotationMemRequestKey: "200Mi",
					signingProxyWebhookAnnotationCPULimitKey:   "400m",
					signingProxyWebhookAnnotationMemLimitKey:   "400Mi",
				},
			},
			expected: &corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("200Mi"),
				},
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("400m"),
					corev1.ResourceMemory: resource.MustParse("400Mi"),
				},
			},
			errorMessage: "Should return ResourceRequiremts value",
		},
		{
			name: "TestSidecarResourceAnnotationRequestsCPUPresent",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					signingProxyWebhookAnnotationCPURequestKey: "200m",
				},
			},
			expected: &corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("200m"),
				},
			},
			errorMessage: "Should return ResourceRequestsCPU only",
		},
		{
			name: "TestSidecarResourceAnnotationLimitCPUPreset",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					signingProxyWebhookAnnotationCPULimitKey: "400m",
				},
			},
			expected: &corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("400m"),
				},
			},
			errorMessage: "Should return ResourceLimitCPU only",
		},
		{
			name: "TestSidecarNoResourceAnnotationPresent",
			podObjectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{},
			},
			expected:     nil,
			errorMessage: "Should return nil",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			whsvr := &WebhookServer{
				server:          nil,
				namespaceClient: nil,
			}

			r, err := whsvr.getResourceRequirements(tc.podObjectMeta)
			assert.Equal(t, tc.expected, r, tc.errorMessage)
			assert.Nil(t, err, "Should succeed")
		})
	}
}
