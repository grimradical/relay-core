package e2e_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	relayv1beta1 "github.com/puppetlabs/relay-core/pkg/apis/relay.sh/v1beta1"
	"github.com/puppetlabs/relay-core/pkg/obj"
	"github.com/puppetlabs/relay-core/pkg/util/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestTenantFinalizer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	WithConfig(t, ctx, []ConfigOption{
		ConfigWithTenantReconciler,
	}, func(cfg *Config) {
		child := fmt.Sprintf("%s-child", cfg.Namespace.GetName())

		tenant := &relayv1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cfg.Namespace.GetName(),
				Name:      "my-test-tenant",
			},
			Spec: relayv1beta1.TenantSpec{
				NamespaceTemplate: relayv1beta1.NamespaceTemplate{
					Metadata: metav1.ObjectMeta{
						Name: child,
					},
				},
			},
		}
		require.NoError(t, e2e.ControllerRuntimeClient.Create(ctx, tenant))

		// Wait for namespace.
		require.NoError(t, retry.Retry(ctx, 500*time.Millisecond, func() *retry.RetryError {
			if err := e2e.ControllerRuntimeClient.Get(ctx, client.ObjectKey{
				Namespace: tenant.GetNamespace(),
				Name:      tenant.GetName(),
			}, tenant); err != nil {
				return retry.RetryPermanent(err)
			}

			for _, cond := range tenant.Status.Conditions {
				if cond.Type == relayv1beta1.TenantNamespaceReady && cond.Status == corev1.ConditionTrue {
					return retry.RetryPermanent(nil)
				}
			}

			return retry.RetryTransient(fmt.Errorf("waiting for namespace to be ready"))
		}))

		// Get child namespace.
		namespace := &corev1.Namespace{}
		require.NoError(t, e2e.ControllerRuntimeClient.Get(ctx, client.ObjectKey{Name: child}, namespace))

		// Delete tenant.
		require.NoError(t, e2e.ControllerRuntimeClient.Delete(ctx, tenant))

		// Get child namespace again, should be gone after delete.
		require.NoError(t, retry.Retry(ctx, 500*time.Millisecond, func() *retry.RetryError {
			if err := e2e.ControllerRuntimeClient.Get(ctx, client.ObjectKey{Name: child}, namespace); errors.IsNotFound(err) {
				return retry.RetryPermanent(nil)
			} else if err != nil {
				return retry.RetryPermanent(err)
			}

			return retry.RetryTransient(fmt.Errorf("waiting for namespace to terminate"))
		}))
	})
}

func TestTenantAPITriggerEventSinkMissingSecret(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	WithConfig(t, ctx, []ConfigOption{
		ConfigWithTenantReconciler,
	}, func(cfg *Config) {
		// Create tenant with event sink pointing at nonexistent secret.
		tenant := &relayv1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cfg.Namespace.GetName(),
				Name:      "my-test-tenant",
			},
			Spec: relayv1beta1.TenantSpec{
				TriggerEventSink: relayv1beta1.TriggerEventSink{
					API: &relayv1beta1.APITriggerEventSink{
						URL: "http://stub.example.com",
						TokenFrom: &relayv1beta1.APITokenSource{
							SecretKeyRef: &relayv1beta1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "xyz",
								},
								Key: "test",
							},
						},
					},
				},
			},
		}
		require.NoError(t, e2e.ControllerRuntimeClient.Create(ctx, tenant))

		// Wait for tenant to reconcile.
		var cond relayv1beta1.TenantCondition
		require.NoError(t, retry.Retry(ctx, 500*time.Millisecond, func() *retry.RetryError {
			if err := e2e.ControllerRuntimeClient.Get(ctx, client.ObjectKey{
				Namespace: tenant.GetNamespace(),
				Name:      tenant.GetName(),
			}, tenant); err != nil {
				return retry.RetryPermanent(err)
			}

			for _, cond = range tenant.Status.Conditions {
				if cond.Type == relayv1beta1.TenantEventSinkReady && cond.Status == corev1.ConditionFalse {
					return retry.RetryPermanent(nil)
				}
			}

			return retry.RetryTransient(fmt.Errorf("waiting for tenant to reconcile"))
		}))
		assert.Equal(t, obj.TenantStatusReasonEventSinkNotConfigured, cond.Reason)
	})
}

