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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
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

	if val, ok := namespace.ObjectMeta.Labels["idling.lagoon.sh/force-scaled"]; ok && val == "true" {
		opLog.Info(fmt.Sprintf("Force scaling environment %s", namespace.Name))
		r.Idler.KubernetesServiceIdler(ctx, opLog, namespace, namespace.ObjectMeta.Labels[r.Idler.Selectors.NamespaceSelectorsLabels.ProjectName], false, true)
		nsMergePatch, _ := json.Marshal(map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]*string{
					"idling.lagoon.sh/force-scaled": nil,
				},
			},
		})
		if err := r.Patch(ctx, &namespace, client.RawPatch(types.MergePatchType, nsMergePatch)); err != nil {
			// log it but try and scale the rest of the deployments anyway (some idled is better than none?)
			opLog.Info(fmt.Sprintf("Error patching namespace %s -%v", namespace.Name, err))
		}
		return ctrl.Result{}, nil
	}

	if val, ok := namespace.ObjectMeta.Labels["idling.lagoon.sh/force-idled"]; ok && val == "true" {
		opLog.Info(fmt.Sprintf("Force idling environment %s", namespace.Name))
		r.Idler.KubernetesServiceIdler(ctx, opLog, namespace, namespace.ObjectMeta.Labels[r.Idler.Selectors.NamespaceSelectorsLabels.ProjectName], true, false)
		nsMergePatch, _ := json.Marshal(map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]*string{
					"idling.lagoon.sh/force-idled": nil,
				},
			},
		})
		if err := r.Patch(ctx, &namespace, client.RawPatch(types.MergePatchType, nsMergePatch)); err != nil {
			// log it but try and scale the rest of the deployments anyway (some idled is better than none?)
			opLog.Info(fmt.Sprintf("Error patching namespace %s -%v", namespace.Name, err))
		}
		return ctrl.Result{}, nil
	}

	if val, ok := namespace.ObjectMeta.Labels["idling.lagoon.sh/unidle"]; ok && val == "true" {
		opLog.Info(fmt.Sprintf("Unidling environment %s", namespace.Name))
		r.Unidler.Unidle(ctx, &namespace, opLog)
		nsMergePatch, _ := json.Marshal(map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]*string{
					"idling.lagoon.sh/unidle": nil,
				},
			},
		})
		if err := r.Patch(ctx, &namespace, client.RawPatch(types.MergePatchType, nsMergePatch)); err != nil {
			// log it but try and scale the rest of the deployments anyway (some idled is better than none?)
			opLog.Info(fmt.Sprintf("Error patching namespace %s -%v", namespace.Name, err))
		}
		return ctrl.Result{}, nil
	}

	/*
		convert old labels or annotations
	*/
	conversions := false
	// if the converted-old-labels label is not set or not true, then run the conversion
	if val, ok := namespace.ObjectMeta.Labels["idling.lagoon.sh/converted-old-labels"]; !ok || val != "true" {
		// ingress
		labelRequirements, _ := labels.NewRequirement("idling.amazee.io/idled", selection.Exists, []string{})
		listOption := (&client.ListOptions{}).ApplyOptions([]client.ListOption{
			client.InNamespace(namespace.Name),
			client.MatchingLabelsSelector{
				Selector: labels.NewSelector().Add(*labelRequirements),
			},
		})
		ingressList := &networkv1.IngressList{}
		if err := r.List(ctx, ingressList, listOption); err != nil {
			opLog.Error(err, fmt.Sprintf("Error getting ingress for namespace %s", namespace.ObjectMeta.Name))
		} else {
			for _, ingress := range ingressList.Items {
				ingressPatchAnnotations := map[string]interface{}{}
				ingressMergePatch, _ := json.Marshal(map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": ingressPatchAnnotations,
					},
				})
				if val, ok := ingress.ObjectMeta.Annotations["idling.amazee.io/allowed-agents"]; ok {
					ingressPatchAnnotations["idling.lagoon.sh/allowed-agents"] = val
					ingressPatchAnnotations["idling.amazee.io/allowed-agents"] = nil
				}
				if val, ok := ingress.ObjectMeta.Annotations["idling.amazee.io/blocked-agents"]; ok {
					ingressPatchAnnotations["idling.lagoon.sh/blocked-agents"] = val
					ingressPatchAnnotations["idling.amazee.io/blocked-agents"] = nil
				}
				if val, ok := ingress.ObjectMeta.Annotations["idling.amazee.io/ip-allow-agents"]; ok {
					ingressPatchAnnotations["idling.lagoon.sh/ip-allow-agents"] = val
					ingressPatchAnnotations["idling.amazee.io/ip-allow-agents"] = nil
				}
				if val, ok := ingress.ObjectMeta.Annotations["idling.amazee.io/ip-block-agents"]; ok {
					ingressPatchAnnotations["idling.lagoon.sh/ip-block-agents"] = val
					ingressPatchAnnotations["idling.amazee.io/ip-block-agents"] = nil
				}
				if len(ingressPatchAnnotations) > 0 {
					conversions = true
					patchIngress := ingress.DeepCopy()
					if err := r.Patch(ctx, patchIngress, client.RawPatch(types.MergePatchType, ingressMergePatch)); err != nil {
						// log it but try and scale the rest of the deployments anyway (some idled is better than none?)
						opLog.Info(fmt.Sprintf("Error patching ingress %s -%v", patchIngress.Name, err))
					}
				}
			}
		}

		// deployments
		labelRequirements1, _ := labels.NewRequirement("idling.amazee.io/watch", selection.Exists, []string{})
		listOption = (&client.ListOptions{}).ApplyOptions([]client.ListOption{
			client.InNamespace(namespace.Name),
			client.MatchingLabelsSelector{
				Selector: labels.NewSelector().Add(*labelRequirements1),
			},
		})
		deployments := &appsv1.DeploymentList{}
		if err := r.List(ctx, deployments, listOption); err != nil {
			opLog.Error(err, fmt.Sprintf("Error getting deployments for namespace %s", namespace.ObjectMeta.Name))
		} else {
			for _, deployment := range deployments.Items {
				deploymentPatchAnnotations := map[string]interface{}{}
				deploymentPatchLabels := map[string]interface{}{}
				deploymentMergePatch, _ := json.Marshal(map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels":      deploymentPatchLabels,
						"annotations": deploymentPatchAnnotations,
					},
				})
				if val, ok := deployment.ObjectMeta.Labels["idling.amazee.io/watch"]; ok {
					deploymentPatchLabels["idling.lagoon.sh/watch"] = val
					deploymentPatchLabels["idling.amazee.io/watch"] = nil
				}
				if _, ok := deployment.ObjectMeta.Annotations["idling.amazee.io/idled"]; ok {
					deploymentPatchAnnotations["idling.lagoon.sh/idled"] = nil
					deploymentPatchAnnotations["idling.amazee.io/idled"] = nil
				}
				if val, ok := deployment.ObjectMeta.Labels["idling.amazee.io/idled"]; ok {
					deploymentPatchLabels["idling.lagoon.sh/idled"] = val
					deploymentPatchLabels["idling.amazee.io/idled"] = nil
				}
				if val, ok := deployment.ObjectMeta.Labels["idling.amazee.io/force-scaled"]; ok {
					deploymentPatchLabels["idling.lagoon.sh/force-scaled"] = val
					deploymentPatchLabels["idling.amazee.io/force-scaled"] = nil
				}
				if val, ok := deployment.ObjectMeta.Labels["idling.amazee.io/force-idled"]; ok {
					deploymentPatchLabels["idling.lagoon.sh/force-idled"] = val
					deploymentPatchLabels["idling.amazee.io/force-idled"] = nil
				}
				if val, ok := deployment.ObjectMeta.Annotations["idling.amazee.io/idled-at"]; ok {
					deploymentPatchAnnotations["idling.lagoon.sh/idled-at"] = val
					deploymentPatchAnnotations["idling.amazee.io/idled-at"] = nil
				}
				if val, ok := deployment.ObjectMeta.Annotations["idling.amazee.io/unidle-replicas"]; ok {
					deploymentPatchAnnotations["idling.lagoon.sh/unidle-replicas"] = val
					deploymentPatchAnnotations["idling.amazee.io/unidle-replicas"] = nil
				}
				if len(deploymentPatchAnnotations) > 0 || len(deploymentPatchLabels) > 0 {
					conversions = true
					patchDeployment := deployment.DeepCopy()
					if err := r.Patch(ctx, patchDeployment, client.RawPatch(types.MergePatchType, deploymentMergePatch)); err != nil {
						// log it but try and scale the rest of the deployments anyway (some idled is better than none?)
						opLog.Info(fmt.Sprintf("Error patching deployment %s -%v", patchDeployment.Name, err))
					}
				}
			}
		}
	}
	// namespace labels and anotations
	nsPatchLabels := map[string]interface{}{}
	nsPatchAnnotations := map[string]interface{}{}
	nsMergePatch, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels":      nsPatchLabels,
			"annotations": nsPatchAnnotations,
		},
	})
	if val, ok := namespace.ObjectMeta.Annotations["idling.amazee.io/disable-request-verification"]; ok {
		nsPatchAnnotations["idling.lagoon.sh/disable-request-verification"] = val
		nsPatchAnnotations["idling.amazee.io/disable-request-verification"] = nil
	}
	if val, ok := namespace.ObjectMeta.Annotations["idling.amazee.io/allowed-agents"]; ok {
		nsPatchAnnotations["idling.lagoon.sh/allowed-agents"] = val
		nsPatchAnnotations["idling.amazee.io/allowed-agents"] = nil
	}
	if val, ok := namespace.ObjectMeta.Annotations["idling.amazee.io/blocked-agents"]; ok {
		nsPatchAnnotations["idling.lagoon.sh/blocked-agents"] = val
		nsPatchAnnotations["idling.amazee.io/blocked-agents"] = nil
	}
	if val, ok := namespace.ObjectMeta.Annotations["idling.amazee.io/ip-allow-agents"]; ok {
		nsPatchAnnotations["idling.lagoon.sh/ip-allow-agents"] = val
		nsPatchAnnotations["idling.amazee.io/ip-allow-agents"] = nil
	}
	if val, ok := namespace.ObjectMeta.Annotations["idling.amazee.io/ip-block-agents"]; ok {
		nsPatchAnnotations["idling.lagoon.sh/ip-block-agents"] = val
		nsPatchAnnotations["idling.amazee.io/ip-block-agents"] = nil
	}
	if val, ok := namespace.ObjectMeta.Annotations["idling.amazee.io/prometheus-interval"]; ok {
		nsPatchAnnotations["idling.lagoon.sh/prometheus-interval"] = val
		nsPatchAnnotations["idling.amazee.io/prometheus-interval"] = nil
	}
	if val, ok := namespace.ObjectMeta.Annotations["idling.amazee.io/pod-interval"]; ok {
		nsPatchAnnotations["idling.lagoon.sh/pod-interval"] = val
		nsPatchAnnotations["idling.amazee.io/pod-interval"] = nil
	}
	if conversions {
		// if any conversions took place, set the label to prevent further conversions taking place
		nsPatchLabels["idling.lagoon.sh/converted-old-labels"] = true
	}
	if len(nsPatchLabels) > 0 || len(nsPatchAnnotations) > 0 {
		if err := r.Patch(ctx, &namespace, client.RawPatch(types.MergePatchType, nsMergePatch)); err != nil {
			// log it but try and scale the rest of the deployments anyway (some idled is better than none?)
			opLog.Info(fmt.Sprintf("Error patching namespace %s -%v", namespace.Name, err))
		}
	}
	/*
		convert old labels
	*/
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
