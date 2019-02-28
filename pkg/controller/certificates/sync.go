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

package certificates

import (
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"reflect"
	"strings"
	"time"

	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog"

	apiutil "github.com/jetstack/cert-manager/pkg/api/util"
	"github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/apis/certmanager/validation"
	"github.com/jetstack/cert-manager/pkg/issuer"
	"github.com/jetstack/cert-manager/pkg/util"
	"github.com/jetstack/cert-manager/pkg/util/errors"
	"github.com/jetstack/cert-manager/pkg/util/kube"
	"github.com/jetstack/cert-manager/pkg/util/pki"
)

const (
	errorIssuerNotFound    = "IssuerNotFound"
	errorIssuerNotReady    = "IssuerNotReady"
	errorIssuerInit        = "IssuerInitError"
	errorSavingCertificate = "SaveCertError"
	errorConfig            = "ConfigError"

	reasonIssuingCertificate  = "IssueCert"
	reasonRenewingCertificate = "RenewCert"

	successCertificateIssued  = "CertIssued"
	successCertificateRenewed = "CertRenewed"

	messageErrorSavingCertificate = "Error saving TLS certificate: "

	restartLabel = "cert_manager_refresh"
)

const (
	TLSCAKey = "ca.crt"
)

var (
	certificateGvk = v1alpha1.SchemeGroupVersion.WithKind("Certificate")
)

func (c *Controller) Sync(ctx context.Context, crt *v1alpha1.Certificate) (err error) {
	crtCopy := crt.DeepCopy()
	defer func() {
		if _, saveErr := c.updateCertificateStatus(crt, crtCopy); saveErr != nil {
			err = utilerrors.NewAggregate([]error{saveErr, err})
		}
	}()

	// grab existing certificate and validate private key
	certs, key, err := kube.SecretTLSKeyPair(c.secretLister, crtCopy.Namespace, crtCopy.Spec.SecretName)
	// if we don't have a certificate, we need to trigger a re-issue immediately
	if err != nil && !(k8sErrors.IsNotFound(err) || errors.IsInvalidData(err)) {
		return err
	}

	var cert *x509.Certificate
	if len(certs) > 0 {
		cert = certs[0]
	}

	// update certificate expiry metric
	defer c.metrics.UpdateCertificateExpiry(crtCopy, c.secretLister)
	c.setCertificateStatus(crtCopy, key, cert)

	el := validation.ValidateCertificate(crtCopy)
	if len(el) > 0 {
		c.Recorder.Eventf(crtCopy, corev1.EventTypeWarning, "BadConfig", "Resource validation failed: %v", el.ToAggregate())
		return nil
	}

	// step zero: check if the referenced issuer exists and is ready
	issuerObj, err := c.helper.GetGenericIssuer(crtCopy.Spec.IssuerRef, crtCopy.Namespace)
	if k8sErrors.IsNotFound(err) {
		c.Recorder.Eventf(crtCopy, corev1.EventTypeWarning, errorIssuerNotFound, err.Error())
		return nil
	}
	if err != nil {
		return err
	}

	el = validation.ValidateCertificateForIssuer(crtCopy, issuerObj)
	if len(el) > 0 {
		c.Recorder.Eventf(crtCopy, corev1.EventTypeWarning, "BadConfig", "Resource validation failed: %v", el.ToAggregate())
		return nil
	}

	// If this is an ACME certificate, ensure the certificate.spec.acme field is
	// non-nil
	if issuerObj.GetSpec().ACME != nil && crtCopy.Spec.ACME == nil {
		c.Recorder.Eventf(crtCopy, corev1.EventTypeWarning, "BadConfig", "spec.acme field must be set")
		return nil
	}

	issuerReady := apiutil.IssuerHasCondition(issuerObj, v1alpha1.IssuerCondition{
		Type:   v1alpha1.IssuerConditionReady,
		Status: v1alpha1.ConditionTrue,
	})
	if !issuerReady {
		c.Recorder.Eventf(crtCopy, corev1.EventTypeWarning, errorIssuerNotReady, "Issuer %s not ready", issuerObj.GetObjectMeta().Name)
		return nil
	}

	i, err := c.issuerFactory.IssuerFor(issuerObj)
	if err != nil {
		c.Recorder.Eventf(crtCopy, corev1.EventTypeWarning, errorIssuerInit, "Internal error initialising issuer: %v", err)
		return nil
	}

	if key == nil || cert == nil {
		klog.V(4).Infof("Invoking issue function as existing certificate does not exist")
		return c.issue(ctx, i, crtCopy)
	}

	// begin checking if the TLS certificate is valid/needs a re-issue or renew
	matches, matchErrs := c.certificateMatchesSpec(crtCopy, key, cert)
	if !matches {
		klog.V(4).Infof("Invoking issue function due to certificate not matching spec: %s", strings.Join(matchErrs, ", "))
		return c.issue(ctx, i, crtCopy)
	}

	// check if the certificate needs renewal
	needsRenew := c.Context.IssuerOptions.CertificateNeedsRenew(cert, crt)
	if needsRenew {
		klog.V(4).Infof("Invoking issue function due to certificate needing renewal")
		return c.issue(ctx, i, crtCopy)
	}
	// end checking if the TLS certificate is valid/needs a re-issue or renew

	// If the Certificate is valid and up to date, we schedule a renewal in
	// the future.
	c.scheduleRenewal(crt)

	return nil
}

