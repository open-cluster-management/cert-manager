/*
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

package webhooks

import (
	"crypto/tls"
	"time"
	"fmt"
	"os"
	"bytes"
	"strings"
	"encoding/json"
	"net/http"
	"net/url"
	"io/ioutil"
	"k8s.io/klog"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	restclient "k8s.io/client-go/rest"

	"github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/apis/certmanager/validation"
)
//OIDCTokenResponse is a type
type OIDCTokenResponse struct {
	AccessToken string `json:"access_token"` //"crn:v1:icp:private:k8:mycluster:n/default:::",
	TokenType   string `json:"token_type"`   //"crn:v1:icp:private:k8:mycluster:n/default:::",
	ExpiresIn   string `json:"expires_in"`   //"crn:v1:icp:private:k8:mycluster:n/default:::",
	Expiration  string `json:"expiration"`   //"crn:v1:icp:private:k8:mycluster:n/default:::",
}
type CertificateAdmissionHook struct {
}

func (c *CertificateAdmissionHook) Initialize(kubeClientConfig *restclient.Config, stopCh <-chan struct{}) error {
	klog.Infof("%v", kubeClientConfig)
	return nil
}

func (c *CertificateAdmissionHook) ValidatingResource() (plural schema.GroupVersionResource, singular string) {
	gv := v1alpha1.SchemeGroupVersion
	gv.Group = "admission." + gv.Group
	// override version to be the version of the admissionresponse resource
	gv.Version = "v1beta1"
	return gv.WithResource("certificates"), "certificate"
}

func (c *CertificateAdmissionHook) Validate(admissionSpec *admissionv1beta1.AdmissionRequest) *admissionv1beta1.AdmissionResponse {
	status := &admissionv1beta1.AdmissionResponse{}

	obj := &v1alpha1.Certificate{}
	err := json.Unmarshal(admissionSpec.Object.Raw, obj)
	if err != nil {
		status.Allowed = false
		status.Result = &metav1.Status{
			Status: metav1.StatusFailure, Code: http.StatusBadRequest, Reason: metav1.StatusReasonBadRequest,
			Message: err.Error(),
		}
		return status
	}

	authorized := allowed(admissionSpec, obj)
	if !authorized {
		status.Allowed = false
		status.Result = &metav1.Status{
			Status: metav1.StatusFailure, Code: http.StatusUnauthorized, Reason: metav1.StatusReasonUnauthorized,
			Message: "User is unauthorized to create this certificate with this issuer.",
		}
		return status
	}

	err = validation.ValidateCertificate(obj).ToAggregate()
	if err != nil {
		status.Allowed = false
		status.Result = &metav1.Status{
			Status: metav1.StatusFailure, Code: http.StatusNotAcceptable, Reason: metav1.StatusReasonNotAcceptable,
			Message: err.Error(),
		}
		return status
	}

	status.Allowed = true

	return status
}

func allowed(request *admissionv1beta1.AdmissionRequest, crt *v1alpha1.Certificate) bool {
	issuerKind := crt.Spec.IssuerRef.Kind
	username := request.UserInfo.Username
	uid, err := url.Parse(username)
	if err != nil {
		klog.Infof("An error occurred parsing the username %s to a url: %s", username, err.Error())
		return false
	}
	if issuerKind == "ClusterIssuer" {
		if uid.Fragment != "" {
			// Check if this user is the default cluster admin
			if value, ok := os.LookupEnv("DEFAULT_ADMIN"); ok {
				value = strings.TrimSpace(value)
				if uid.Fragment == value {
					return true
				}
			}
		}
		// If the user is in systems:master group (ClusterAdmin)
		groups := request.UserInfo.Groups
		for _, group := range groups {
			if group == "system:serviceaccounts:cert-manager" || group == "system:masters" {
				return true
			}
		}
		return false
	}
	return true
}
