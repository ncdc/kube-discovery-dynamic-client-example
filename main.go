/*
Copyright 2023 Andy Goldstein

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(flag.CommandLine)

	kubeconfig := flag.String("kubeconfig", "", "Path to kubeconfig file.")
	labelSelector := flag.String("selector", "", "Label selector.")

	flag.Parse()

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != nil {
		loadingRules.ExplicitPath = *kubeconfig
	}

	startingConfig, err := loadingRules.GetStartingConfig()
	checkError(err, "error getting starting config")

	clientConfig := clientcmd.NewDefaultClientConfig(*startingConfig, nil)
	restConfig, err := clientConfig.ClientConfig()
	checkError(err, "error getting REST config")
	restConfig.QPS = 50
	restConfig.Burst = 100

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	checkError(err, "error creating discovery client")

	resourceLists, err := discoveryClient.ServerPreferredResources()
	checkError(err, "error getting server preferred resources")

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	checkError(err, "error creating dynamic client")

	ctx := context.Background()
	listOptions := metav1.ListOptions{}
	if labelSelector != nil {
		listOptions.LabelSelector = *labelSelector
	}

	for _, resourceList := range resourceLists {
		gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		checkError(err, fmt.Sprintf("error parsing GroupVersion %s", resourceList.GroupVersion))
		for _, resource := range resourceList.APIResources {
			supportedVerbs := sets.NewString(resource.Verbs...)
			if !supportedVerbs.Has("list") {
				continue
			}
			gvr := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: resource.Name,
			}

			fmt.Printf("Processing %s\n", gvr)
			list, err := dynamicClient.Resource(gvr).List(ctx, listOptions)
			checkError(err, fmt.Sprintf("error listing %s", gvr))

			for _, item := range list.Items {
				if item.GetNamespace() != "" {
					fmt.Printf("Namespace: %s\n", item.GetNamespace())
				}
				fmt.Printf("Name: %s\n\n", item.GetName())
			}
		}
	}
}

func checkError(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
		os.Exit(1)
	}
}
