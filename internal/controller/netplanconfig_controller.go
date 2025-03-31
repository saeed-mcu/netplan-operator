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
	"path/filepath"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	networkv1 "github.com/saeed-mcu/netplan-operator/api/v1"

	netplanbin "github.com/saeed-mcu/netplan-operator/pkg/client"
	"github.com/saeed-mcu/netplan-operator/pkg/file"

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

	_, err := netplanbin.ExecuteCommand("netplan", "info")
	if err != nil {
		logger.Info("failed retrieving netplan info")
		//logger.Error(err, "failed retrieving netplan info")
		//return ctrl.Result{}, err
	}

	// Write the network configuration to a file
	//logger.Info("NetplanPath", "NetplanPath", r.Config.NetplanPath)
	//filePath := filepath.Join(r.Config.NetplanPath, fmt.Sprintf("%s.yaml", req.Name))
	filePath := filepath.Join("/etc/netplan", fmt.Sprintf("%s.yaml", req.Name))
	logger.Info("Start Reconcileing", "filePath", filePath)

	netConfig := &networkv1.NetplanConfig{}
	err = r.Get(ctx, req.NamespacedName, netConfig)
	if err != nil && errors.IsNotFound(err) {
		err = file.RemoveConfigFile(filePath)
		if err != nil {
			// TODO:
			logger.Error(err, "Error Delete File")
		} else {
			_, err = netplanbin.RunWithNsenter("netplan", "apply")
			if err != nil {
				logger.Error(err, "Netplan Apply error")
				return reconcile.Result{}, err
			}
			logger.Info("Netplan cleanup done")
		}
		return ctrl.Result{}, nil
	} else if err != nil {
		logger.Error(err, "Error getting operator CE object")
		return ctrl.Result{}, err
	}

	err = file.WriteConfigToFile(filePath, netConfig.Spec.NetworkConfig)
	if err != nil {
		logger.Error(err, "Failed to write network config to file", "path", filePath)
		netConfig.Status.Error = err.Error()
		netConfig.Status.Applied = false
		r.Status().Update(ctx, netConfig)
		return reconcile.Result{}, err
	}

	_, err = netplanbin.ExecuteCommand("netplan", "generate")
	if err != nil {
		logger.Error(err, "Netplan Config error")
		netConfig.Status.Error = err.Error()
		netConfig.Status.Applied = false
		r.Status().Update(ctx, netConfig)
		return reconcile.Result{}, err
	} else {
		_, err = netplanbin.RunWithNsenter("netplan", "apply")
		if err != nil {
			logger.Error(err, "Netplan Apply error")
			return reconcile.Result{}, err
		}
	}

	logger.Info("Network configuration applied successfully", "file", filePath)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetplanConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1.NetplanConfig{}).
		Complete(r)
}
