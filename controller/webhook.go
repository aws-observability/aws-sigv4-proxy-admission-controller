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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corev1Types "k8s.io/client-go/kubernetes/typed/core/v1"
	"log"
	"net/http"
	"os"
	"strings"
)

const (
	signingProxyWebhookAnnotationHostKey    = "sidecar.aws.signing-proxy/host"
	signingProxyWebhookAnnotationInjectKey  = "sidecar.aws.signing-proxy/inject"
	signingProxyWebhookAnnotationNameKey    = "sidecar.aws.signing-proxy/name"
	signingProxyWebhookAnnotationRegionKey  = "sidecar.aws.signing-proxy/region"
	signingProxyWebhookAnnotationRoleArnKey = "sidecar.aws.signing-proxy/role-arn"
	signingProxyWebhookAnnotationStatusKey  = "sidecar.aws.signing-proxy/status"
	signingProxyWebhookLabelHostKey         = "sidecar-host"
	signingProxyWebhookLabelNameKey         = "sidecar-name"
	signingProxyWebhookLabelRegionKey       = "sidecar-region"
	signingProxyWebhookLabelRoleArnKey      = "sidecar-role-arn"
)

var (
	namespaceSelector = []metav1.LabelSelector{{
		MatchLabels: map[string]string{"sidecar-inject": "true"},
	}}
)

type WebhookServer struct {
	server          *http.Server
	namespaceClient KubernetesNamespaceClient
}

type KubernetesNamespaceClient interface {
	corev1Types.NamespaceInterface
}

type PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func NewWebhookServer(server *http.Server, k8sClient *kubernetes.Clientset) *WebhookServer {
	return &WebhookServer{
		server:          server,
		namespaceClient: k8sClient.CoreV1().Namespaces(),
	}
}

func (whsvr *WebhookServer) Handler(writer http.ResponseWriter, request *http.Request) {
	if request.Body == nil {
		fmt.Errorf("Error: empty request body")
		http.Error(writer, "Empty request body", http.StatusBadRequest)
		return
	}

	if request.Header.Get("Content-Type") != "application/json" {
		fmt.Errorf("Invalid Content-Type %s, expected application/json", request.Header.Get("Content-Type"))
		http.Error(writer, "Invalid Content-Type, expected application/json", http.StatusUnsupportedMediaType)
		return
	}

	body, err := ioutil.ReadAll(request.Body)

	if err != nil {
		fmt.Errorf("Error reading body: %v", err)
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	admissionReview := v1beta1.AdmissionReview{}

	err = json.Unmarshal(body, &admissionReview)

	if err != nil {
		fmt.Errorf("Error unmarshaling body: %v", err)
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var admissionResponse *v1beta1.AdmissionResponse

	admissionResponse, err = whsvr.mutate(request.Context(), &admissionReview)

	if err != nil {
		fmt.Errorf("Error mutating AdmissionReview: %v", err)
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
	}

	response, err := json.Marshal(admissionReview)

	if err != nil {
		fmt.Errorf("Error encoding response: %v", err)
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if _, err := writer.Write(response); err != nil {
		fmt.Errorf("Error writing response: %v", err)
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func (whsvr *WebhookServer) mutate(ctx context.Context, admissionReview *v1beta1.AdmissionReview) (*v1beta1.AdmissionResponse, error) {
	admissionRequest := admissionReview.Request

	var pod corev1.Pod

	if err := json.Unmarshal(admissionRequest.Object.Raw, &pod); err != nil {
		return &v1beta1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}}, fmt.Errorf("Error unmarshaling AdmissionRequest into Pod: %v", err)
	}

	nsLabels, err := whsvr.describeNamespace(ctx, admissionRequest.Namespace)

	if err != nil {
		return &v1beta1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}}, fmt.Errorf("Error describing namespace: %v", err)
	}

	if !whsvr.shouldMutate(nsLabels, &pod.ObjectMeta) {
		return &v1beta1.AdmissionResponse{Allowed: true, UID: admissionRequest.UID}, nil
	}

	var patchOperations []PatchOperation

	host, name, region := whsvr.getUpstreamEndpointParameters(nsLabels, &pod.ObjectMeta)

	sidecarArgs := []string{"--name", name, "--region", region, "--host", host, "--port", ":8005"}

	roleArn := whsvr.getRoleArn(nsLabels, &pod.ObjectMeta)

	if roleArn != "" {
		sidecarArgs = append(sidecarArgs, "--role-arn", roleArn)
	}

	image := whsvr.getProxyImage()

	sidecarContainer := []corev1.Container{{
		Name:            "sidecar-aws-sigv4-proxy",
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Ports: []corev1.ContainerPort{{
			ContainerPort: 8005,
		}},
		Args: sidecarArgs,
	}}

	patchOperations = append(patchOperations, addContainers(pod.Spec.Containers, sidecarContainer, "/spec/containers")...)

	annotations := map[string]string{signingProxyWebhookAnnotationStatusKey: "injected"}

	patchOperations = append(patchOperations, updateAnnotations(pod.Annotations, annotations)...)

	patchBytes, err := json.Marshal(patchOperations)

	if err != nil {
		return &v1beta1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}}, fmt.Errorf("Error unmarshaling AdmissionRequest into Pod: %v", err)
	}

	log.Printf("Admission Response: %v", string(patchBytes))

	return &v1beta1.AdmissionResponse{
		Allowed: true,
		UID:     admissionRequest.UID,
		Patch:   patchBytes,
		PatchType: func() *v1beta1.PatchType {
			pt := v1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}, nil
}

