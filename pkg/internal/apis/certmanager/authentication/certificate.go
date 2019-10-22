package authentication

import (
	"fmt"

	"github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	authorizationv1 "k8s.io/api/authorization/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	authclientv1beta1 "k8s.io/client-go/kubernetes/typed/authorization/v1beta1"
	restclient "k8s.io/client-go/rest"

	"k8s.io/klog"
)

type CertificateAuthenticator struct {
	authClient *authclientv1beta1.AuthorizationV1beta1Client
}

func NewCertificateAuthenticator() *CertificateAuthenticator {
	return &CertificateAuthenticator{}
}

// Initialize will get the config for this certificate authenticator.
func (c *CertificateAuthenticator) Initialize(kubeClientConfig *restclient.Config, stopCh <-chan struct{}) error {
	c.authClient, _ = authclientv1beta1.NewForConfig(kubeClientConfig)
	return nil
}

// Authenticate will check a certificate creation request to see if the user is allowed to create this certificate with the issuer
// they specify.
func (c *CertificateAuthenticator) Authenticate(request *admissionv1beta1.AdmissionRequest, obj runtime.Object) (bool, string) {
	crt := obj.(*v1alpha1.Certificate)
	issuerKind := crt.Spec.IssuerRef.Kind
	// Check authorization if the Certificate is to be issued/signed by a ClusterIssuer
	if issuerKind == "ClusterIssuer" {
		username := request.UserInfo.Username
		groups := request.UserInfo.Groups
		allowed := c.allowed(username, groups)
		klog.V(4).Infof("%s/%v is allowed to use ClusterIssuer %s? %t", username, groups, crt.Spec.IssuerRef.Name, allowed)
		if !allowed {
			message := fmt.Sprintf("User: %s is not allowed to use the ClusterIssuer %s to sign the Certificate %s.", request.UserInfo.Username, crt.Spec.IssuerRef.Name, crt.ObjectMeta.Name)
			return allowed, message
		}
		return allowed, ""
	}
	return true, ""
}

// Checks if the user requesting to create this Certificate is allowed to
// Returns true if they are, returns false otherwise
func (c *CertificateAuthenticator) allowed(username string, groups []string) bool {
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
	client := c.authClient.SubjectAccessReviews()
	res, err := client.Create(sar)
	klog.V(3).Infof("The resulting subject access review: %v", res)
	if err != nil {
		klog.Errorf("Error occurred using subject access review client to create %v\nError: %s", sar, err.Error())
		return false
	}
	return res.Status.Allowed
}
