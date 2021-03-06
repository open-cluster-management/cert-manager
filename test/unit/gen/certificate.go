/*
IBM Confidential
OCO Source Materials

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

package gen

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
)

type CertificateModifier func(*v1alpha1.Certificate)

func Certificate(name string, mods ...CertificateModifier) *v1alpha1.Certificate {
	c := &v1alpha1.Certificate{
		ObjectMeta: ObjectMeta(name),
	}
	for _, mod := range mods {
		mod(c)
	}
	return c
}

func CertificateFrom(crt *v1alpha1.Certificate, mods ...CertificateModifier) *v1alpha1.Certificate {
	crt = crt.DeepCopy()
	for _, mod := range mods {
		mod(crt)
	}
	return crt
}

// SetIssuer sets the Certificate.spec.issuerRef field
func SetCertificateIssuer(o v1alpha1.ObjectReference) CertificateModifier {
	return func(c *v1alpha1.Certificate) {
		c.Spec.IssuerRef = o
	}
}

func SetCertificateDNSNames(dnsNames ...string) CertificateModifier {
	return func(crt *v1alpha1.Certificate) {
		crt.Spec.DNSNames = dnsNames
	}
}

func SetCertificateCommonName(commonName string) CertificateModifier {
	return func(crt *v1alpha1.Certificate) {
		crt.Spec.CommonName = commonName
	}
}

func SetCertificateIsCA(isCA bool) CertificateModifier {
	return func(crt *v1alpha1.Certificate) {
		crt.Spec.IsCA = isCA
	}
}

func SetCertificateKeyAlgorithm(keyAlgorithm v1alpha1.KeyAlgorithm) CertificateModifier {
	return func(crt *v1alpha1.Certificate) {
		crt.Spec.KeyAlgorithm = keyAlgorithm
	}
}

func SetCertificateKeySize(keySize int) CertificateModifier {
	return func(crt *v1alpha1.Certificate) {
		crt.Spec.KeySize = keySize
	}
}

func SetCertificateKeyEncoding(keyEncoding v1alpha1.KeyEncoding) CertificateModifier {
	return func(crt *v1alpha1.Certificate) {
		crt.Spec.KeyEncoding = keyEncoding
	}
}

func SetCertificateSecretName(secretName string) CertificateModifier {
	return func(crt *v1alpha1.Certificate) {
		crt.Spec.SecretName = secretName
	}
}

func SetCertificateDuration(duration time.Duration) CertificateModifier {
	return func(crt *v1alpha1.Certificate) {
		crt.Spec.Duration = &metav1.Duration{Duration: duration}
	}
}

func SetCertificateRenewBefore(renewBefore time.Duration) CertificateModifier {
	return func(crt *v1alpha1.Certificate) {
		crt.Spec.RenewBefore = &metav1.Duration{Duration: renewBefore}
	}
}

func SetCertificateACMEConfig(cfg v1alpha1.ACMECertificateConfig) CertificateModifier {
	return func(crt *v1alpha1.Certificate) {
		crt.Spec.ACME = &cfg
	}
}

func SetCertificateStatusCondition(c v1alpha1.CertificateCondition) CertificateModifier {
	return func(crt *v1alpha1.Certificate) {
		if len(crt.Status.Conditions) == 0 {
			crt.Status.Conditions = []v1alpha1.CertificateCondition{c}
			return
		}
		for i, existingC := range crt.Status.Conditions {
			if existingC.Type == c.Type {
				crt.Status.Conditions[i] = c
				return
			}
		}
		crt.Status.Conditions = append(crt.Status.Conditions, c)
	}
}

func SetCertificateLastFailureTime(p metav1.Time) CertificateModifier {
	return func(crt *v1alpha1.Certificate) {
		crt.Status.LastFailureTime = &p
	}
}

func SetCertificateNotAfter(p metav1.Time) CertificateModifier {
	return func(crt *v1alpha1.Certificate) {
		crt.Status.NotAfter = &p
	}
}

func SetCertificateOrganization(orgs ...string) CertificateModifier {
	return func(ch *v1alpha1.Certificate) {
		ch.Spec.Organization = orgs
	}
}

// SetLabels - ICP, the labels required for restarting pods
func SetLabels() CertificateModifier {
	return func(crt *v1alpha1.Certificate) {
		issuerNameLabel := "certmanager.k8s.io/issuer-name"
		issuerKindLabel := "certmanager.k8s.io/issuer-kind"
		if crt.ObjectMeta.Labels == nil {
			crt.ObjectMeta.Labels = make(map[string]string)
		}
		crt.ObjectMeta.Labels[issuerNameLabel] = crt.Spec.IssuerRef.Name
		crt.ObjectMeta.Labels[issuerKindLabel] = crt.Spec.IssuerRef.Kind
	}
}
func SetCertificateNamespace(namespace string) CertificateModifier {
	return func(crt *v1alpha1.Certificate) {
		crt.ObjectMeta.Namespace = namespace
	}
}

func SetCertificateKeyUsages(usages ...v1alpha1.KeyUsage) CertificateModifier {
	return func(cr *v1alpha1.Certificate) {
		cr.Spec.Usages = usages
	}
}
