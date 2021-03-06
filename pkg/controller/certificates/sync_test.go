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

package certificates

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	coretesting "k8s.io/client-go/testing"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	clock "k8s.io/utils/clock/testing"

	cmapi "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	testpkg "github.com/jetstack/cert-manager/pkg/controller/test"
	"github.com/jetstack/cert-manager/pkg/feature"
	"github.com/jetstack/cert-manager/pkg/issuer"
	"github.com/jetstack/cert-manager/pkg/issuer/fake"
	_ "github.com/jetstack/cert-manager/pkg/issuer/selfsigned"
	utilfeature "github.com/jetstack/cert-manager/pkg/util/feature"
	"github.com/jetstack/cert-manager/pkg/util/pki"
	"github.com/jetstack/cert-manager/test/unit/gen"
)

func generatePrivateKey(t *testing.T) *rsa.PrivateKey {
	pk, err := pki.GenerateRSAPrivateKey(2048)
	if err != nil {
		t.Errorf("failed to generate private key: %v", err)
		t.FailNow()
	}
	return pk
}

var serialNumberLimit = new(big.Int).Lsh(big.NewInt(1), 128)

func generateSelfSignedCert(t *testing.T, crt *cmapi.Certificate, sn *big.Int, key crypto.Signer, notBefore, notAfter time.Time) []byte {
	commonName := pki.CommonNameForCertificate(crt)
	dnsNames := pki.DNSNamesForCertificate(crt)

	if sn == nil {
		var err error
		sn, err = rand.Int(rand.Reader, serialNumberLimit)
		if err != nil {
			t.Errorf("failed to generate serial number: %v", err)
			t.FailNow()
		}
	}

	template := &x509.Certificate{
		Version:               3,
		BasicConstraintsValid: true,
		SerialNumber:          sn,
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,
		// see http://golang.org/pkg/crypto/x509/#KeyUsage
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		DNSNames: dnsNames,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		t.Errorf("error signing cert: %v", err)
		t.FailNow()
	}

	pemByteBuffer := bytes.NewBuffer([]byte{})
	err = pem.Encode(pemByteBuffer, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		t.Errorf("failed to encode cert: %v", err)
		t.FailNow()
	}

	return pemByteBuffer.Bytes()
}