func (whsvr *WebhookServer) describeNamespace(ctx context.Context, namespace string) (map[string]string, error) {
	ns, err := whsvr.namespaceClient.Get(ctx, namespace, metav1.GetOptions{})

	if err != nil {
		return nil, fmt.Errorf("Error describing namespace: %v", err)
	}

	log.Printf("Namespace labels: %s", ns.Labels)

	return ns.Labels, nil
}

func (whsvr *WebhookServer) shouldMutate(nsLabels map[string]string, podMetadata *metav1.ObjectMeta) bool {
	annotations := podMetadata.GetAnnotations()

	if annotations == nil {
		annotations = map[string]string{}
	}

	if annotations[signingProxyWebhookAnnotationStatusKey] == "injected" {
		return false
	}

	if annotations[signingProxyWebhookAnnotationHostKey] == "" && nsLabels[signingProxyWebhookLabelHostKey] == "" {
		return false
	}

	var annotationInject bool
	var annotationReject bool

	switch strings.ToLower(annotations[signingProxyWebhookAnnotationInjectKey]) {
	case "y", "yes", "true", "on":
		annotationInject = true
	case "n", "no", "false", "off":
		annotationReject = true
	}

	var labelInject bool

	for _, nsSelector := range namespaceSelector {
		selector, err := metav1.LabelSelectorAsSelector(&nsSelector)

		if err != nil {
			fmt.Errorf("Invalid selector for NamespaceSelector")
			return false
		} else if !selector.Empty() && selector.Matches(labels.Set(nsLabels)) {
			labelInject = true
		} else if !annotationInject {
			return false
		}
	}

	if labelInject {
		return !annotationReject
	}

	return annotationInject
}

func (whsvr *WebhookServer) getUpstreamEndpointParameters(nsLabels map[string]string, podMetadata *metav1.ObjectMeta) (string, string, string) {
	annotations := podMetadata.GetAnnotations()

	if annotations == nil {
		annotations = map[string]string{}
	}

	host := annotations[signingProxyWebhookAnnotationHostKey]

	var labelInject bool

	if strings.TrimSpace(host) == "" {
		labelInject = true
		host = nsLabels[signingProxyWebhookLabelHostKey]
	}

	if labelInject {
		return extractParameters(host, nsLabels[signingProxyWebhookLabelNameKey], nsLabels[signingProxyWebhookLabelRegionKey])
	}

	return extractParameters(host, annotations[signingProxyWebhookAnnotationNameKey], annotations[signingProxyWebhookAnnotationRegionKey])
}

func extractParameters(host string, name string, region string) (string, string, string) {
	if strings.TrimSpace(name) == "" {
		name = host[:strings.IndexByte(host, '.')]
	}

	hostModified := host[strings.IndexByte(host, '.')+1:]

	if strings.TrimSpace(region) == "" {
		region = hostModified[:strings.IndexByte(hostModified, '.')]
	}

	return host, name, region
}

func (whsvr *WebhookServer) getRoleArn(nsLabels map[string]string, podMetadata *metav1.ObjectMeta) string {
	annotations := podMetadata.GetAnnotations()

	if annotations == nil {
		annotations = map[string]string{}
	}

	roleArn := annotations[signingProxyWebhookAnnotationRoleArnKey]

	if strings.TrimSpace(roleArn) == "" {
		roleArn = nsLabels[signingProxyWebhookLabelRoleArnKey]
	}

	return roleArn
}

func (whsvr *WebhookServer) getProxyImage() string {
	image := os.Getenv("AWS-SIGV4-PROXY-IMAGE")

	if image == "" {
		image = "public.ecr.aws/aws-observability/aws-sigv4-proxy:latest"
	}

	return image
}

func addContainers(target, containers []corev1.Container, basePath string) (patch []PatchOperation) {
	first := len(target) == 0

	var value interface{}

	for _, container := range containers {
		value = container
		path := basePath

		if first {
			first = false
			value = []corev1.Container{container}
		} else {
			path += "/-"
		}

		patch = append(patch, PatchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}

	return patch
}

func updateAnnotations(target map[string]string, annotations map[string]string) (patch []PatchOperation) {
	for key, value := range annotations {
		op := "replace"
		if target == nil || target[key] == "" {
			op = "add"
		}
		patch = append(patch, PatchOperation{
			Op:    op,
			Path:  "/metadata/annotations/" + strings.ReplaceAll(key, "/", "~1"),
			Value: value,
		})
	}

	return patch
}
