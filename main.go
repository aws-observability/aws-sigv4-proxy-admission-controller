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

package main

import (
	"aws-signingproxy-admissioncontroller/controller"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"os"
	signal "os/signal"
	"syscall"
	"time"
)

type WhSvrParameters struct {
	port           int    // Webhook server port
	certFile       string // Path to the x509 HTTPS certificate
	keyFile        string // Path to the x509 private key matching the certFile
}

func main() {
	var parameters WhSvrParameters

	flag.IntVar(&parameters.port, "port", 443, "Webhook server port.")
	flag.StringVar(&parameters.certFile, "tlsCertFile", "/etc/webhook/certs/cert.pem", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&parameters.keyFile, "tlsKeyFile", "/etc/webhook/certs/key.pem", "File containing the x509 private key to --tlsCertFile.")
	flag.Parse()

	keyPair, err := tls.LoadX509KeyPair(parameters.certFile, parameters.keyFile)
	if err != nil {
		fmt.Errorf("Error loading key pair: %v", err)
	}

	server := &http.Server{
		Addr:      fmt.Sprintf(":%v", parameters.port),
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{keyPair}},
	}

	client, err := newKubernetesClient()

	if err != nil {
		fmt.Errorf("Error creating Kubernetes client: %v", err)
	}

	whsvr := controller.NewWebhookServer(server, client)

	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", whsvr.Handler)
	server.Handler = mux

	go func() {
		if err := server.ListenAndServeTLS("", ""); err != nil {
			fmt.Errorf("Error listening and serving webhook server: %v", err)
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	log.Println("Got OS shutdown signal, shutting down webhook server gracefully")

	shutdownCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	
	server.Shutdown(shutdownCtx)
}

func newKubernetesClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()

	if err != nil {
		return nil, fmt.Errorf("Error initializing Kubernetes client: %v", err)
	}

	client, err := kubernetes.NewForConfig(config)

	if err != nil {
		return nil, fmt.Errorf("Error describing namespace: %v", err)
	}

	return client, nil
}