func TestTenantAPITriggerEventSinkWithSecret(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	WithConfig(t, ctx, []ConfigOption{
		ConfigWithTenantReconciler,
	}, func(cfg *Config) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cfg.Namespace.GetName(),
				Name:      "my-test-tenant",
			},
			StringData: map[string]string{
				"token": "test",
			},
		}
		require.NoError(t, e2e.ControllerRuntimeClient.Create(ctx, secret))

		tenant := &relayv1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cfg.Namespace.GetName(),
				Name:      "my-test-tenant",
			},
			Spec: relayv1beta1.TenantSpec{
				TriggerEventSink: relayv1beta1.TriggerEventSink{
					API: &relayv1beta1.APITriggerEventSink{
						URL: "http://stub.example.com",
						TokenFrom: &relayv1beta1.APITokenSource{
							SecretKeyRef: &relayv1beta1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secret.GetName(),
								},
								Key: "token",
							},
						},
					},
				},
			},
		}
		require.NoError(t, e2e.ControllerRuntimeClient.Create(ctx, tenant))

		// Wait for tenant to reconcile.
		require.NoError(t, retry.Retry(ctx, 500*time.Millisecond, func() *retry.RetryError {
			if err := e2e.ControllerRuntimeClient.Get(ctx, client.ObjectKey{
				Namespace: tenant.GetNamespace(),
				Name:      tenant.GetName(),
			}, tenant); err != nil {
				return retry.RetryPermanent(err)
			}

			for _, cond := range tenant.Status.Conditions {
				if cond.Type == relayv1beta1.TenantReady && cond.Status == corev1.ConditionTrue {
					return retry.RetryPermanent(nil)
				}
			}

			return retry.RetryTransient(fmt.Errorf("waiting for tenant to reconcile"))
		}))
	})
}

func TestTenantAPITriggerEventSinkWithNamespaceAndSecret(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	WithConfig(t, ctx, []ConfigOption{
		ConfigWithTenantReconciler,
	}, func(cfg *Config) {
		child := fmt.Sprintf("%s-child", cfg.Namespace.GetName())

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cfg.Namespace.GetName(),
				Name:      "my-test-tenant",
			},
			StringData: map[string]string{
				"token": "test",
			},
		}
		require.NoError(t, e2e.ControllerRuntimeClient.Create(ctx, secret))

		tenant := &relayv1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cfg.Namespace.GetName(),
				Name:      "my-test-tenant",
			},
			Spec: relayv1beta1.TenantSpec{
				NamespaceTemplate: relayv1beta1.NamespaceTemplate{
					Metadata: metav1.ObjectMeta{
						Name: child,
					},
				},
				TriggerEventSink: relayv1beta1.TriggerEventSink{
					API: &relayv1beta1.APITriggerEventSink{
						URL: "http://stub.example.com",
						TokenFrom: &relayv1beta1.APITokenSource{
							SecretKeyRef: &relayv1beta1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secret.GetName(),
								},
								Key: "token",
							},
						},
					},
				},
			},
		}
		require.NoError(t, e2e.ControllerRuntimeClient.Create(ctx, tenant))

		// Wait for tenant to reconcile.
		require.NoError(t, retry.Retry(ctx, 500*time.Millisecond, func() *retry.RetryError {
			if err := e2e.ControllerRuntimeClient.Get(ctx, client.ObjectKey{
				Namespace: tenant.GetNamespace(),
				Name:      tenant.GetName(),
			}, tenant); err != nil {
				return retry.RetryPermanent(err)
			}

			for _, cond := range tenant.Status.Conditions {
				if cond.Type == relayv1beta1.TenantReady && cond.Status == corev1.ConditionTrue {
					return retry.RetryPermanent(nil)
				}
			}

			return retry.RetryTransient(fmt.Errorf("waiting for tenant to reconcile"))
		}))
	})
}