// setCertificateStatus will update the status subresource of the certificate.
// It will not actually submit the resource to the apiserver.
func (c *Controller) setCertificateStatus(crt *v1alpha1.Certificate, key crypto.Signer, cert *x509.Certificate) {
	if key == nil || cert == nil {
		apiutil.SetCertificateCondition(crt, v1alpha1.CertificateConditionReady, v1alpha1.ConditionFalse, "NotFound", "Certificate does not exist")
		return
	}

	metaNotAfter := metav1.NewTime(cert.NotAfter)
	crt.Status.NotAfter = &metaNotAfter

	// Derive & set 'Ready' condition on Certificate resource
	matches, matchErrs := c.certificateMatchesSpec(crt, key, cert)
	reason := "Ready"
	if cert.NotAfter.Before(c.clock.Now()) {
		reason = "Expired"
		matchErrs = append(matchErrs, fmt.Sprintf("Certificate has expired"))
	}
	if !matches {
		reason = "DoesNotMatch"
	}
	if len(matchErrs) > 0 {
		apiutil.SetCertificateCondition(crt, v1alpha1.CertificateConditionReady, v1alpha1.ConditionFalse, reason, strings.Join(matchErrs, ", "))
		return
	}

	apiutil.SetCertificateCondition(crt, v1alpha1.CertificateConditionReady, v1alpha1.ConditionTrue, reason, "Certificate is up to date and has not expired")

	return
}

func (c *Controller) certificateMatchesSpec(crt *v1alpha1.Certificate, key crypto.Signer, cert *x509.Certificate) (bool, []string) {
	var errs []string

	// TODO: add checks for KeySize, KeyAlgorithm fields
	// TODO: add checks for Organization field
	// TODO: add checks for IsCA field

	// check if the private key is the corresponding pair to the certificate
	matches, err := pki.PublicKeyMatchesCertificate(key.Public(), cert)
	if err != nil {
		errs = append(errs, err.Error())
	} else if !matches {
		errs = append(errs, fmt.Sprintf("Certificate private key does not match certificate"))
	}

	// validate the common name is correct
	expectedCN := pki.CommonNameForCertificate(crt)
	if expectedCN != cert.Subject.CommonName {
		errs = append(errs, fmt.Sprintf("Common name on TLS certificate not up to date: %q", cert.Subject.CommonName))
	}

	// validate the dns names are correct
	expectedDNSNames := pki.DNSNamesForCertificate(crt)
	if !util.EqualUnsorted(cert.DNSNames, expectedDNSNames) {
		errs = append(errs, fmt.Sprintf("DNS names on TLS certificate not up to date: %q", cert.DNSNames))
	}

	// validate the ip addresses are correct
	if !util.EqualUnsorted(pki.IPAddressesToString(cert.IPAddresses), crt.Spec.IPAddresses) {
		errs = append(errs, fmt.Sprintf("IP addresses on TLS certificate not up to date: %q", pki.IPAddressesToString(cert.IPAddresses)))
	}

	return len(errs) == 0, errs
}

