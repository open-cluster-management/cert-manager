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
	"bytes"
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
type DefaultAdmin string

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

	klog.Infof("%s", obj.Spec.IssuerRef.Kind)
	klog.Infof("------------- USER INFO FOR %s --------------", obj.ObjectMeta.Name)
	findUser(admissionSpec)
	authorized := allowed(admissionSpec, obj)
	if !authorized {
		klog.Info("UNAUTHORIZED")
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

func findUser(admissionSpec *admissionv1beta1.AdmissionRequest) {
	klog.Infof("USERINFO: %v", admissionSpec.UserInfo)
	klog.Infof("USERNAME: %s", admissionSpec.UserInfo.Username)
	klog.Infof("UID: %s", admissionSpec.UserInfo.UID)
	klog.Infof("GROUPS: %v", admissionSpec.UserInfo.Groups)
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
			// Make api call to iam to check user id
			if uid.Fragment == DefaultAdmin {
				return true
			}
/* 			accessToken, err := getAccessToken()
			if err != nil {
				klog.Infof("Error occurred getting the access token: %s", err.Error())
				return false
			}
			highestRole, err := getHighestRole(accessToken, uid.Fragment)
			if err != nil {
				klog.Infof("Error occurred getting the highest role for user: %s", err.Error())
				return false
			}
			klog.Infof("Highest role: %s", highestRole)
			if highestRole == "ClusterAdmin" {
				return true
			} */
		}
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

func getAccessToken() (string, error) {
	file := "/etc/cfc/api-key"
	apiKeyFile, err := ioutil.ReadFile(file)
	if err != nil {
		klog.Infof("Error occurred reading api key file: %s", err.Error())
		return "", err
	}
	apiKey := string(apiKeyFile)
	// Use api key to get access token
	management_url := "https://9.46.73.170:8443"
	accessTokenApi := "iam-token/oidc/token"
	data := url.Values{}
	data.Set("grant_type", "urn:ibm:params:oauth:grant-type:apikey")
	data.Add("apikey", apiKey)
	data.Add("response_type", "cloud_iam")
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second}
	reqBody := []byte(data.Encode())
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/%s", management_url, accessTokenApi), ioutil.NopCloser(bytes.NewReader(reqBody)))
	if err != nil {
		klog.Infof("Error occurred creating a new request: %s", err.Error())
		return "", err
	}
	request.Header.Add("Accept", "application/json")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(request)
	if err != nil {
		klog.Infof("Error occurred sending request: %s", err.Error())
		return "", err
	}
	if res.StatusCode > 299 {
		return "", fmt.Errorf("%s\n%s", "StatusCode from request is not 200, ", res.Status)
	}
	defer res.Body.Close()
	out, err := ioutil.ReadAll(res.Body)
	if err != nil {
		klog.Infof("Error retrieving OIDC Key: %v", err)
		return "", err
	}
	var tokenResponse OIDCTokenResponse
	json.Unmarshal(out, &tokenResponse)
	return tokenResponse.AccessToken, nil
}

func getHighestRole(token string, uid string) (string, error) {
	management_url := "https://9.46.73.170:8443"
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   60 * time.Second}
	api := fmt.Sprintf("/idmgmt/identity/api/v1/teams/roleMappings?userid=%s", uid)
	request, err := http.NewRequest("GET", fmt.Sprintf("%s%s", management_url, api), nil)
	if err != nil {
		klog.Infof("Error creating request for highest role: %s", err.Error())
		return "", err
	}
	request.Header.Add("Accept", "application/json")
	request.Header.Add("Authorization", "Bearer " + token)
	klog.Infof("URL: %s", fmt.Sprintf("%s%s", management_url, api))
	klog.Infof("auth bearer: %s", token)
	res, err := client.Do(request)
	if err != nil { 
		klog.Infof("Error occurred sending request to get highest role: %s", err.Error())
		return "", err
	}
	out, err := ioutil.ReadAll(res.Body)
	if err != nil {
		klog.Infof("Error retrieving highest role: %s", err.Error())
		return "", err
	}
	var highestRole string
	json.Unmarshal(out, &highestRole)
	return highestRole, nil
}