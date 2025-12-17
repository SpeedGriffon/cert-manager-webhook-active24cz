package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"cert-manager-webhook-active24cz/api"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"

	corev1 "k8s.io/api/core/v1"
	extev1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var GroupName = os.Getenv("GROUP_NAME")

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	// This will register our custom DNS provider with the webhook serving
	// library, making it available as an API under the provided GroupName.
	// You can register multiple DNS provider implementations with a single
	// webhook, where the Name() method will be used to disambiguate between
	// the different implementations.
	cmd.RunWebhookServer(GroupName, &active24czSolver{})
}

// active24czSolver implements the provider-specific logic needed to
// 'present' an ACME challenge TXT record for active24.cz.
// To do so, it must implement the `github.com/cert-manager/cert-manager/pkg/acme/webhook.Solver`
// interface.
type active24czSolver struct {
	client *kubernetes.Clientset
}

// active24czConfig is a structure that is used to decode into when
// solving a DNS01 challenge.
type active24czConfig struct {
	// These fields will be set by users in the
	// `issuer.spec.acme.dns01.providers.webhook.config` field.
	APIKeySecretRef corev1.SecretKeySelector `json:"apiKeySecretRef"`
}

// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
// This should be unique **within the group name**, i.e. you can have two
// solvers configured with the same Name() **so long as they do not co-exist
// within a single webhook deployment**.
func (c *active24czSolver) Name() string {
	return "active24cz"
}

// Present is responsible for actually presenting the DNS record with the
// DNS provider.
// This method should tolerate being called multiple times with the same value.
// cert-manager itself will later perform a self check to ensure that the
// solver has correctly configured the DNS provider.
func (c *active24czSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	return c.makeRequest(ch, (*api.Client).CreateRecord)
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (c *active24czSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	return c.makeRequest(ch, (*api.Client).DeleteRecord)
}

// Initialize will be called when the webhook first starts.
// Typically, the kubeClientConfig parameter is used to build a Kubernetes
// client that can be used to fetch resources from the Kubernetes API, e.g.
// Secret resources containing credentials used to authenticate with DNS
// provider accounts.
// The stopCh can be used to handle early termination of the webhook, in cases
// where a SIGTERM or similar signal is sent to the webhook process.
func (c *active24czSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}
	c.client = cl
	return nil
}

// Make a DNS API request.
func (c *active24czSolver) makeRequest(ch *v1alpha1.ChallengeRequest, req api.Request) error {
	secret, err := c.loadSecret(ch)
	if err != nil {
		return err
	}
	return req(newClient(ch, secret), dnsRecord(ch))
}

// Load a Kubernetes secret.
func (c *active24czSolver) loadSecret(ch *v1alpha1.ChallengeRequest) (*corev1.Secret, error) {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return nil, err
	}

	ns := ch.ResourceNamespace
	name := cfg.APIKeySecretRef.Name

	secret, err := c.client.CoreV1().Secrets(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to load secret %s/%s: %w", ns, name, err)
	}

	return secret, nil
}

// Create a new client using data from a secret.
func newClient(ch *v1alpha1.ChallengeRequest, secret *corev1.Secret) *api.Client {
	return api.NewClient(
		&api.Config{
			ApiKey:    string(secret.Data["apiKey"]),
			ApiSecret: string(secret.Data["apiSecret"]),
			DnsZone:   strings.TrimSuffix("."+ch.ResolvedZone, "."),
			ServiceId: string(secret.Data["serviceId"]),
		})
}

// Create a DNS record for a challenge request.
func dnsRecord(ch *v1alpha1.ChallengeRequest) *api.DnsRecord {
	return &api.DnsRecord{
		Type:    "TXT",
		Name:    strings.TrimSuffix(ch.ResolvedFQDN, "."+ch.ResolvedZone),
		Content: ch.Key,
		Ttl:     300,
	}
}

// loadConfig is a small helper function that decodes JSON configuration into
// the typed config struct.
func loadConfig(cfgJSON *extev1.JSON) (active24czConfig, error) {
	cfg := active24czConfig{}
	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}

	return cfg, nil
}