func (c *Controller) scheduleRenewal(crt *v1alpha1.Certificate) {
	key, err := keyFunc(crt)

	if err != nil {
		runtime.HandleError(fmt.Errorf("error getting key for certificate resource: %s", err.Error()))
		return
	}

	cert, err := kube.SecretTLSCert(c.secretLister, crt.Namespace, crt.Spec.SecretName)

	if err != nil {
		if !errors.IsInvalidData(err) {
			runtime.HandleError(fmt.Errorf("[%s/%s] Error getting certificate '%s': %s", crt.Namespace, crt.Name, crt.Spec.SecretName, err.Error()))
		}
		return
	}

	renewIn := c.Context.IssuerOptions.CalculateDurationUntilRenew(cert, crt)
	c.scheduledWorkQueue.Add(key, renewIn)

	klog.Infof("Certificate %s/%s scheduled for renewal in %s", crt.Namespace, crt.Name, renewIn.String())
}

// issuerKind returns the kind of issuer for a certificate
func issuerKind(crt *v1alpha1.Certificate) string {
	if crt.Spec.IssuerRef.Kind == "" {
		return v1alpha1.IssuerKind
	}
	return crt.Spec.IssuerRef.Kind
}

func ownerRef(crt *v1alpha1.Certificate) metav1.OwnerReference {
	controller := true
	return metav1.OwnerReference{
		APIVersion: v1alpha1.SchemeGroupVersion.String(),
		Kind:       v1alpha1.CertificateKind,
		Name:       crt.Name,
		UID:        crt.UID,
		Controller: &controller,
	}
}

func (c *Controller) updateSecret(crt *v1alpha1.Certificate, namespace string, cert, key, ca []byte) (*corev1.Secret, error) {
	secret, err := c.secretLister.Secrets(namespace).Get(crt.Spec.SecretName)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, err
	}
	if k8sErrors.IsNotFound(err) {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      crt.Spec.SecretName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{},
		}
	}

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data[corev1.TLSCertKey] = cert
	secret.Data[corev1.TLSPrivateKeyKey] = key
	secret.Data[TLSCAKey] = ca

	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}

	// If we are updating the Certificate, we update the secret metadata to
	// reflect the actual certificate it contains
	if cert != nil {
		x509Cert, err := pki.DecodeX509CertificateBytes(cert)
		if err != nil {
			return nil, fmt.Errorf("invalid certificate data: %v", err)
		}

		secret.Annotations[v1alpha1.IssuerNameAnnotationKey] = crt.Spec.IssuerRef.Name
		secret.Annotations[v1alpha1.IssuerKindAnnotationKey] = issuerKind(crt)
		secret.Annotations[v1alpha1.CommonNameAnnotationKey] = x509Cert.Subject.CommonName
		secret.Annotations[v1alpha1.AltNamesAnnotationKey] = strings.Join(x509Cert.DNSNames, ",")
		secret.Annotations[v1alpha1.IPSANAnnotationKey] = strings.Join(pki.IPAddressesToString(x509Cert.IPAddresses), ",")
	}

	// Always set the certificate name label on the target secret
	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}
	secret.Labels[v1alpha1.CertificateNameKey] = crt.Name

	// if it is a new resource
	if secret.SelfLink == "" {
		enableOwner := c.CertificateOptions.EnableOwnerRef
		if enableOwner {
			secret.SetOwnerReferences(append(secret.GetOwnerReferences(), ownerRef(crt)))
		}
		secret, err = c.Client.CoreV1().Secrets(namespace).Create(secret)
	} else {
		secret, err = c.Client.CoreV1().Secrets(namespace).Update(secret)
		// Secret is updated, refresh
		klog.Info("Secret updated, refresh the pods")
		
		deploymentsInterface := c.Client.AppsV1().Deployments(namespace)
		statefulsetsInterface := c.Client.AppsV1().StatefulSets(namespace)
		daemonsetsInterface  := c.Client.AppsV1().DaemonSets(namespace)
		restart(deploymentsInterface, statefulsetsInterface, daemonsetsInterface, secret.Name)
	}

	if err != nil {
		return nil, err
	}

	return secret, nil
}

