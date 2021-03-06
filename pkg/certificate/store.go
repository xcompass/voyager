package certificate

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/appscode/go/errors"
	v1u "github.com/appscode/kutil/core/v1"
	vu "github.com/appscode/kutil/voyager/v1beta1"
	api "github.com/appscode/voyager/apis/voyager/v1beta1"
	acs "github.com/appscode/voyager/client/typed/voyager/v1beta1"
	vault "github.com/hashicorp/vault/api"
	"github.com/xenolf/lego/acme"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

type CertStore struct {
	KubeClient    clientset.Interface
	VoyagerClient acs.VoyagerV1beta1Interface
	VaultClient   *vault.Client
}

func NewCertStore(kubeClient clientset.Interface, voyagerClient acs.VoyagerV1beta1Interface) (*CertStore, error) {
	store := &CertStore{
		KubeClient:    kubeClient,
		VoyagerClient: voyagerClient,
	}

	var err error
	if os.Getenv(vault.EnvVaultAddress) != "" {
		store.VaultClient, err = vault.NewClient(vault.DefaultConfig())
		if err != nil {
			return nil, err
		}
		if os.Getenv(vault.EnvVaultToken) == "" {
			token, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
			if err != nil {
				return nil, err
			}
			store.VaultClient.SetToken(string(token))
		}
	}
	return store, nil
}

func (s *CertStore) Get(crd *api.Certificate) (pemCrt, pemKey []byte, err error) {
	if crd.Spec.Storage.Secret != nil {
		var secret *apiv1.Secret
		secret, err = s.KubeClient.CoreV1().Secrets(crd.Namespace).Get(crd.SecretName(), metav1.GetOptions{})
		if k8serror.IsNotFound(err) {
			return nil, nil, nil
		}
		if err == nil {
			if data, found := secret.Data[apiv1.TLSCertKey]; !found {
				err = fmt.Errorf("secret %s@%s is missing tls.crt", crd.SecretName(), crd.Namespace)
				return
			} else {
				pemCrt = data
			}
			if data, found := secret.Data[apiv1.TLSPrivateKeyKey]; !found {
				err = fmt.Errorf("secret %s@%s is missing tls.key", crd.SecretName(), crd.Namespace)
				return
			} else {
				pemKey = data
			}
		}
	} else if crd.Spec.Storage.Vault != nil {
		var secret *vault.Secret
		secret, err = s.VaultClient.Logical().Read(path.Join(crd.Spec.Storage.Vault.Prefix, crd.Namespace, crd.SecretName()))
		if err != nil {
			return
		}
		if secret == nil && err == nil {
			return nil, nil, nil
		}

		if data, found := secret.Data[apiv1.TLSCertKey]; !found {
			err = fmt.Errorf("secret %s@%s is missing tls.crt", crd.SecretName(), crd.Namespace)
			return
		} else {
			pemCrt = []byte(data.(string))
		}
		if data, found := secret.Data[apiv1.TLSPrivateKeyKey]; !found {
			err = fmt.Errorf("secret %s@%s is missing tls.key", crd.SecretName(), crd.Namespace)
			return
		} else {
			pemKey = []byte(data.(string))
		}
	}
	return
}

func (s *CertStore) Save(crd *api.Certificate, cert acme.CertificateResource) error {
	if crd.Spec.Storage.Secret != nil {
		_, err := v1u.CreateOrPatchSecret(s.KubeClient,
			metav1.ObjectMeta{Namespace: crd.Namespace, Name: crd.SecretName()},
			func(in *apiv1.Secret) *apiv1.Secret {
				in.Type = apiv1.SecretTypeTLS
				if in.Data == nil {
					in.Data = make(map[string][]byte)
				}
				in.Data[apiv1.TLSCertKey] = cert.Certificate
				in.Data[apiv1.TLSPrivateKeyKey] = cert.PrivateKey
				return in
			})
		return err
	} else if crd.Spec.Storage.Vault != nil {
		data := map[string]interface{}{
			apiv1.TLSCertKey:       string(cert.Certificate),
			apiv1.TLSPrivateKeyKey: string(cert.PrivateKey),
		}
		_, err := s.VaultClient.Logical().Write(path.Join(crd.Spec.Storage.Vault.Prefix, crd.Namespace, crd.SecretName()), data)
		return err
	}

	// Decode cert
	pemBlock, _ := pem.Decode(cert.Certificate)
	crt, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return errors.FromErr(err).WithMessage("Error decoding x509 encoded certificate").Err()
	}
	_, err = vu.PatchCertificate(s.VoyagerClient, crd, func(in *api.Certificate) *api.Certificate {
		// Update certificate data to add Details Information
		t := metav1.Now()
		in.Status.LastIssuedCertificate = &api.CertificateDetails{
			SerialNumber:  crt.SerialNumber.String(),
			NotBefore:     metav1.NewTime(crt.NotBefore),
			NotAfter:      metav1.NewTime(crt.NotAfter),
			CertURL:       cert.CertURL,
			CertStableURL: cert.CertStableURL,
			AccountRef:    cert.AccountRef,
		}

		found := false
		for i := range in.Status.Conditions {
			if in.Status.Conditions[i].Type == api.CertificateIssued {
				in.Status.Conditions[i].LastUpdateTime = t
				found = true
			}
		}
		if !found {
			in.Status.Conditions = append(in.Status.Conditions, api.CertificateCondition{
				Type:           api.CertificateIssued,
				LastUpdateTime: t,
			})
		}
		return in
	})
	return err
}
