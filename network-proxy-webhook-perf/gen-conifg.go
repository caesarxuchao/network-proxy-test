package main

import (
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/caesarxuchao/network-proxy-test/network-proxy-webhook-perf/utils"
	"github.com/ghodss/yaml"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/utils/pointer"
)

type certContext struct {
	cert        []byte
	key         []byte
	signingCert []byte
}

// Setup the server cert. For example, user apiservers and admission webhooks
// can use the cert to prove their identify to the kube-apiserver
func setupServerCert(namespaceName, serviceName string) *certContext {
	certDir, err := ioutil.TempDir("", "test-e2e-server-cert")
	if err != nil {
		panic(fmt.Errorf("Failed to create a temp dir for cert generation %v", err))
	}
	defer os.RemoveAll(certDir)
	signingKey, err := utils.NewPrivateKey()
	if err != nil {
		panic(fmt.Errorf("Failed to create CA private key %v", err))
	}
	signingCert, err := cert.NewSelfSignedCACert(cert.Config{CommonName: "e2e-server-cert-ca"}, signingKey)
	if err != nil {
		panic(fmt.Errorf("Failed to create CA cert for apiserver %v", err))
	}
	caCertFile, err := ioutil.TempFile(certDir, "ca.crt")
	if err != nil {
		panic(fmt.Errorf("Failed to create a temp file for ca cert generation %v", err))
	}
	if err := ioutil.WriteFile(caCertFile.Name(), utils.EncodeCertPEM(signingCert), 0644); err != nil {
		panic(fmt.Errorf("Failed to write CA cert %v", err))
	}
	key, err := utils.NewPrivateKey()
	if err != nil {
		panic(fmt.Errorf("Failed to create private key for %v", err))
	}
	signedCert, err := utils.NewSignedCert(
		&cert.Config{
			CommonName: serviceName + "." + namespaceName + ".svc",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		},
		key, signingCert, signingKey,
	)
	if err != nil {
		panic(fmt.Errorf("Failed to create cert%v", err))
	}
	certFile, err := ioutil.TempFile(certDir, "server.crt")
	if err != nil {
		panic(fmt.Errorf("Failed to create a temp file for cert generation %v", err))
	}
	keyFile, err := ioutil.TempFile(certDir, "server.key")
	if err != nil {
		panic(fmt.Errorf("Failed to create a temp file for key generation %v", err))
	}
	if err = ioutil.WriteFile(certFile.Name(), utils.EncodeCertPEM(signedCert), 0600); err != nil {
		panic(fmt.Errorf("Failed to write cert file %v", err))
	}
	privateKeyPEM, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		panic(fmt.Errorf("Failed to marshal key %v", err))
	}
	if err = ioutil.WriteFile(keyFile.Name(), privateKeyPEM, 0644); err != nil {
		panic(fmt.Errorf("Failed to write key file %v", err))
	}
	return &certContext{
		cert:        utils.EncodeCertPEM(signedCert),
		key:         privateKeyPEM,
		signingCert: utils.EncodeCertPEM(signingCert),
	}
}

func createSecret(name string, context *certContext) *v1.Secret {
	return &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Type: v1.SecretTypeOpaque,
		Data: map[string][]byte{
			"tls.crt": context.cert,
			"tls.key": context.key,
		},
	}
}

const (
	servicePort   = int32(8443)
	containerPort = int32(8444)
)

var (
	podLabels     = map[string]string{"app": "sample-webhook", "webhook": "true"}
	serviceLabels = map[string]string{"webhook": "true"}
	zero          = int64(0)
	replicas      = int32(1)
)

