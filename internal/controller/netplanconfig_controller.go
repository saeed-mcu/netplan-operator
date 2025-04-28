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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	networkv1 "github.com/saeed-mcu/netplan-operator/api/v1"

	netplanbin "github.com/saeed-mcu/netplan-operator/pkg/client"
	"github.com/saeed-mcu/netplan-operator/pkg/file"

	"github.com/saeed-mcu/netplan-operator/pkg/config"
)

const (
	finalizerName = "ae.digicloud/netplan"
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
	nodeName := os.Getenv("NODE_NAME")

	_, err := netplanbin.ExecuteCommand("netplan", "info")
	if err != nil {
		// logger.Error(err, "failed retrieving netplan info")
		// return ctrl.Result{}, err
	}

	// Write the network configuration to a file
	//logger.Info("NetplanPath", "NetplanPath", r.Config.NetplanPath)
	//filePath := filepath.Join(r.Config.NetplanPath, fmt.Sprintf("%s.yaml", req.Name))
	filePath := filepath.Join("/etc/netplan", fmt.Sprintf("%s.yaml", req.Name))
	//logger.Info("Start Reconcileing", "filePath", filePath)

	netConfig := &networkv1.NetplanConfig{}
	err = r.Get(ctx, req.NamespacedName, netConfig)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile req.
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the req.
		logger.Info("Error reading the object")
		return ctrl.Result{}, err
	}

	if netConfig.Spec.NodeName != nodeName {
		return ctrl.Result{}, nil
	}

	// --- Handle Deletion ---
	if !netConfig.ObjectMeta.DeletionTimestamp.IsZero() {
		// Resource is being deleted
		if controllerutil.ContainsFinalizer(netConfig, finalizerName) {
			if err := r.cleanupResource(ctx, netConfig, filePath); err != nil {
				return ctrl.Result{}, err
			}
			// Remove finalizer to allow deletion
			controllerutil.RemoveFinalizer(netConfig, finalizerName)
			if err := r.Update(ctx, netConfig); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	err = file.WriteConfigToFile(filePath, netConfig.Spec.NetworkConfig)
	if err != nil {
		logger.Error(err, "Failed to write network config to file", "path", filePath)
		netConfig.Status.State = err.Error()
		r.Status().Update(ctx, netConfig)
		return reconcile.Result{}, err
	}

	_, err = netplanbin.ExecuteCommand("netplan", "generate")
	if err != nil {

		netConfig.Status.Applied = "False"
		netConfig.Status.State = err.Error()

		meta.SetStatusCondition(&netConfig.Status.Conditions, metav1.Condition{
			Type:               "OperatorDegraded",
			Status:             metav1.ConditionTrue,
			Reason:             networkv1.ReasonOperandDeploymentFailed,
			LastTransitionTime: metav1.NewTime(time.Now()),
			Message:            "Operator Failed",
		})

		logger.Error(err, "Netplan generate error")
		r.Status().Update(ctx, netConfig)
		return reconcile.Result{}, nil

	} else {
		_, err = netplanbin.RunWithNsenter("netplan", "apply")
		if err != nil {

			netConfig.Status.Applied = "False"
			netConfig.Status.State = err.Error()

			meta.SetStatusCondition(&netConfig.Status.Conditions, metav1.Condition{
				Type:               "OperatorDegraded",
				Status:             metav1.ConditionTrue,
				Reason:             networkv1.ReasonOperandDeploymentFailed,
				LastTransitionTime: metav1.NewTime(time.Now()),
				Message:            "Operator Failed",
			})

			logger.Error(err, "Netplan Apply error")
			return reconcile.Result{}, nil
		}
	}

	meta.SetStatusCondition(&netConfig.Status.Conditions, metav1.Condition{
		Type:               "OperatorDegraded",
		Status:             metav1.ConditionTrue,
		Reason:             networkv1.ReasonSucceeded,
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            "Operator successfully reconciling",
	})

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(netConfig, finalizerName) {
		controllerutil.AddFinalizer(netConfig, finalizerName)
		if err := r.Update(ctx, netConfig); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	netConfig.Status.Applied = "True"
	netConfig.Status.State = networkv1.NoError
	r.Status().Update(ctx, netConfig)
	logger.Info("Apply Netplan Done !!!")
	return ctrl.Result{Requeue: false}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetplanConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1.NetplanConfig{}).
		Complete(r)
}

func (r *NetplanConfigReconciler) cleanupResource(ctx context.Context, netConfig *networkv1.NetplanConfig, filePath string) error {

	logger := log.FromContext(ctx)

	err := file.RemoveConfigFile(filePath)
	if err != nil {
		// TODO:
		logger.Error(err, "Error Delete File")
	}

	logger.Info("Cleanup Done")
	return nil
}
