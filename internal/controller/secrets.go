package controller

import (
	"context"
	"time"

	atroxyzv1alpha1 "github.com/atropos112/atrok.git/api/v1alpha1"
	"github.com/atropos112/gocore/utils"
	extsec "github.com/external-secrets/external-secrets/apis/externalsecrets/v1beta1"
	equality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/*
	SecretStoreRef *string                        `json:"secretStoreRef,omitempty"`
	SecretEnvs     map[string]string              `json:"secretEnvs,omitempty"`
*/

// CreateExpectedExternalSecret creates the expected external secret from the appbundle or returns nil if no secret is needed
func CreateExpectedExternalSecret(ab *atroxyzv1alpha1.AppBundle) (*extsec.ExternalSecret, error) {
	expectedExternalSecret := &extsec.ExternalSecret{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}

	// GATHERING ALL SECRETS NEEDED
	// Key is secret key, value is the remote ref.
	secretsToGet := make(map[string]string)

	for key, value := range ab.Spec.SourcedEnvs {
		if value.ExternalSecret != "" {
			secretsToGet[key] = value.ExternalSecret
		}
	}

	for _, cfg := range ab.Spec.Configs {
		for secretKey, secret := range cfg.Secrets {
			secretsToGet[secretKey] = secret
		}
	}

	if len(secretsToGet) == 0 {
		// No secrets to get, no need for an external secret
		return nil, nil
	}

	if ab.Spec.SecretStoreRef == nil {
		return nil, &utils.DeveloperError{Message: "SecretStoreRef is nil"}
	}

	data := make([]extsec.ExternalSecretData, 0, len(secretsToGet))

	for _, key := range getSortedKeys(secretsToGet) {
		data = append(data, extsec.ExternalSecretData{
			SecretKey: key,
			RemoteRef: extsec.ExternalSecretDataRemoteRef{
				Key:                secretsToGet[key],
				DecodingStrategy:   extsec.ExternalSecretDecodeNone,
				ConversionStrategy: extsec.ExternalSecretConversionDefault,
				MetadataPolicy:     extsec.ExternalSecretMetadataPolicyNone,
			},
		})
	}

	templates := map[string]string{}
	for _, key := range getSortedKeys(ab.Spec.SourcedEnvs) {
		if ab.Spec.SourcedEnvs[key].ExternalSecret != "" {
			templates["env"+key] = "{{ ." + key + " }}"
		}
	}

	for key, cfg := range ab.Spec.Configs {
		if len(cfg.Secrets) != 0 {
			templates["cfg"+key] = cfg.Content
		}
	}

	target := extsec.ExternalSecretTarget{
		Name: ab.Name,
		Template: &extsec.ExternalSecretTemplate{
			EngineVersion: "v2",
			Data:          templates,
			MergePolicy:   extsec.MergePolicyReplace,
		},
		CreationPolicy: extsec.CreatePolicyOwner,
		DeletionPolicy: extsec.DeletionPolicyDelete,
	}

	refreshInterval := metav1.Duration{Duration: time.Duration(15 * time.Minute)}
	expectedExternalSecret.Spec = extsec.ExternalSecretSpec{
		SecretStoreRef: extsec.SecretStoreRef{
			Name: *ab.Spec.SecretStoreRef,
			Kind: "SecretStore",
		},
		RefreshInterval: &refreshInterval,
		Data:            data,
		Target:          target,
	}

	return expectedExternalSecret, nil
}

// ReconcileExternalSecret reconciles the service for the appbundle
func (r *AppBundleReconciler) ReconcileExternalSecret(ctx context.Context, ab *atroxyzv1alpha1.AppBundle) error {
	// LOCK the resource
	mu := getMutex("extsec", ab.Name, ab.Namespace)
	mu.Lock()
	defer mu.Unlock()

	// GET THE CURRENT EXTERNALSECRET
	currentExternalSecret := &extsec.ExternalSecret{ObjectMeta: GetAppBundleObjectMetaWithOwnerReference(ab)}
	er := r.Get(ctx, client.ObjectKeyFromObject(currentExternalSecret), currentExternalSecret)

	// GET THE EXPECTED EXTERNALSECRET
	expectedExternalSecret, err := CreateExpectedExternalSecret(ab)
	if err != nil {
		return err
	}

	// There is no exterernal secret and no need for one, leave now
	if errors.IsNotFound(er) && expectedExternalSecret == nil {
		return nil
	}

	// There is external secret but no need for one, delete it
	if expectedExternalSecret == nil {
		// IN case a different error happened by now and wasn't accounted for yet
		if er != nil {
			return er
		}

		// By now we know there was no error getting current ext secret (so there is one)
		// And the expected one, is expected to not be there so we delete
		return r.Delete(ctx, currentExternalSecret)
	}

	if !equality.Semantic.DeepDerivative(expectedExternalSecret.Spec, currentExternalSecret.Spec) {
		reason, err := FormulateDiffMessageForSpecs(currentExternalSecret.Spec, expectedExternalSecret.Spec)
		if err != nil {
			return err
		}

		// Delete first (as ExternalSecrets is not so happy about mutations)
		if r.Delete(ctx, currentExternalSecret) != nil {
			return err
		}

		return UpsertResource(ctx, r, expectedExternalSecret, reason, er, false)
	}

	return nil
}