func TestSync(t *testing.T) {
	nowTime := time.Now()
	nowMetaTime := metav1.NewTime(nowTime)
	fixedClock := clock.NewFakeClock(nowTime)

	testIssuer := gen.Issuer("test",
		gen.SetIssuerSelfSigned(cmapi.SelfSignedIssuer{}),
	)
	testIssuerReady := gen.IssuerFrom(testIssuer,
		gen.AddIssuerCondition(cmapi.IssuerCondition{
			Type:   cmapi.IssuerConditionReady,
			Status: cmapi.ConditionTrue,
		}),
	)

	exampleCert := gen.Certificate("test",
		gen.SetCertificateDNSNames("example.com"),
		gen.SetCertificateIssuer(cmapi.ObjectReference{Name: "test"}),
		gen.SetCertificateSecretName("output"),
		gen.SetLabels(),
	)
	exampleCertNotFoundCondition := gen.CertificateFrom(exampleCert,
		gen.SetCertificateStatusCondition(cmapi.CertificateCondition{
			Type:               cmapi.CertificateConditionReady,
			Status:             cmapi.ConditionFalse,
			Reason:             "NotFound",
			Message:            "Certificate does not exist",
			LastTransitionTime: &nowMetaTime,
		}),
	)
	exampleCertTemporaryCondition := gen.CertificateFrom(exampleCert,
		gen.SetCertificateStatusCondition(cmapi.CertificateCondition{
			Type:               cmapi.CertificateConditionReady,
			Status:             cmapi.ConditionFalse,
			Reason:             "TemporaryCertificate",
			Message:            "Certificate issuance in progress. Temporary certificate issued.",
			LastTransitionTime: &nowMetaTime,
		}),
	)

	pk1 := generatePrivateKey(t)
	pk1PEM := pki.EncodePKCS1PrivateKey(pk1)
	cert1PEM := generateSelfSignedCert(t, exampleCert, nil, pk1, nowTime, nowTime.Add(time.Hour*12))
	cert1, err := pki.DecodeX509CertificateBytes(cert1PEM)
	if err != nil {
		t.Errorf("Error decoding test cert1 bytes: %v", err)
		t.FailNow()
	}

	pk2 := generatePrivateKey(t)
	// pk2PEM := pki.EncodePKCS1PrivateKey(pk2)
	cert2PEM := generateSelfSignedCert(t, exampleCert, nil, pk2, nowTime, nowTime.Add(time.Hour*24))
	cert2, err := pki.DecodeX509CertificateBytes(cert2PEM)
	if err != nil {
		t.Errorf("Error decoding test cert2 bytes: %v", err)
		t.FailNow()
	}
	updatedCertStatus := gen.CertificateFrom(exampleCert,
		gen.SetCertificateStatusCondition(cmapi.CertificateCondition{
			Type:               cmapi.CertificateConditionReady,
			Status:             cmapi.ConditionTrue,
			Reason:             "Ready",
			Message:            "Certificate is up to date and has not expired",
			LastTransitionTime: &nowMetaTime,
		}),
		gen.SetCertificateNotAfter(metav1.NewTime(cert1.NotAfter)),
	)

	localTempCert := generateSelfSignedCert(t, exampleCert, big.NewInt(staticTemporarySerialNumber), pk1, nowTime, nowTime)

	exampleCertWrongGroup := exampleCert.DeepCopy()
	exampleCertWrongGroup.Spec.IssuerRef.Group = "wrong.group.io"

	tests := map[string]testTDefault{
		"should update certificate with NotExists if issuer does not return a keypair": {
			certificate: exampleCert,
			issuerImpl: &fake.Issuer{
				IssueFunc: func(context.Context, *cmapi.Certificate) (*issuer.IssueResponse, error) {
					// By not returning a response, we trigger a 'no-op' action
					// which causes the certificate controller to only update
					// the status of the Certificate and not create a Secret.
					return nil, nil
				},
			},
			builder: &testpkg.Builder{
				CertManagerObjects: []runtime.Object{
					testIssuerReady,
					exampleCert,
				},
				ExpectedActions: []testpkg.Action{
					testpkg.NewAction(coretesting.NewUpdateAction(
						cmapi.SchemeGroupVersion.WithResource("certificates"),
						gen.DefaultTestNamespace,
						exampleCertNotFoundCondition,
					)),
				},
			},
		},
		"should create a secret containing the private key only when one doesn't exist": {
			certificate: exampleCert,
			issuerImpl: &fake.Issuer{
				IssueFunc: func(context.Context, *cmapi.Certificate) (*issuer.IssueResponse, error) {
					return &issuer.IssueResponse{
						PrivateKey: pk1PEM,
					}, nil
				},
			},
			staticTemporaryCert: localTempCert,
			builder: &testpkg.Builder{
				CertManagerObjects: []runtime.Object{
					exampleCert,
					testIssuerReady,
				},
				ExpectedActions: []testpkg.Action{
					testpkg.NewAction(coretesting.NewUpdateAction(
						cmapi.SchemeGroupVersion.WithResource("certificates"),
						gen.DefaultTestNamespace,
						exampleCertNotFoundCondition,
					)),
					testpkg.NewAction(coretesting.NewCreateAction(
						corev1.SchemeGroupVersion.WithResource("secrets"),
						gen.DefaultTestNamespace,
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: gen.DefaultTestNamespace,
								Name:      "output",
								Labels: map[string]string{
									cmapi.CertificateNameKey: "test",
								},
								Annotations: map[string]string{
									cmapi.CertificateNameKey:         "test",
									"certmanager.k8s.io/alt-names":   "example.com",
									"certmanager.k8s.io/common-name": "example.com",
									"certmanager.k8s.io/ip-sans":     "",
									"certmanager.k8s.io/issuer-kind": "Issuer",
									"certmanager.k8s.io/issuer-name": "test",
								},
							},
							Type: corev1.SecretTypeTLS,
							Data: map[string][]byte{
								corev1.TLSCertKey:       localTempCert,
								corev1.TLSPrivateKeyKey: pk1PEM,
								cmapi.TLSCAKey:          nil,
							},
						},
					)),
				},
				ExpectedEvents: []string{`Normal GenerateSelfSigned Generated temporary self signed certificate`},
			},
		},
		"should update an existing empty secret with the private key": {
			certificate: exampleCert,
			issuerImpl: &fake.Issuer{
				IssueFunc: func(context.Context, *cmapi.Certificate) (*issuer.IssueResponse, error) {
					return &issuer.IssueResponse{
						PrivateKey: pk1PEM,
					}, nil
				},
			},
			staticTemporaryCert: localTempCert,
			builder: &testpkg.Builder{
				KubeObjects: []runtime.Object{
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: gen.DefaultTestNamespace,
							Name:      "output",
							SelfLink:  "abc",
							Labels: map[string]string{
								cmapi.CertificateNameKey: "nottest",
							},
							Annotations: map[string]string{
								"testannotation": "true",
							},
						},
					},
				},
				CertManagerObjects: []runtime.Object{
					testIssuerReady,
					exampleCert,
				},
				ExpectedActions: []testpkg.Action{
					testpkg.NewAction(coretesting.NewUpdateAction(
						cmapi.SchemeGroupVersion.WithResource("certificates"),
						gen.DefaultTestNamespace,
						exampleCertNotFoundCondition,
					)),
					testpkg.NewAction(coretesting.NewUpdateAction(
						corev1.SchemeGroupVersion.WithResource("secrets"),
						gen.DefaultTestNamespace,
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: gen.DefaultTestNamespace,
								Name:      "output",
								SelfLink:  "abc",
								Labels: map[string]string{
									cmapi.CertificateNameKey: "test",
								},
								Annotations: map[string]string{
									"testannotation":                 "true",
									cmapi.CertificateNameKey:         "test",
									"certmanager.k8s.io/alt-names":   "example.com",
									"certmanager.k8s.io/common-name": "example.com",
									"certmanager.k8s.io/ip-sans":     "",
									"certmanager.k8s.io/issuer-kind": "Issuer",
									"certmanager.k8s.io/issuer-name": "test",
								},
							},
							Data: map[string][]byte{
								corev1.TLSCertKey:       localTempCert,
								corev1.TLSPrivateKeyKey: pk1PEM,
								cmapi.TLSCAKey:          nil,
							},
						},
					)),
				},
				ExpectedEvents: []string{`Normal GenerateSelfSigned Generated temporary self signed certificate`},
			},
		},
		"should create a new secret containing private key and cert": {
			certificate: exampleCert,
			issuerImpl: &fake.Issuer{
				IssueFunc: func(context.Context, *cmapi.Certificate) (*issuer.IssueResponse, error) {
					return &issuer.IssueResponse{
						PrivateKey:  pk1PEM,
						Certificate: cert1PEM,
					}, nil
				},
			},
			builder: &testpkg.Builder{
				CertManagerObjects: []runtime.Object{
					testIssuerReady,
					exampleCert,
				},
				ExpectedActions: []testpkg.Action{
					testpkg.NewAction(coretesting.NewUpdateAction(
						cmapi.SchemeGroupVersion.WithResource("certificates"),
						gen.DefaultTestNamespace,
						exampleCertNotFoundCondition,
					)),
					testpkg.NewAction(coretesting.NewCreateAction(
						corev1.SchemeGroupVersion.WithResource("secrets"),
						gen.DefaultTestNamespace,
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: gen.DefaultTestNamespace,
								Name:      "output",
								Labels: map[string]string{
									cmapi.CertificateNameKey: "test",
								},
								Annotations: map[string]string{
									cmapi.CertificateNameKey:         "test",
									"certmanager.k8s.io/alt-names":   "example.com",
									"certmanager.k8s.io/common-name": "example.com",
									"certmanager.k8s.io/ip-sans":     "",
									"certmanager.k8s.io/issuer-kind": "Issuer",
									"certmanager.k8s.io/issuer-name": "test",
								},
							},
							Data: map[string][]byte{
								corev1.TLSCertKey:       cert1PEM,
								corev1.TLSPrivateKeyKey: pk1PEM,
								cmapi.TLSCAKey:          nil,
							},
							Type: corev1.SecretTypeTLS,
						},
					)),
				},
				ExpectedEvents: []string{`Normal CertIssued Certificate issued successfully`},
			},
		},
		"should update an existing secret with private key and cert": {
			certificate: exampleCert,
			issuerImpl: &fake.Issuer{
				IssueFunc: func(context.Context, *cmapi.Certificate) (*issuer.IssueResponse, error) {
					return &issuer.IssueResponse{
						PrivateKey:  pk1PEM,
						Certificate: cert1PEM,
					}, nil
				},
			},
			builder: &testpkg.Builder{
				KubeObjects: []runtime.Object{
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: gen.DefaultTestNamespace,
							Name:      "output",
							SelfLink:  "abc",
							Labels: map[string]string{
								cmapi.CertificateNameKey: "nottest",
							},
							Annotations: map[string]string{
								"testannotation": "true",
							},
						},
					},
				},
				CertManagerObjects: []runtime.Object{
					testIssuerReady,
					exampleCert,
				},
				ExpectedActions: []testpkg.Action{
					testpkg.NewAction(coretesting.NewUpdateAction(
						cmapi.SchemeGroupVersion.WithResource("certificates"),
						gen.DefaultTestNamespace,
						exampleCertNotFoundCondition,
					)),
					testpkg.NewAction(coretesting.NewUpdateAction(
						corev1.SchemeGroupVersion.WithResource("secrets"),
						gen.DefaultTestNamespace,
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: gen.DefaultTestNamespace,
								Name:      "output",
								SelfLink:  "abc",
								Labels: map[string]string{
									cmapi.CertificateNameKey: "test",
								},
								Annotations: map[string]string{
									"testannotation":                 "true",
									cmapi.CertificateNameKey:         "test",
									"certmanager.k8s.io/alt-names":   "example.com",
									"certmanager.k8s.io/common-name": "example.com",
									"certmanager.k8s.io/ip-sans":     "",
									"certmanager.k8s.io/issuer-kind": "Issuer",
									"certmanager.k8s.io/issuer-name": "test",
								},
							},
							Data: map[string][]byte{
								corev1.TLSCertKey:       cert1PEM,
								corev1.TLSPrivateKeyKey: pk1PEM,
								cmapi.TLSCAKey:          nil,
							},
						},
					)),
				},
				ExpectedEvents: []string{`Normal CertIssued Certificate issued successfully`},
			},
		},
		"should mark certificate with invalid private key as DoesNotMatch": {
			certificate: exampleCert,
			issuerImpl: &fake.Issuer{
				IssueFunc: func(context.Context, *cmapi.Certificate) (*issuer.IssueResponse, error) {
					return &issuer.IssueResponse{
						PrivateKey:  pk1PEM,
						Certificate: cert1PEM,
					}, nil
				},
			},
			builder: &testpkg.Builder{
				KubeObjects: []runtime.Object{
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: gen.DefaultTestNamespace,
							Name:      "output",
							SelfLink:  "abc",
							Labels: map[string]string{
								cmapi.CertificateNameKey: "nottest",
							},
							Annotations: map[string]string{
								"testannotation": "true",
								// We want ONLY invalid key, issuer annotations should be correct
								"certmanager.k8s.io/issuer-kind": "Issuer",
								"certmanager.k8s.io/issuer-name": "test",
							},
						},
						Data: map[string][]byte{
							corev1.TLSCertKey:       cert2PEM,
							corev1.TLSPrivateKeyKey: pk1PEM,
							cmapi.TLSCAKey:          nil,
						},
					},
				},
				CertManagerObjects: []runtime.Object{
					testIssuerReady,
					gen.Certificate("test"),
				},
				ExpectedActions: []testpkg.Action{
					testpkg.NewAction(coretesting.NewUpdateAction(
						cmapi.SchemeGroupVersion.WithResource("certificates"),
						gen.DefaultTestNamespace,
						gen.CertificateFrom(exampleCert,
							gen.SetCertificateStatusCondition(cmapi.CertificateCondition{
								Type:               cmapi.CertificateConditionReady,
								Status:             cmapi.ConditionFalse,
								Reason:             "DoesNotMatch",
								Message:            "Certificate private key does not match certificate",
								LastTransitionTime: &nowMetaTime,
							}),
							gen.SetCertificateNotAfter(metav1.NewTime(cert2.NotAfter)),
						),
					)),
					testpkg.NewAction(coretesting.NewUpdateAction(
						corev1.SchemeGroupVersion.WithResource("secrets"),
						gen.DefaultTestNamespace,
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: gen.DefaultTestNamespace,
								Name:      "output",
								SelfLink:  "abc",
								Labels: map[string]string{
									cmapi.CertificateNameKey: "test",
								},
								Annotations: map[string]string{
									"testannotation":                 "true",
									cmapi.CertificateNameKey:         "test",
									"certmanager.k8s.io/alt-names":   "example.com",
									"certmanager.k8s.io/common-name": "example.com",
									"certmanager.k8s.io/ip-sans":     "",
									"certmanager.k8s.io/issuer-kind": "Issuer",
									"certmanager.k8s.io/issuer-name": "test",
								},
							},
							Data: map[string][]byte{
								corev1.TLSCertKey:       cert1PEM,
								corev1.TLSPrivateKeyKey: pk1PEM,
								cmapi.TLSCAKey:          nil,
							},
						},
					)),
				},
				ExpectedEvents: []string{`Normal CertIssued Certificate issued successfully`},
			},
		},
		"should update status of up to date certificate": {
			certificate: exampleCert,
			issuerImpl: &fake.Issuer{
				IssueFunc: func(context.Context, *cmapi.Certificate) (*issuer.IssueResponse, error) {
					return &issuer.IssueResponse{
						PrivateKey:  pk1PEM,
						Certificate: cert1PEM,
					}, nil
				},
			},
			builder: &testpkg.Builder{
				KubeObjects: []runtime.Object{
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: gen.DefaultTestNamespace,
							Name:      "output",
							SelfLink:  "abc",
							Labels: map[string]string{
								cmapi.CertificateNameKey: "test",
							},
							Annotations: map[string]string{
								"testannotation":                 "true",
								"certmanager.k8s.io/alt-names":   "example.com",
								"certmanager.k8s.io/common-name": "example.com",
								"certmanager.k8s.io/ip-sans":     "",
								"certmanager.k8s.io/issuer-kind": "Issuer",
								"certmanager.k8s.io/issuer-name": "test",
							},
						},
						Data: map[string][]byte{
							corev1.TLSCertKey:       cert1PEM,
							corev1.TLSPrivateKeyKey: pk1PEM,
							cmapi.TLSCAKey:          nil,
						},
					},
				},
				CertManagerObjects: []runtime.Object{
					testIssuerReady,
					gen.Certificate("test"),
				},
				ExpectedActions: []testpkg.Action{
					testpkg.NewAction(coretesting.NewUpdateAction(
						cmapi.SchemeGroupVersion.WithResource("certificates"),
						gen.DefaultTestNamespace,
						updatedCertStatus,
					)),
				},
			},
		},
		"should update the reason field with temporary self signed cert text": {
			certificate: exampleCert,
			issuerImpl: &fake.Issuer{
				IssueFunc: func(context.Context, *cmapi.Certificate) (*issuer.IssueResponse, error) {
					return &issuer.IssueResponse{
						PrivateKey: pk1PEM,
					}, nil
				},
			},
			// set this to something other than localTempCert, so that we can
			// assert that the controller doesn't enter in a loop updating the
			// Secret resource with a newly generated certificate
			staticTemporaryCert: cert1PEM,
			builder: &testpkg.Builder{
				KubeObjects: []runtime.Object{
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: gen.DefaultTestNamespace,
							Name:      "output",
							SelfLink:  "abc",
							Labels: map[string]string{
								cmapi.CertificateNameKey: "nottest",
							},
							Annotations: map[string]string{
								"testannotation": "true",
							},
						},
						Data: map[string][]byte{
							corev1.TLSCertKey:       localTempCert,
							corev1.TLSPrivateKeyKey: pk1PEM,
							cmapi.TLSCAKey:          nil,
						},
					},
				},
				CertManagerObjects: []runtime.Object{
					testIssuerReady,
					gen.Certificate("test"),
				},
				ExpectedActions: []testpkg.Action{
					testpkg.NewAction(coretesting.NewUpdateAction(
						corev1.SchemeGroupVersion.WithResource("secrets"),
						gen.DefaultTestNamespace,
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: gen.DefaultTestNamespace,
								Name:      "output",
								SelfLink:  "abc",
								Labels: map[string]string{
									cmapi.CertificateNameKey: "test",
								},
								Annotations: map[string]string{
									"testannotation":                 "true",
									cmapi.CertificateNameKey:         "test",
									"certmanager.k8s.io/alt-names":   "example.com",
									"certmanager.k8s.io/common-name": "example.com",
									"certmanager.k8s.io/ip-sans":     "",
									"certmanager.k8s.io/issuer-kind": "Issuer",
									"certmanager.k8s.io/issuer-name": "test",
								},
							},
							Data: map[string][]byte{
								corev1.TLSCertKey:       localTempCert,
								corev1.TLSPrivateKeyKey: pk1PEM,
								cmapi.TLSCAKey:          nil,
							},
						},
					)),
					testpkg.NewAction(coretesting.NewUpdateAction(
						cmapi.SchemeGroupVersion.WithResource("certificates"),
						gen.DefaultTestNamespace,
						exampleCertTemporaryCondition,
					)),
				},
			},
		},
		"should mark certificate with wrong issuer name as DoesNotMatch": {
			certificate: exampleCert,
			issuerImpl: &fake.Issuer{
				IssueFunc: func(context.Context, *cmapi.Certificate) (*issuer.IssueResponse, error) {
					return &issuer.IssueResponse{
						PrivateKey:  pk1PEM,
						Certificate: cert1PEM,
					}, nil
				},
			},
			builder: &testpkg.Builder{
				KubeObjects: []runtime.Object{
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: gen.DefaultTestNamespace,
							Name:      "output",
							SelfLink:  "abc",
							Labels: map[string]string{
								cmapi.CertificateNameKey: "test",
							},
							Annotations: map[string]string{
								"testannotation":                 "true",
								"certmanager.k8s.io/issuer-kind": "Issuer",
								"certmanager.k8s.io/issuer-name": "not-test",
							},
						},
						Data: map[string][]byte{
							corev1.TLSCertKey:       cert1PEM,
							corev1.TLSPrivateKeyKey: pk1PEM,
							cmapi.TLSCAKey:          nil,
						},
					},
				},
				CertManagerObjects: []runtime.Object{
					testIssuerReady,
					exampleCert,
				},
				ExpectedActions: []testpkg.Action{
					testpkg.NewAction(coretesting.NewUpdateAction(
						cmapi.SchemeGroupVersion.WithResource("certificates"),
						gen.DefaultTestNamespace,
						gen.CertificateFrom(exampleCert,
							gen.SetCertificateStatusCondition(cmapi.CertificateCondition{
								Type:               cmapi.CertificateConditionReady,
								Status:             cmapi.ConditionFalse,
								Reason:             "DoesNotMatch",
								Message:            "Issuer of the certificate is not up to date: \"not-test\"",
								LastTransitionTime: &nowMetaTime,
							}),
							gen.SetCertificateNotAfter(metav1.NewTime(cert1.NotAfter)),
						),
					)),
					testpkg.NewAction(coretesting.NewUpdateAction(
						corev1.SchemeGroupVersion.WithResource("secrets"),
						gen.DefaultTestNamespace,
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: gen.DefaultTestNamespace,
								Name:      "output",
								SelfLink:  "abc",
								Labels: map[string]string{
									cmapi.CertificateNameKey: "test",
								},
								Annotations: map[string]string{
									"testannotation":                 "true",
									cmapi.CertificateNameKey:         "test",
									"certmanager.k8s.io/alt-names":   "example.com",
									"certmanager.k8s.io/common-name": "example.com",
									"certmanager.k8s.io/ip-sans":     "",
									"certmanager.k8s.io/issuer-kind": "Issuer",
									"certmanager.k8s.io/issuer-name": "test",
								},
							},
							Data: map[string][]byte{
								corev1.TLSCertKey:       cert1PEM,
								corev1.TLSPrivateKeyKey: pk1PEM,
								cmapi.TLSCAKey:          nil,
							},
						},
					)),
				},
				ExpectedEvents: []string{`Normal CertIssued Certificate issued successfully`},
			},
		},
		"should mark certificate with duplicate secretName as DuplicateSecretName": {
			certificate: exampleCert,
			issuerImpl: &fake.Issuer{
				IssueFunc: func(context.Context, *cmapi.Certificate) (*issuer.IssueResponse, error) {
					return &issuer.IssueResponse{
						PrivateKey:  pk1PEM,
						Certificate: cert1PEM,
					}, nil
				},
			},
			builder: &testpkg.Builder{
				CertManagerObjects: []runtime.Object{
					testIssuerReady,
					exampleCert,
					gen.Certificate("dup-test",
						gen.SetCertificateDNSNames("example.com"),
						gen.SetCertificateIssuer(cmapi.ObjectReference{Name: "test"}),
						gen.SetCertificateSecretName("output"),
					),
				},
				ExpectedActions: []testpkg.Action{
					testpkg.NewAction(coretesting.NewUpdateAction(
						cmapi.SchemeGroupVersion.WithResource("certificates"),
						gen.DefaultTestNamespace,
						gen.CertificateFrom(exampleCert,
							gen.SetCertificateStatusCondition(cmapi.CertificateCondition{
								Type:               cmapi.CertificateConditionReady,
								Status:             cmapi.ConditionFalse,
								Reason:             "DuplicateSecretName",
								Message:            "Another Certificate is using the same secretName",
								LastTransitionTime: &nowMetaTime,
							}),
						),
					)),
				},
				ExpectedEvents: []string{`Warning DuplicateSecretNameError Another Certificate dup-test already specifies spec.secretName output, please update the secretName on either Certificate`},
			},
		},
		"should allow duplicate secretName in different namespaces": {
			certificate: exampleCert,
			issuerImpl: &fake.Issuer{
				IssueFunc: func(context.Context, *cmapi.Certificate) (*issuer.IssueResponse, error) {
					return &issuer.IssueResponse{
						PrivateKey:  pk1PEM,
						Certificate: cert1PEM,
					}, nil
				},
			},
			builder: &testpkg.Builder{
				CertManagerObjects: []runtime.Object{
					testIssuerReady,
					exampleCert,
					gen.CertificateFrom(exampleCert,
						gen.SetCertificateNamespace("other-unit-test-ns")),
				},
				ExpectedActions: []testpkg.Action{
					// specifically tests that a secret is created - behaves as usual
					testpkg.NewAction(coretesting.NewUpdateAction(
						cmapi.SchemeGroupVersion.WithResource("certificates"),
						gen.DefaultTestNamespace,
						exampleCertNotFoundCondition,
					)),
					testpkg.NewAction(coretesting.NewCreateAction(
						corev1.SchemeGroupVersion.WithResource("secrets"),
						gen.DefaultTestNamespace,
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: gen.DefaultTestNamespace,
								Name:      "output",
								Labels: map[string]string{
									cmapi.CertificateNameKey: "test",
								},
								Annotations: map[string]string{
									cmapi.CertificateNameKey:         "test",
									"certmanager.k8s.io/alt-names":   "example.com",
									"certmanager.k8s.io/common-name": "example.com",
									"certmanager.k8s.io/ip-sans":     "",
									"certmanager.k8s.io/issuer-kind": "Issuer",
									"certmanager.k8s.io/issuer-name": "test",
								},
							},
							Data: map[string][]byte{
								corev1.TLSCertKey:       cert1PEM,
								corev1.TLSPrivateKeyKey: pk1PEM,
								cmapi.TLSCAKey:          nil,
							},
							Type: corev1.SecretTypeTLS,
						},
					)),
				},
				ExpectedEvents: []string{`Normal CertIssued Certificate issued successfully`},
			},
		},
		"should exit sync nil if group is not certmanager.k8s.io": {
			certificate: exampleCertWrongGroup,
			issuerImpl: &fake.Issuer{
				IssueFunc: func(context.Context, *cmapi.Certificate) (*issuer.IssueResponse, error) {
					return nil, errors.New("unexpected issue call")
				},
			},
			builder: &testpkg.Builder{
				CertManagerObjects: []runtime.Object{
					testIssuerReady,
					exampleCert,
				},
				ExpectedActions: []testpkg.Action{},
			},
		},
		//"should add annotations to already existing secret resource": {
		//	Issuer: gen.Issuer("test",
		//		gen.AddIssuerCondition(cmapi.IssuerCondition{
		//			Type:   cmapi.IssuerConditionReady,
		//			Status: cmapi.ConditionTrue,
		//		}),
		//		gen.SetIssuerSelfSigned(cmapi.SelfSignedIssuer{}),
		//	),
		//	Certificate: *gen.CertificateFrom(exampleCert,
		//		gen.SetCertificateStatusCondition(cmapi.CertificateCondition{
		//			Type:               cmapi.CertificateConditionReady,
		//			Status:             cmapi.ConditionTrue,
		//			Reason:             "Ready",
		//			Message:            "Certificate is up to date and has not expired",
		//			LastTransitionTime: nowMetaTime,
		//		}),
		//		gen.SetCertificateNotAfter(metav1.NewTime(cert1.NotAfter)),
		//	),
		//	IssuerImpl: &fake.Issuer{
		//		IssueFunc: func(context.Context, *cmapi.Certificate) (*issuer.IssueResponse, error) {
		//			return &issuer.IssueResponse{
		//				PrivateKey:  pk1PEM,
		//				Certificate: cert1PEM,
		//			}, nil
		//		},
		//	},
		//	Builder: &testpkg.Builder{
		//		KubeObjects: []runtime.Object{
		//			&corev1.Secret{
		//				ObjectMeta: metav1.ObjectMeta{
		//					Namespace: gen.DefaultTestNamespace,
		//					Name:      "output",
		//					SelfLink:  "abc",
		//					Labels: map[string]string{
		//						cmapi.CertificateNameKey: "nottest",
		//					},
		//					Annotations: map[string]string{
		//						"testannotation": "true",
		//					},
		//				},
		//				Data: map[string][]byte{
		//					corev1.TLSCertKey:       cert1PEM,
		//					corev1.TLSPrivateKeyKey: pk1PEM,
		//					TLSCAKey:                nil,
		//				},
		//			},
		//		},
		//		CertManagerObjects: []runtime.Object{gen.Certificate("test")},
		//		ExpectedActions: []testpkg.Action{
		//			testpkg.NewAction(coretesting.NewGetAction(
		//				corev1.SchemeGroupVersion.WithResource("secrets"),
		//				gen.DefaultTestNamespace,
		//				"output",
		//			)),
		//			testpkg.NewAction(coretesting.NewUpdateAction(
		//				corev1.SchemeGroupVersion.WithResource("secrets"),
		//				gen.DefaultTestNamespace,
		//				&corev1.Secret{
		//					ObjectMeta: metav1.ObjectMeta{
		//						Namespace: gen.DefaultTestNamespace,
		//						Name:      "output",
		//						SelfLink:  "abc",
		//						Labels: map[string]string{
		//							cmapi.CertificateNameKey: "test",
		//						},
		//						Annotations: map[string]string{
		//							"testannotation":                 "true",
		//							"certmanager.k8s.io/alt-names":   "example.com",
		//							"certmanager.k8s.io/common-name": "example.com",
		//							"certmanager.k8s.io/issuer-kind": "Issuer",
		//							"certmanager.k8s.io/issuer-name": "test",
		//						},
		//					},
		//					Data: map[string][]byte{
		//						corev1.TLSCertKey:       cert1PEM,
		//						corev1.TLSPrivateKeyKey: pk1PEM,
		//						TLSCAKey:                nil,
		//					},
		//				},
		//			)),
		//		},
		//	},
		//},
	}
	for n, test := range tests {
		t.Run(n, func(t *testing.T) {
			// reset the fixedClock
			fixedClock.SetTime(nowTime)
			test.builder.Clock = fixedClock
			runTestDefault(t, test)
		})
	}
}