func createSecretAndDeployment(name string, index int, context *certContext) (*v1.Secret, *appsv1.Deployment) {
	secret := createSecret(name, context)
	mounts := []v1.VolumeMount{
		{
			Name:      "webhook-certs",
			ReadOnly:  true,
			MountPath: "/webhook.local.config/certificates",
		},
	}
	volumes := []v1.Volume{
		{
			Name: "webhook-certs",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{SecretName: name},
			},
		},
	}
	containers := []v1.Container{
		{
			Name:         "sample-webhook",
			VolumeMounts: mounts,
			Args: []string{
				"webhook",
				"--tls-cert-file=/webhook.local.config/certificates/tls.crt",
				"--tls-private-key-file=/webhook.local.config/certificates/tls.key",
				"--alsologtostderr",
				"-v=4",
				// Use a non-default port for containers.
				fmt.Sprintf("--port=%d", containerPort),
			},
			ReadinessProbe: &v1.Probe{
				Handler: v1.Handler{
					HTTPGet: &v1.HTTPGetAction{
						Scheme: v1.URISchemeHTTPS,
						Port:   intstr.FromInt(int(containerPort)),
						Path:   "/readyz",
					},
				},
				PeriodSeconds:    1,
				SuccessThreshold: 1,
				FailureThreshold: 30,
			},
			Image:           "gcr.io/chao1-149704/agnhost:2.10",
			ImagePullPolicy: v1.PullAlways,
			Ports:           []v1.ContainerPort{{ContainerPort: containerPort}},
		},
	}
	return secret, &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"index": fmt.Sprintf("%d", index),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"index": fmt.Sprintf("%d", index),
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"index": fmt.Sprintf("%d", index),
					},
				},
				Spec: v1.PodSpec{
					TerminationGracePeriodSeconds: &zero,
					Containers:                    containers,
					Volumes:                       volumes,
				},
			},
		},
	}
}

func strPtr(s string) *string { return &s }

// name is the service name
func newMutateConfigMapWebhookFixture(name string, context *certContext) admissionregistrationv1.MutatingWebhook {
	sideEffectsNone := admissionregistrationv1.SideEffectClassNone
	return admissionregistrationv1.MutatingWebhook{
		Name: fmt.Sprintf("%s.k8s.io", name),
		Rules: []admissionregistrationv1.RuleWithOperations{{
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{""},
				APIVersions: []string{"v1"},
				Resources:   []string{"configmaps"},
			},
		}},
		ClientConfig: admissionregistrationv1.WebhookClientConfig{
			Service: &admissionregistrationv1.ServiceReference{
				Namespace: "default",
				Name:      name,
				Path:      strPtr("/mutating-configmaps"),
				Port:      pointer.Int32Ptr(servicePort),
			},
			CABundle: context.signingCert,
		},
		SideEffects:             &sideEffectsNone,
		AdmissionReviewVersions: []string{"v1", "v1beta1"},
	}
}

func createWebhookRegistration(name string, context *certContext) *admissionregistrationv1.MutatingWebhookConfiguration {
	return &admissionregistrationv1.MutatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1",
			Kind:       "MutatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			newMutateConfigMapWebhookFixture(name, context),
		},
	}
}

func createService(name string, index int) *v1.Service {
	return &v1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"index": fmt.Sprintf("%d", index),
			},
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"index": fmt.Sprintf("%d", index),
			},
			Ports: []v1.ServicePort{
				{
					Protocol:   "TCP",
					Port:       servicePort,
					TargetPort: intstr.FromInt(int(containerPort)),
				},
			},
		},
	}
}

func main() {
	var config string
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("proxy-perf-%d", i)
		cert := setupServerCert("default", name)
		secret, deployment := createSecretAndDeployment(name, i, cert)
		service := createService(name, i)
		webhookReg := createWebhookRegistration(name, cert)
		sd, err := yaml.Marshal(secret)
		if err != nil {
			panic(err)
		}
		dd, err := yaml.Marshal(deployment)
		if err != nil {
			panic(err)
		}
		ssd, err := yaml.Marshal(service)
		if err != nil {
			panic(err)
		}
		wd, err := yaml.Marshal(webhookReg)
		if err != nil {
			panic(err)
		}
		config = config + fmt.Sprintf("---\n%s---\n%s---\n%s---\n%s", sd, dd, ssd, wd)
	}
	err := ioutil.WriteFile("./giant-config.yaml", []byte(config), 0644)
	if err != nil {
		panic(err)
	}
}