func restart(deploymentsInterface v1.DeploymentInterface, statefulsetsInterface v1.StatefulSetInterface, daemonsetsInterface v1.DaemonSetInterface, secret string) {
	listOptions := metav1.ListOptions{}
	deployments, _ := deploymentsInterface.List(listOptions)
	statefulsets, _ := statefulsetsInterface.List(listOptions)
	daemonsets, _ := daemonsetsInterface.List(listOptions)

	update := time.Now().Format("2006-1-31.0600")
NEXT_DEPLOYMENT:
	for _, deployment := range deployments.Items {
		for _, volume := range deployment.Spec.Template.Spec.Volumes {
			if volume.Secret != nil && volume.Secret.SecretName != "" && volume.Secret.SecretName == secret {
				klog.Info("!!!! DEPLOYMENT Affected !!!! ")
				klog.Info(deployment.Name)
				klog.Info("the updated time " + update)
				
				deployment.ObjectMeta.Labels[restartLabel] = update
				deployment.Spec.Template.ObjectMeta.Labels[restartLabel] = update
				update, err := deploymentsInterface.Update(deployment)
				if err != nil {
					klog.Info("Error updating deployment ")
					klog.Info(err)
				}

				continue NEXT_DEPLOYMENT
			}
		}
	}
NEXT_STATEFULSET:
	for _, statefulset := range statefulsets.Items {
		for _, volume := range statefulset.Spec.Template.Spec.Volumes {
			if volume.Secret != nil && volume.Secret.SecretName != "" && volume.Secret.SecretName == secret {
				klog.Info("!!! Stateful set affected ")
				klog.Info(statefulset.Name)
				statefulset.ObjectMeta.Labels[restartLabel] = update
				statefulset.Spec.Template.ObjectMeta.Labels[restartLabel] = update
				//statefulsetsInterface.Update(statefulset)
				continue NEXT_STATEFULSET
			}
		}
	}
NEXT_DAEMONSET:
	for _, daemonset := range daemonsets.Items {
		for _, volume := range daemonset.Spec.Template.Spec.Volumes {
			if volume.Secret != nil && volume.Secret.SecretName != "" && volume.Secret.SecretName == secret {
				daemonset.ObjectMeta.Labels[restartLabel] = update
				daemonset.Spec.Template.ObjectMeta.Labels[restartLabel] = update
				//daemonsetsInterface.Update(daemonset)
				continue NEXT_DAEMONSET
			}
		}
	}
}

// return an error on failure. If retrieval is succesful, the certificate data
// and private key will be stored in the named secret
func (c *Controller) issue(ctx context.Context, issuer issuer.Interface, crt *v1alpha1.Certificate) error {
	resp, err := issuer.Issue(ctx, crt)
	if err != nil {
		klog.Infof("Error issuing certificate for %s/%s: %v", crt.Namespace, crt.Name, err)
		return err
	}

	if resp == nil {
		return nil
	}

	if _, err := c.updateSecret(crt, crt.Namespace, resp.Certificate, resp.PrivateKey, resp.CA); err != nil {
		s := messageErrorSavingCertificate + err.Error()
		klog.Info(s)
		c.Recorder.Event(crt, corev1.EventTypeWarning, errorSavingCertificate, s)
		return err
	}

	if len(resp.Certificate) > 0 {
		c.Recorder.Event(crt, corev1.EventTypeNormal, successCertificateIssued, "Certificate issued successfully")
		// as we have just written a certificate, we should schedule it for renewal
		c.scheduleRenewal(crt)
	}

	klog.Info("Finished issuing certificate")

	// When a certificate is issued, we should find all resources that listens to its secret and refresh them

	return nil
}

func (c *Controller) updateCertificateStatus(old, new *v1alpha1.Certificate) (*v1alpha1.Certificate, error) {
	if reflect.DeepEqual(old.Status, new.Status) {
		return nil, nil
	}
	// TODO: replace Update call with UpdateStatus. This requires a custom API
	// server with the /status subresource enabled and/or subresource support
	// for CRDs (https://github.com/kubernetes/kubernetes/issues/38113)
	return c.CMClient.CertmanagerV1alpha1().Certificates(new.Namespace).Update(new)
}
