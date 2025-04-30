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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/saeed-mcu/netplan-operator/api/shared"
	networkv1 "github.com/saeed-mcu/netplan-operator/api/v1"

	netplanbin "github.com/saeed-mcu/netplan-operator/pkg/client"
	"github.com/saeed-mcu/netplan-operator/pkg/file"
	"github.com/saeed-mcu/netplan-operator/pkg/nmstatectl"

	"github.com/saeed-mcu/netplan-operator/pkg/config"
)

const (
	finalizerName = "ae.digicloud/netplan"

	defaultGwProbeTimeout = 120 * time.Second
	apiServerProbeTimeout = 120 * time.Second
	// DesiredStateConfigurationTimeout doubles the default gw ping probe and API server
	// connectivity check timeout to ensure the Checkpoint is alive before rolling it back
	// https://nmstate.github.io/cli_guide#manual-transaction-control
	DesiredStateConfigurationTimeout = (defaultGwProbeTimeout + apiServerProbeTimeout) * 2
)

// NetplanConfigReconciler reconciles a NetplanConfig object
type NetplanConfigReconciler struct {
	client.Client
	APIClient client.Client
	Scheme    *runtime.Scheme
	Config    *config.Config
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

	_, err := netplanbin.ExecuteCommand("nmstatectl", "version")
	if err != nil {
		logger.Error(err, "failed retrieving nmstate version")
		return ctrl.Result{}, err
	}

	netConfig := &networkv1.NetplanConfig{}
	err = r.Get(ctx, req.NamespacedName, netConfig)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if netConfig.Spec.NodeName != nodeName {
		return ctrl.Result{}, nil
	}

	// --- Handle Deletion ---
	if !netConfig.ObjectMeta.DeletionTimestamp.IsZero() {
		// Resource is being deleted
		if controllerutil.ContainsFinalizer(netConfig, finalizerName) {
			// if err := r.cleanupResource(ctx, netConfig, filePath); err != nil {
			// 	return ctrl.Result{}, err
			// }

			// Remove finalizer to allow deletion
			controllerutil.RemoveFinalizer(netConfig, finalizerName)
			if err := r.Update(ctx, netConfig); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	_, err = netplanbin.ExecuteCommand("nmstatectl", "version")
	if err != nil {
		logger.Error(err, "failed retrieving nmstate version")
		return ctrl.Result{}, err
	}

	desiredState := shared.NewState(netConfig.Spec.NetworkConfig)
	logger.Info("desiredState.raw", "raw", desiredState.Raw)
	_, err = ApplyDesiredState(r.APIClient, desiredState)

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
	logger.Info("Apply nmstate Done !!!")
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

func ApplyDesiredState(cli client.Client, desiredState shared.State) (string, error) {
	if string(desiredState.Raw) == "" {
		return "Ignoring empty desired state", nil
	}

	// Before apply we get the probes that are working fine, they should be
	// working fine after apply
	//probes := probe.Select(cli)

	// Rollback before Apply to remove pending checkpoints (for example handler pod restarted
	// before Commit)
	nmstatectl.Rollback()

	setOutput, err := nmstatectl.Set(desiredState, DesiredStateConfigurationTimeout)
	if err != nil {
		return setOutput, err
	}

	//err = probe.Run(cli, probes)
	//if err != nil {
	//	return "", rollback(cli, probes, errors.Wrap(err, "failed runnig probes after network changes"))
	//}

	commitOutput, err := nmstatectl.Commit()
	if err != nil {
		// We cannot rollback if commit fails, just return the error
		return commitOutput, err
	}

	commandOutput := fmt.Sprintf("setOutput: %s \n", setOutput)
	return commandOutput, nil
}