func TestDisableOldConfigFeatureFlagDisabled(t *testing.T) {
	nowTime := time.Now()
	nowMetaTime := metav1.NewTime(nowTime)
	fixedClock := clock.NewFakeClock(nowTime)

	iss := gen.Issuer("testissuer",
		gen.SetIssuerACME(cmapi.ACMEIssuer{}),
	)
	// the 'new format' means not specifying any ACMECertificateConfig
	newFormatCertificate := gen.Certificate("test",
		gen.SetCertificateIssuer(cmapi.ObjectReference{
			Name: iss.Name,
		}),
		gen.SetCertificateDNSNames("test.com"),
		gen.SetCertificateSecretName("test-tls"),
	)
	oldFormatCertificate := gen.CertificateFrom(newFormatCertificate,
		gen.SetCertificateACMEConfig(cmapi.ACMECertificateConfig{}),
	)

	tests := map[string]testTDefault{
		"log an event and exit if a certificate that specifies the old config format is processed": {
			certificate: oldFormatCertificate,
			builder: &testpkg.Builder{
				CertManagerObjects: []runtime.Object{
					iss,
				},
				ExpectedEvents: []string{
					`Warning DeprecatedField Deprecated spec.acme field specified and deprecated field feature gate is enabled.`,
				},
			},
		},
		"begin processing the Certificate if it does not specify the old config format": {
			certificate: newFormatCertificate,
			builder: &testpkg.Builder{
				CertManagerObjects: []runtime.Object{
					iss,
					newFormatCertificate,
				},
				ExpectedActions: []testpkg.Action{
					testpkg.NewAction(coretesting.NewUpdateAction(
						cmapi.SchemeGroupVersion.WithResource("certificates"),
						gen.DefaultTestNamespace,
						gen.CertificateFrom(newFormatCertificate,
							gen.SetCertificateStatusCondition(cmapi.CertificateCondition{
								Type:               cmapi.CertificateConditionReady,
								Status:             cmapi.ConditionFalse,
								Reason:             "NotFound",
								Message:            "Certificate does not exist",
								LastTransitionTime: &nowMetaTime,
							}),
						),
					)),
				},
				ExpectedEvents: []string{
					`Warning IssuerNotReady Issuer testissuer not ready`,
				},
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, feature.DisableDeprecatedACMECertificates, true)()
			// reset the fixedClock
			fixedClock.SetTime(nowTime)
			test.builder.Clock = fixedClock
			runTestDefault(t, test)
		})
	}
}

