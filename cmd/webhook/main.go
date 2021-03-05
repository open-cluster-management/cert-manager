/*
(C) Copyright IBM Corporation 2016, 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.

Copyright 2019 The Jetstack cert-manager contributors.

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
	"flag"
	"os"
	"time"

	"github.com/openshift/generic-admission-server/pkg/cmd"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog"

	"github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/logs"
	"github.com/jetstack/cert-manager/pkg/webhook"
	"github.com/jetstack/cert-manager/pkg/webhook/handlers"
)

var (
	GroupName = "webhook." + v1alpha1.SchemeGroupVersion.Group
)

var (
	validationFuncs = map[schema.GroupVersionKind]handlers.ValidationFunc{
		v1alpha1.SchemeGroupVersion.WithKind(v1alpha1.CertificateKind):        webhook.ValidateCertificate,
		v1alpha1.SchemeGroupVersion.WithKind(v1alpha1.CertificateRequestKind): webhook.ValidateCertificateRequest,
		v1alpha1.SchemeGroupVersion.WithKind(v1alpha1.IssuerKind):             webhook.ValidateIssuer,
		v1alpha1.SchemeGroupVersion.WithKind(v1alpha1.ClusterIssuerKind):      webhook.ValidateClusterIssuer,
	}

	// ICP - added authenticators for RBAC
	authenticators = map[schema.GroupVersionKind]handlers.Authenticator{
		v1alpha1.SchemeGroupVersion.WithKind(v1alpha1.CertificateKind): webhook.AuthenticateCertificate,
	}
)

func main() {

	var tlsCertFile, tlsKeyFile, tlsSecurePort string

	flag.StringVar(&tlsCertFile, "tls-cert-file", "", "path to the file containing the TLS certificate to serve with")
	flag.StringVar(&tlsKeyFile, "tls-private-key-file", "", "path to the file containing the TLS private key to serve with")
	flag.StringVar(&tlsSecurePort, "secure-port", "", "the port to use for the secure connection")

	flag.Set("logtostderr", "true")
	flag.Parse()

	if tlsCertFile == "" {
		klog.Info("warning: serving insecurely as tls certificate data not provided")
	} else {
		klog.Info("enabling TLS as certificate file flags specified")
		runfilewatch(tlsCertFile)
	}

	var validationHook cmd.ValidatingAdmissionHook = handlers.NewFuncBackedValidator(logs.Log, GroupName, webhook.Scheme, validationFuncs, authenticators)
	var mutationHook cmd.MutatingAdmissionHook = handlers.NewSchemeBackedDefaulter(logs.Log, GroupName, webhook.Scheme)

	cmd.RunAdmissionServer(
		validationHook,
		mutationHook,
	)
}

func runfilewatch(filename string) {
	info, err := os.Stat(filename)
	if err != nil {
		// missing TLS cert file will get turned into a proper error later
		return
	}
	modtime := info.ModTime()
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			info, err := os.Stat(filename)
			if err != nil {
				continue
			}
			if info.ModTime().After(modtime) {
				// let the k8s scheduler restart us
				// TODO(dmo): figure out if there's a way to do this with clean
				// shutdown
				klog.Infof("Detected change in TLS certificate %s. Restarting to pick up new certificate", filename)
				os.Exit(0)
			}
		}
	}()
}
