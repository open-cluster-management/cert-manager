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
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"k8s.io/klog"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	authorizationv1 "k8s.io/api/authorization/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	authclientv1beta1 "k8s.io/client-go/kubernetes/typed/authorization/v1beta1"
	restclient "k8s.io/client-go/rest"

	"github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/apis/certmanager/validation"
)

type CertificateAdmissionHook struct {
	authClient *authclientv1beta1.AuthorizationV1beta1Client
}

func (c *CertificateAdmissionHook) Initialize(kubeClientConfig *restclient.Config, stopCh <-chan struct{}) error {
	c.authClient, _ = authclientv1beta1.NewForConfig(kubeClientConfig)
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

	authorized := allowed(admissionSpec, obj, c.authClient)
	if !authorized {
		timeStamp := time.Now()
		message := fmt.Sprintf("User: %s is not allowed to use the ClusterIssuer %s to sign the Certificate %s.", admissionSpec.UserInfo.Username, obj.Spec.IssuerRef.Name, obj.ObjectMeta.Name)
		klog.Errorf("[UNAUTHORIZED] %s\n%s", timeStamp.String(), message)

		status.Allowed = false
		status.Result = &metav1.Status{
			Status: metav1.StatusFailure, Code: http.StatusForbidden, Reason: metav1.StatusReasonForbidden,
			Message: message,
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

// Checks if the user requesting to create this Certificate is allowed to
// Returns true if they are, returns false otherwise
func allowed(request *admissionv1beta1.AdmissionRequest, crt *v1alpha1.Certificate, authClient *authclientv1beta1.AuthorizationV1beta1Client) bool {
	issuerKind := crt.Spec.IssuerRef.Kind

	// Check authorization if the Certificate is to be issued/signed by a ClusterIssuer
	if issuerKind == "ClusterIssuer" {
		username := request.UserInfo.Username
		groups := request.UserInfo.Groups

		// Create Subject Access Review object
		sar := &authorizationv1.SubjectAccessReview{
			Spec: authorizationv1.SubjectAccessReviewSpec{
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Verb:     "use",
					Group:    "certmanager.k8s.io",
					Resource: "clusterissuers",
				},
				User:   username,
				Groups: groups,
			},
		}

		// Authorization check
		client := authClient.SubjectAccessReviews()
		res, err := client.Create(sar)
		if err != nil {
			klog.Infof("Error occurred using subject access review client to create %v\nError: %s", sar, err.Error())
			return false
		}
		return res.Status.Allowed
	}
	return true
}
