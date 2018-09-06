package pki

import (
	"bytes"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CommonNameForCertificate(crt *v1alpha1.Certificate) string {
	if crt.Spec.CommonName != "" {
		return crt.Spec.CommonName
	}
	if len(crt.Spec.DNSNames) == 0 {
		return ""
	}
	return crt.Spec.DNSNames[0]
}

func DNSNamesForCertificate(crt *v1alpha1.Certificate) []string {
	if len(crt.Spec.DNSNames) == 0 {
		if crt.Spec.CommonName == "" {
			return []string{}
		}
		return []string{crt.Spec.CommonName}
	}
	if crt.Spec.CommonName != "" {
		return util.RemoveDuplicates(append([]string{crt.Spec.CommonName}, crt.Spec.DNSNames...))
	}
	return crt.Spec.DNSNames
}

func ValidityPeriodForCertificate(crt *v1alpha1.Certificate) int {
	if crt.Spec.ValidityPeriod > 0 {
		return crt.Spec.ValidityPeriod
	}
	return 0
}

var serialNumberLimit = new(big.Int).Lsh(big.NewInt(1), 128)

// TODO: allow this to be configurable
const defaultOrganization = "cert-manager"

const defaultSignatureAlgorithm = x509.SHA256WithRSA

// default certification duration is 1 year
const defaultNotAfter = time.Hour * 24 * 365

func GenerateCSR(issuer v1alpha1.GenericIssuer, crt *v1alpha1.Certificate) (*x509.CertificateRequest, error) {
	commonName := CommonNameForCertificate(crt)
	dnsNames := DNSNamesForCertificate(crt)
	if len(commonName) == 0 && len(dnsNames) == 0 {
		return nil, fmt.Errorf("no domains specified on certificate")
	}

	return &x509.CertificateRequest{
		Version:            3,
		SignatureAlgorithm: defaultSignatureAlgorithm,
		Subject: pkix.Name{
			Organization: []string{defaultOrganization},
			CommonName:   commonName,
		},
		DNSNames: dnsNames,
		// TODO: work out how best to handle extensions/key usages here
		ExtraExtensions: []pkix.Extension{},
	}, nil
}

// GenerateTemplate will create a x509.Certificate for the given Certificate resource.
// This should create a Certificate template that is equivalent to the CertificateRequest
// generated by GenerateCSR.
// The PublicKey field must be populated by the caller.
func GenerateTemplate(issuer v1alpha1.GenericIssuer, crt *v1alpha1.Certificate, serialNo *big.Int) (*x509.Certificate, error) {
	commonName := CommonNameForCertificate(crt)
	dnsNames := DNSNamesForCertificate(crt)
	validityPeriod := time.Duration(ValidityPeriodForCertificate(crt))
	if len(commonName) == 0 && len(dnsNames) == 0 {
		return nil, fmt.Errorf("no domains specified on certificate")
	}

	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %s", err.Error())
	}

	if validityPeriod != 0 {
		validityPeriod = time.Hour * validityPeriod
	} else {
		validityPeriod = defaultNotAfter
	}

	expireTime := time.Now().Add(validityPeriod)
	crt.Status.NotAfter = metav1.NewTime(expireTime)

	return &x509.Certificate{
		Version:               3,
		BasicConstraintsValid: true,
		SerialNumber:          serialNumber,
		SignatureAlgorithm:    defaultSignatureAlgorithm,
		Subject: pkix.Name{
			Organization: []string{defaultOrganization},
			CommonName:   commonName,
		},
		NotBefore: time.Now(),
		NotAfter:  expireTime,
		// see http://golang.org/pkg/crypto/x509/#KeyUsage
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		DNSNames: dnsNames,
	}, nil
}

// SignCertificate returns a signed x509.Certificate object for the given
// *v1alpha1.Certificate crt.
// publicKey is the public key of the signee, and signerKey is the private
// key of the signer.
func SignCertificate(template *x509.Certificate, issuerCert *x509.Certificate, publicKey interface{}, signerKey interface{}) ([]byte, *x509.Certificate, error) {
	derBytes, err := x509.CreateCertificate(rand.Reader, template, issuerCert, publicKey, signerKey)

	if err != nil {
		return nil, nil, fmt.Errorf("error creating x509 certificate: %s", err.Error())
	}

	cert, err := DecodeDERCertificateBytes(derBytes)

	if err != nil {
		return nil, nil, fmt.Errorf("error decoding DER certificate bytes: %s", err.Error())
	}

	pemBytes := bytes.NewBuffer([]byte{})
	err = pem.Encode(pemBytes, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		return nil, nil, fmt.Errorf("error encoding certificate PEM: %s", err.Error())
	}

	// bundle the CA
	err = pem.Encode(pemBytes, &pem.Block{Type: "CERTIFICATE", Bytes: issuerCert.Raw})
	if err != nil {
		return nil, nil, fmt.Errorf("error encoding issuer cetificate PEM: %s", err.Error())
	}

	return pemBytes.Bytes(), cert, err
}

func EncodeCSR(template *x509.CertificateRequest, key interface{}) ([]byte, error) {
	derBytes, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	if err != nil {
		return nil, fmt.Errorf("error creating x509 certificate: %s", err.Error())
	}

	return derBytes, nil
}
