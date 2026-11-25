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
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/uselagoon/aergia-controller/internal/handlers/idler"
	"github.com/uselagoon/aergia-controller/internal/handlers/unidler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// ControlConfigMapName is the name of the configmap used to request idling actions from within a namespace
const ControlConfigMapName = "aergia-control"

// ControlLabel is the label that marks the control configmap so that it is watched
const ControlLabel = "idling.lagoon.sh/control"

// +kubebuilder:rbac:groups="",resources=configmaps,verbs=create;get;list;watch;patch

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

	var controlConfigMap *corev1.ConfigMap
	// provisioning errors are returned after any requested actions are handled, so a failure here never blocks an action
	var provisionErr error
	if val, ok := namespace.Labels["idling.amazee.io/idled"]; ok && val == "true" && namespace.DeletionTimestamp.IsZero() {
		cm, err := r.ensureControlConfigMap(ctx, namespace.Name)
		if err != nil {
			if isPermanentProvisionError(err) {
				opLog.Info(fmt.Sprintf("Not retrying creation of control configmap in namespace %s - %v", namespace.Name, err))
			} else {
				opLog.Info(fmt.Sprintf("Error creating control configmap in namespace %s, will retry - %v", namespace.Name, err))
				provisionErr = err
			}
		}
		controlConfigMap = cm
	}
	if controlConfigMap == nil {
		var cm corev1.ConfigMap
		if err := r.Get(ctx, types.NamespacedName{Namespace: namespace.Name, Name: ControlConfigMapName}, &cm); err == nil {
			controlConfigMap = &cm
		} else if !apierrors.IsNotFound(err) {
			// a failed read must not be treated as "no requests", otherwise a pending request is lost until the next event
			provisionErr = errors.Join(provisionErr, fmt.Errorf("error reading control configmap in namespace %s: %w", namespace.Name, err))
		}
	}

	if controlConfigMap != nil && controlConfigMap.Labels[ControlLabel] != "true" {
		opLog.Info(fmt.Sprintf("Ignoring configmap %s in namespace %s, it is missing the %s=true label", ControlConfigMapName, namespace.Name, ControlLabel))
		controlConfigMap = nil
	}

	// namespace labels take priority over the configmap, and both are cleared once the action is performed
	actions := []string{"force-scaled", "force-idled", "unidle"}
	action, inNamespace, inConfigMap := "", false, false
	for _, candidate := range actions {
		if namespace.Labels["idling.amazee.io/"+candidate] == "true" {
			action, inNamespace = candidate, true
			break
		}
	}
	if action == "" && controlConfigMap != nil {
		for _, candidate := range actions {
			if controlConfigMap.Data[candidate] == "true" {
				action = candidate
				break
			}
		}
	}
	// failing to clear a one-shot request is returned so that it is retried, otherwise it would replay on later updates
	var clearErr error
	if action != "" {
		key := "idling.amazee.io/" + action
		inConfigMap = controlConfigMap != nil && controlConfigMap.Data[action] == "true"
		var actionErr error
		projectName := namespace.Labels[r.Idler.Selectors.NamespaceSelectorsLabels.ProjectName]
		switch action {
		case "force-scaled":
			opLog.Info(fmt.Sprintf("Force scaling environment %s", namespace.Name))
			actionErr = r.Idler.KubernetesServiceIdler(ctx, opLog, namespace, projectName, false, true)
		case "force-idled":
			opLog.Info(fmt.Sprintf("Force idling environment %s", namespace.Name))
			actionErr = r.Idler.KubernetesServiceIdler(ctx, opLog, namespace, projectName, true, false)
		case "unidle":
			opLog.Info(fmt.Sprintf("Unidling environment %s", namespace.Name))
			actionErr = r.Unidler.Unidle(ctx, &namespace, opLog)
		}
		if actionErr != nil {
			// leave the request in place so that it is retried, clearing it would silently drop the action
			opLog.Info(fmt.Sprintf("Error performing %s for environment %s, will retry - %v", action, namespace.Name, actionErr))
			return ctrl.Result{}, errors.Join(actionErr, provisionErr)
		}
		if inNamespace {
			nsMergePatch, _ := json.Marshal(map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]*string{key: nil},
				},
			})
			if err := r.Patch(ctx, &namespace, client.RawPatch(types.MergePatchType, nsMergePatch)); err != nil {
				opLog.Info(fmt.Sprintf("Error clearing %s from namespace %s, will retry - %v", key, namespace.Name, err))
				if !apierrors.IsNotFound(err) {
					clearErr = errors.Join(clearErr, err)
				}
			}
		}
		if inConfigMap {
			cmMergePatch, _ := json.Marshal(map[string]interface{}{
				"data": map[string]*string{action: nil},
			})
			if err := r.Patch(ctx, controlConfigMap, client.RawPatch(types.MergePatchType, cmMergePatch)); err != nil {
				opLog.Info(fmt.Sprintf("Error clearing %s from configmap %s in namespace %s, will retry - %v", action, ControlConfigMapName, namespace.Name, err))
				if !apierrors.IsNotFound(err) {
					clearErr = errors.Join(clearErr, err)
				}
			}
		}
	}
	return ctrl.Result{}, errors.Join(clearErr, provisionErr)
}

// isPermanentProvisionError returns true for errors where retrying will not help, such as missing permissions,
// an invalid object, or a namespace that is being terminated.
func isPermanentProvisionError(err error) bool {
	return apierrors.IsForbidden(err) ||
		apierrors.IsInvalid(err) ||
		apierrors.HasStatusCause(err, corev1.NamespaceTerminatingCause)
}

// ensureControlConfigMap creates an empty control configmap in an idled namespace if it doesn't already exist.
func (r *IdlingReconciler) ensureControlConfigMap(ctx context.Context, namespace string) (*corev1.ConfigMap, error) {
	var existing corev1.ConfigMap
	err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: ControlConfigMapName}, &existing)
	if err == nil {
		return &existing, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ControlConfigMapName,
			Namespace: namespace,
			Labels: map[string]string{
				ControlLabel: "true",
			},
		},
	}
	if err := r.Create(ctx, configMap); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil, nil
		}
		return nil, err
	}
	return configMap, nil
}

// SetupWithManager sets up the watch on the namespace resource with an event filter (see predicates.go)
func (r *IdlingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetNamespace()}}}
			}),
			builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
				return obj.GetName() == ControlConfigMapName && obj.GetLabels()[ControlLabel] == "true"
			})),
		).
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