// type name testT is already used by certificate_request_test.go
type testTDefault struct {
	builder             *testpkg.Builder
	issuerImpl          *fake.Issuer
	staticTemporaryCert []byte
	certificate         *cmapi.Certificate
	expectedErr         bool
}

func runTestDefault(t *testing.T, test testTDefault) {
	test.builder.T = t
	test.builder.Init()
	defer test.builder.Stop()

	c := &controller{}
	c.Register(test.builder.Context)
	c.localTemporarySigner = func(crt *cmapi.Certificate, pk []byte) ([]byte, error) {
		if test.staticTemporaryCert == nil {
			return nil, fmt.Errorf("localTemporarySigner not implemented")
		}
		return test.staticTemporaryCert, nil
	}
	c.issuerFactory = &fake.Factory{
		IssuerForFunc: func(cmapi.GenericIssuer) (issuer.Interface, error) {
			if test.issuerImpl == nil {
				return nil, fmt.Errorf("no issuerImpl defined in test fixture")
			}
			return test.issuerImpl, nil
		},
	}
	test.builder.Start()

	err := c.Sync(context.Background(), test.certificate)
	if err != nil && !test.expectedErr {
		t.Errorf("expected to not get an error, but got: %v", err)
	}
	if err == nil && test.expectedErr {
		t.Errorf("expected to get an error but did not get one")
	}

	test.builder.CheckAndFinish(err)
}
