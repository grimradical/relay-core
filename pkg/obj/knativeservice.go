package obj

import (
	"context"

	relayv1beta1 "github.com/puppetlabs/relay-core/pkg/apis/relay.sh/v1beta1"
	"github.com/puppetlabs/relay-core/pkg/authenticate"
	"github.com/puppetlabs/relay-core/pkg/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AmbassadorIDAnnotation        = "getambassador.io/ambassador-id"
	KnativeServiceVisibilityLabel = "serving.knative.dev/visibility"
)

const (
	AmbassadorID                         = "webhook"
	KnativeServiceVisibilityClusterLocal = "cluster-local"
)

var (
	KnativeServiceKind = servingv1.SchemeGroupVersion.WithKind("Service")
)

type KnativeService struct {
	Key    client.ObjectKey
	Object *servingv1.Service
}

var _ Persister = &KnativeService{}
var _ Loader = &KnativeService{}
var _ Ownable = &KnativeService{}
var _ LabelAnnotatableFrom = &KnativeService{}

func (ks *KnativeService) Persist(ctx context.Context, cl client.Client) error {
	if err := CreateOrUpdate(ctx, cl, ks.Key, ks.Object); err != nil {
		return err
	}

	return nil
}

func (ks *KnativeService) Load(ctx context.Context, cl client.Client) (bool, error) {
	return GetIgnoreNotFound(ctx, cl, ks.Key, ks.Object)
}

func (ks *KnativeService) Owned(ctx context.Context, owner Owner) error {
	return Own(ks.Object, owner)
}

func (ks *KnativeService) Label(ctx context.Context, name, value string) {
	Label(&ks.Object.ObjectMeta, name, value)
}

func (ks *KnativeService) Annotate(ctx context.Context, name, value string) {
	Annotate(&ks.Object.ObjectMeta, name, value)
}

func (ks *KnativeService) LabelAnnotateFrom(ctx context.Context, from metav1.ObjectMeta) {
	CopyLabelsAndAnnotations(&ks.Object.ObjectMeta, from)
}

func NewKnativeService(key client.ObjectKey) *KnativeService {
	return &KnativeService{
		Key:    key,
		Object: &servingv1.Service{},
	}
}

func ConfigureKnativeService(ctx context.Context, s *KnativeService, wtd *WebhookTriggerDeps) error {
	// FIXME This should be configurable
	s.Annotate(ctx, AmbassadorIDAnnotation, AmbassadorID)
	s.Label(ctx, KnativeServiceVisibilityLabel, KnativeServiceVisibilityClusterLocal)
	s.LabelAnnotateFrom(ctx, wtd.WebhookTrigger.Object.ObjectMeta)

	// Owned by the owner ConfigMap so we only have to worry about deleting one
	// thing.
	if err := wtd.OwnerConfigMap.Own(ctx, s); err != nil {
		return err
	}

	// We also set this as a dependency of the webhook trigger so that changes
	// to the service will propagate back using our event handler.
	SetDependencyOf(&s.Object.ObjectMeta, Owner{Object: wtd.WebhookTrigger.Object, GVK: relayv1beta1.WebhookTriggerKind})

	template := servingv1.RevisionTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				model.RelayControllerWebhookTriggerIDLabel: wtd.WebhookTrigger.Key.Name,
			},
			// Keep any existing token annotations.
			Annotations: map[string]string{
				model.RelayControllerTokenHashAnnotation: s.Object.Spec.ConfigurationSpec.Template.GetAnnotations()[model.RelayControllerTokenHashAnnotation],
				authenticate.KubernetesTokenAnnotation:   s.Object.Spec.ConfigurationSpec.Template.GetAnnotations()[authenticate.KubernetesTokenAnnotation],
				authenticate.KubernetesSubjectAnnotation: s.Object.Spec.ConfigurationSpec.Template.GetAnnotations()[authenticate.KubernetesSubjectAnnotation],
			},
		},
		Spec: servingv1.RevisionSpec{
			PodSpec: corev1.PodSpec{
				ServiceAccountName: wtd.KnativeServiceAccount.Key.Name,
			},
		},
	}

	image := wtd.WebhookTrigger.Object.Spec.Image
	if image == "" {
		// Theoretically someone could write some socat action and use the
		// Alpine image, so we leave this here for consistency.
		image = model.DefaultImage
	}

	container := corev1.Container{
		Name:            wtd.WebhookTrigger.Object.Name,
		Image:           image,
		ImagePullPolicy: corev1.PullAlways,
		Env: []corev1.EnvVar{
			{
				Name:  "METADATA_API_URL",
				Value: wtd.MetadataAPIURL.String(),
			},
		},
	}

	if len(wtd.WebhookTrigger.Object.Spec.Input) > 0 {
		tm := ModelWebhookTrigger(wtd.WebhookTrigger)

		template.Spec.PodSpec.Volumes = append(template.Spec.PodSpec.Volumes, corev1.Volume{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: wtd.ImmutableConfigMap.Key.Name,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  scriptConfigMapKey(tm),
							Path: "input-script",
							Mode: func(i int32) *int32 { return &i }(0755),
						},
					},
				},
			},
		})

		container.VolumeMounts = []corev1.VolumeMount{
			{
				Name:      "config",
				ReadOnly:  true,
				MountPath: "/var/run/puppet/relay/config",
			},
		}
		container.Command = []string{"/var/run/puppet/relay/config/input-script"}
	} else {
		if command := wtd.WebhookTrigger.Object.Spec.Command; command != "" {
			container.Command = []string{command}
		}

		if args := wtd.WebhookTrigger.Object.Spec.Args; len(args) > 0 {
			container.Args = args
		}
	}

	template.Spec.PodSpec.Containers = []corev1.Container{container}

	if err := wtd.AnnotateTriggerToken(ctx, &template.ObjectMeta); err != nil {
		return err
	}

	s.Object.Spec = servingv1.ServiceSpec{
		ConfigurationSpec: servingv1.ConfigurationSpec{
			Template: template,
		},
	}

	return nil
}

func ApplyKnativeService(ctx context.Context, cl client.Client, wtd *WebhookTriggerDeps) (*KnativeService, error) {
	s := NewKnativeService(client.ObjectKey{
		Namespace: wtd.TenantDeps.Namespace.Name,
		Name:      wtd.WebhookTrigger.Key.Name,
	})

	if _, err := s.Load(ctx, cl); err != nil {
		return nil, err
	}

	if err := ConfigureKnativeService(ctx, s, wtd); err != nil {
		return nil, err
	}

	if err := s.Persist(ctx, cl); err != nil {
		return nil, err
	}

	return s, nil
}

type KnativeServiceResult struct {
	KnativeService *KnativeService
	Error          error
}

func AsKnativeServiceResult(ks *KnativeService, err error) *KnativeServiceResult {
	return &KnativeServiceResult{
		KnativeService: ks,
		Error:          err,
	}
}
