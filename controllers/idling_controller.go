/*

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

package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/uselagoon/aergia-controller/handlers/idler"
	"github.com/uselagoon/aergia-controller/handlers/unidler"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// IdlingReconciler reconciles idling
type IdlingReconciler struct {
	client.Client
	Log     logr.Logger
	Scheme  *runtime.Scheme
	Idler   *idler.Idler
	Unidler *unidler.Unidler
}

func (r *IdlingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	opLog := r.Log.WithValues("idler", req.NamespacedName)

	var namespace corev1.Namespace
	if err := r.Get(ctx, req.NamespacedName, &namespace); err != nil {
		return ctrl.Result{}, ignoreNotFound(err)
	}

	if val, ok := namespace.ObjectMeta.Labels["idling.amazee.io/force-scaled"]; ok && val == "true" {
		opLog.Info(fmt.Sprintf("Force scaling environment %s", namespace.Name))
		r.Idler.KubernetesServiceIdler(ctx, opLog, namespace, namespace.ObjectMeta.Labels[r.Idler.Selectors.NamespaceSelectorsLabels.ProjectName], false, true)
		nsMergePatch, _ := json.Marshal(map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]*string{
					"idling.amazee.io/force-scaled": nil,
				},
			},
		})
		if err := r.Patch(ctx, &namespace, client.RawPatch(types.MergePatchType, nsMergePatch)); err != nil {
			// log it but try and scale the rest of the deployments anyway (some idled is better than none?)
			opLog.Info(fmt.Sprintf("Error patching namespace %s -%v", namespace.Name, err))
		}
		return ctrl.Result{}, nil
	}

	if val, ok := namespace.ObjectMeta.Labels["idling.amazee.io/force-idled"]; ok && val == "true" {
		opLog.Info(fmt.Sprintf("Force idling environment %s", namespace.Name))
		r.Idler.KubernetesServiceIdler(ctx, opLog, namespace, namespace.ObjectMeta.Labels[r.Idler.Selectors.NamespaceSelectorsLabels.ProjectName], true, false)
		nsMergePatch, _ := json.Marshal(map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]*string{
					"idling.amazee.io/force-idled": nil,
				},
			},
		})
		if err := r.Patch(ctx, &namespace, client.RawPatch(types.MergePatchType, nsMergePatch)); err != nil {
			// log it but try and scale the rest of the deployments anyway (some idled is better than none?)
			opLog.Info(fmt.Sprintf("Error patching namespace %s -%v", namespace.Name, err))
		}
		return ctrl.Result{}, nil
	}

	if val, ok := namespace.ObjectMeta.Labels["idling.amazee.io/unidle"]; ok && val == "true" {
		opLog.Info(fmt.Sprintf("Unidling environment %s", namespace.Name))
		r.Unidler.Unidle(ctx, &namespace, opLog)
		nsMergePatch, _ := json.Marshal(map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]*string{
					"idling.amazee.io/unidle": nil,
				},
			},
		})
		if err := r.Patch(ctx, &namespace, client.RawPatch(types.MergePatchType, nsMergePatch)); err != nil {
			// log it but try and scale the rest of the deployments anyway (some idled is better than none?)
			opLog.Info(fmt.Sprintf("Error patching namespace %s -%v", namespace.Name, err))
		}
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the watch on the namespace resource with an event filter (see predicates.go)
func (r *IdlingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		WithEventFilter(NamespacePredicates{}).
		Complete(r)
}

// will ignore not found errors
func ignoreNotFound(err error) error {
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
