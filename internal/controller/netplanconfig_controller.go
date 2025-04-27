/*
Copyright 2025.

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

package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"

	"github.com/pkg/errors"
	rbac "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/saeed-mcu/netplan-operator/api/names"
	networkv1 "github.com/saeed-mcu/netplan-operator/api/v1"
	netplanbin "github.com/saeed-mcu/netplan-operator/pkg/client"
	nmstaterenderer "github.com/saeed-mcu/netplan-operator/pkg/render"
	corev1 "k8s.io/api/core/v1"

	"github.com/openshift/cluster-network-operator/pkg/render"
	"github.com/saeed-mcu/netplan-operator/pkg/config"
)

// NetplanConfigReconciler reconciles a NetplanConfig object
type NetplanConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *config.Config
}

// +kubebuilder:rbac:groups=network.netplan.io,resources=netplanconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.netplan.io,resources=netplanconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=network.netplan.io,resources=netplanconfigs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NetplanConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *NetplanConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	logger := log.FromContext(ctx)
	//nodeName := os.Getenv("NODE_NAME")

	_, err := netplanbin.ExecuteCommand("netplan", "info")
	if err != nil {
		// logger.Error(err, "failed retrieving netplan info")
		// return ctrl.Result{}, err
	}

	// Fetch the NMState instance
	instanceList := &networkv1.NetplanConfigList{}
	err = r.List(context.TODO(), instanceList, &client.ListOptions{})
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed listing all NMState instances")
	}

	instance := &networkv1.NetplanConfig{}
	err = r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile req.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			logger.Info("Request object not found")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the req.
		logger.Info("Error reading the object")
		return ctrl.Result{}, err
	}

	if err := r.applyManifests(instance, ctx); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Reconcile complete.")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetplanConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1.NetplanConfig{}).
		Complete(r)
}

func (r *NetplanConfigReconciler) applyManifests(instance *networkv1.NetplanConfig, ctx context.Context) error {
	if err := r.applyCRDs(instance); err != nil {
		errors.Wrap(err, "failed applying CRDs")
		return err
	}

	if err := r.applyNamespace(instance); err != nil {
		errors.Wrap(err, "failed applying Namespace")
		return err
	}

	if err := r.applyRBAC(instance); err != nil {
		errors.Wrap(err, "failed applying RBAC")
		return err
	}

	if err := r.applyHandler(instance); err != nil {
		errors.Wrap(err, "failed applying Handler")
		return err
	}

	return nil
}

func (r *NetplanConfigReconciler) applyCRDs(instance *networkv1.NetplanConfig) error {
	data := render.MakeRenderData()
	return r.renderAndApply(instance, data, "crds", false)
}

func (r *NetplanConfigReconciler) applyNamespace(instance *networkv1.NetplanConfig) error {
	data := render.MakeRenderData()
	data.Data["HandlerNamespace"] = os.Getenv("HANDLER_NAMESPACE")
	data.Data["HandlerPrefix"] = os.Getenv("HANDLER_PREFIX")
	return r.renderAndApply(instance, data, "namespace", false)
}

func (r *NetplanConfigReconciler) applyRBAC(instance *networkv1.NetplanConfig) error {
	data := render.MakeRenderData()
	data.Data["HandlerNamespace"] = os.Getenv("HANDLER_NAMESPACE")
	data.Data["HandlerImage"] = os.Getenv("RELATED_IMAGE_HANDLER_IMAGE")
	data.Data["HandlerPullPolicy"] = os.Getenv("HANDLER_IMAGE_PULL_POLICY")
	data.Data["HandlerPrefix"] = os.Getenv("HANDLER_PREFIX")

	if err := setClusterReaderExist(r.Client, data); err != nil {
		return errors.Wrap(err, "failed checking if cluster-reader ClusterRole exists")
	}

	return r.renderAndApply(instance, data, "rbac", true)
}

// nolint: funlen
func (r *NetplanConfigReconciler) applyHandler(instance *networkv1.NetplanConfig) error {
	data := render.MakeRenderData()
	// Register ToYaml template method
	data.Funcs["toYaml"] = nmstaterenderer.ToYaml
	// Prepare defaults
	masterExistsNoScheduleTolerations := []corev1.Toleration{
		{
			Key:      "node-role.kubernetes.io/master",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
		{
			Key:      "node-role.kubernetes.io/control-plane",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}
	operatorExistsToleration := corev1.Toleration{
		Key:      "",
		Operator: corev1.TolerationOpExists,
	}
	archNodeSelector := map[string]string{
		"kubernetes.io/arch": goruntime.GOARCH,
	}
	archAndCRNodeSelector := instance.Spec.NodeSelector
	if archAndCRNodeSelector == nil {
		archAndCRNodeSelector = map[string]string{
			"kubernetes.io/arch": goruntime.GOARCH,
			"kubernetes.io/os":   "linux",
		}
	}
	handlerTolerations := instance.Spec.Tolerations
	if handlerTolerations == nil {
		handlerTolerations = []corev1.Toleration{operatorExistsToleration}
	}
	handlerAffinity := instance.Spec.Affinity
	if handlerAffinity == nil {
		handlerAffinity = &corev1.Affinity{}
	}

	archAndCRInfraNodeSelector := instance.Spec.InfraNodeSelector
	if archAndCRInfraNodeSelector == nil {
		archAndCRInfraNodeSelector = archNodeSelector
	} else {
		archAndCRInfraNodeSelector["kubernetes.io/arch"] = goruntime.GOARCH
	}

	infraTolerations := instance.Spec.InfraTolerations
	if infraTolerations == nil {
		infraTolerations = masterExistsNoScheduleTolerations
	}

	infraAffinity := instance.Spec.InfraAffinity
	if infraAffinity == nil {
		infraAffinity = &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
					{
						Weight: 10,
						Preference: corev1.NodeSelectorTerm{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "node-role.kubernetes.io/control-plane",
									Operator: corev1.NodeSelectorOpExists,
								},
							},
						},
					},
					{
						Weight: 1,
						Preference: corev1.NodeSelectorTerm{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "node-role.kubernetes.io/master",
									Operator: corev1.NodeSelectorOpExists,
								},
							},
						},
					},
				},
			},
		}
	}

	data.Data["HandlerNamespace"] = os.Getenv("HANDLER_NAMESPACE")
	data.Data["HandlerImage"] = os.Getenv("RELATED_IMAGE_HANDLER_IMAGE")
	data.Data["HandlerPullPolicy"] = os.Getenv("HANDLER_IMAGE_PULL_POLICY")
	data.Data["HandlerPrefix"] = os.Getenv("HANDLER_PREFIX")
	data.Data["MonitoringNamespace"] = os.Getenv("MONITORING_NAMESPACE")
	data.Data["KubeRBACProxyImage"] = os.Getenv("KUBE_RBAC_PROXY_IMAGE")
	data.Data["InfraNodeSelector"] = archAndCRInfraNodeSelector
	data.Data["InfraTolerations"] = infraTolerations
	data.Data["WebhookAffinity"] = infraAffinity
	data.Data["HandlerNodeSelector"] = archAndCRNodeSelector
	data.Data["HandlerTolerations"] = handlerTolerations
	data.Data["HandlerAffinity"] = handlerAffinity

	return r.renderAndApply(instance, data, "handler", true)
}

func (r *NetplanConfigReconciler) renderAndApply(
	instance *networkv1.NetplanConfig,
	data render.RenderData,
	sourceDirectory string,
	setControllerReference bool,
) error {
	var err error

	sourceFullDirectory := filepath.Join(names.ManifestDir, "kubernetes-nmstate", sourceDirectory)
	objs, err := render.RenderDir(sourceFullDirectory, &data)
	if err != nil {
		return errors.Wrapf(err, "failed to render kubernetes-nmstate %s", sourceDirectory)
	}

	// If no file found in directory - return error
	if len(objs) == 0 {
		return fmt.Errorf("no manifests rendered from %s", sourceFullDirectory)
	}

	for _, obj := range objs {
		// RenderDir seems to add an extra null entry to the list. It appears to be because of the
		// nested templates. This just makes sure we don't try to apply an empty obj.
		if obj.GetName() == "" {
			continue
		}
		if setControllerReference {
			// Set the controller reference. When the CR is removed, it will remove the CRDs as well
			err = controllerutil.SetControllerReference(instance, obj, r.Scheme)
			if err != nil {
				return errors.Wrap(err, "failed to set owner reference")
			}
		}
		if err := r.apply(context.TODO(), obj); err != nil {
			return fmt.Errorf("failed to apply object %v: %w", obj, err)
		}
	}
	return nil
}

func (r *NetplanConfigReconciler) apply(ctx context.Context, newObj *unstructured.Unstructured) error {
	key := client.ObjectKeyFromObject(newObj)

	oldObj := &unstructured.Unstructured{}
	oldObj.SetGroupVersionKind(newObj.GroupVersionKind())
	if err := r.Client.Get(ctx, key, oldObj); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err := r.Client.Create(ctx, newObj); err != nil {
			return fmt.Errorf("failed creating %q \"%s:%s: %w", newObj.GetKind(), newObj.GetNamespace(), newObj.GetName(), err)
		}
		return nil
	}

	newObj.SetResourceVersion(oldObj.GetResourceVersion())
	if err := r.Client.Patch(ctx, newObj, client.MergeFrom(oldObj)); err != nil {
		return fmt.Errorf("failed patching %q \"%s:%s: %w", newObj.GetKind(), newObj.GetNamespace(), newObj.GetName(), err)
	}

	return nil
}

func setClusterReaderExist(c client.Client, data render.RenderData) error {
	var clusterReader rbac.ClusterRole
	key := types.NamespacedName{Name: "cluster-reader"}
	err := c.Get(context.TODO(), key, &clusterReader)

	found := true
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		found = false
	}

	data.Data["ClusterReaderExists"] = found
	return nil
}
