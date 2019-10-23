/*
Copyright (c) Puppet, Inc.
*/

// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	internalinterfaces "github.com/puppetlabs/nebula-tasks/pkg/generated/informers/externalversions/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// SecretAuths returns a SecretAuthInformer.
	SecretAuths() SecretAuthInformer
	// WorkflowRuns returns a WorkflowRunInformer.
	WorkflowRuns() WorkflowRunInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

// SecretAuths returns a SecretAuthInformer.
func (v *version) SecretAuths() SecretAuthInformer {
	return &secretAuthInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// WorkflowRuns returns a WorkflowRunInformer.
func (v *version) WorkflowRuns() WorkflowRunInformer {
	return &workflowRunInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}
