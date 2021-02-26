module github.com/jetstack/cert-manager

go 1.12

require (
	cloud.google.com/go v0.39.0
	github.com/Azure/azure-sdk-for-go v32.5.0+incompatible
	github.com/Azure/go-autorest/autorest v0.9.0
	github.com/Azure/go-autorest/autorest/adal v0.5.0
	github.com/Azure/go-autorest/autorest/to v0.3.0
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/Venafi/vcert v0.0.0-20190613103158-62139eb19b25
	github.com/aws/aws-sdk-go v1.24.1
	github.com/cloudflare/cloudflare-go v0.8.5
	github.com/coreos/etcd v3.3.24+incompatible
	github.com/cpu/goacmedns v0.0.0-20180701200144-565ecf2a84df
	github.com/digitalocean/godo v1.6.0
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/go-openapi/spec v0.19.2
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/google/gofuzz v1.0.0
	github.com/gorilla/context v1.1.1 // indirect
	github.com/gorilla/mux v1.6.2
	github.com/gorilla/websocket v1.4.2
	github.com/hashicorp/vault/api v1.0.4
	github.com/hashicorp/vault/sdk v0.1.13
	github.com/kr/pretty v0.1.0
	github.com/mattbaird/jsonpatch v0.0.0-20171005235357-81af80346b1a
	github.com/miekg/dns v1.0.15
	github.com/munnerz/goautoneg v0.0.0-20190414153302-2ae31c8b6b30 // indirect
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/openshift/generic-admission-server v1.14.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.0.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.4.0
	github.com/tent/http-link-go v0.0.0-20130702225549-ac974c61c2f9 // indirect
	go.etcd.io/etcd v3.3.24+incompatible // indirect
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550
	golang.org/x/net v0.0.0-20191004110552-13f9640d40b9
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	google.golang.org/api v0.5.0
	gopkg.in/ini.v1 v1.42.0 // indirect
	k8s.io/api v0.16.5
	k8s.io/apiextensions-apiserver v0.0.0-20191114105449-027877536833
	k8s.io/apimachinery v0.16.5
	k8s.io/apiserver v0.0.0-20191114103151-9ca1dc586682
	k8s.io/client-go v0.16.5
	k8s.io/code-generator v0.0.0-20191004115455-8e001e5d1894
	k8s.io/component-base v0.0.0-20191114102325-35a9586014f7
	k8s.io/klog v1.0.0
	k8s.io/kube-aggregator v0.0.0-20191114103820-f023614fb9ea
	k8s.io/kube-openapi v0.0.0-20191107075043-30be4d16710a
	k8s.io/utils v0.0.0-20190801114015-581e00157fb1
	launchpad.net/gocheck v0.0.0-20140225173054-000000000087 // indirect
	sigs.k8s.io/controller-runtime v0.3.1-0.20191022174215-ad57a976ffa1
	sigs.k8s.io/controller-tools v0.2.2
	sigs.k8s.io/testing_frameworks v0.1.1
)

replace (
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.4
	golang.org/x/text => golang.org/x/text v0.3.3 // CVE-2020-14040
)
