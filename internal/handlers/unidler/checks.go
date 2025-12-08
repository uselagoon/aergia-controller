package unidler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

func (h *Unidler) checkForceScaled(ctx context.Context, ns string, opLog logr.Logger) bool {
	// get the deployments in the namespace if they have the `watch=true` label
	labelRequirements1, _ := labels.NewRequirement("idling.amazee.io/force-scaled", selection.Equals, []string{"true"})
	listOption := (&ctrlClient.ListOptions{}).ApplyOptions([]ctrlClient.ListOption{
		ctrlClient.InNamespace(ns),
		ctrlClient.MatchingLabelsSelector{
			Selector: labels.NewSelector().Add(*labelRequirements1),
		},
	})
	deployments := &appsv1.DeploymentList{}
	if err := h.Client.List(ctx, deployments, listOption); err != nil {
		opLog.Info(fmt.Sprintf("Unable to get any deployments - %s", ns))
		return false
	}
	if len(deployments.Items) > 0 {
		return true
	}
	return false
}

func (h *Unidler) hasRunningPod(ctx context.Context, namespace, deployment string) wait.ConditionWithContextFunc {
	return func(context.Context) (bool, error) {
		var d appsv1.Deployment
		if err := h.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: deployment}, &d); err != nil {
			return false, err
		}
		var pods corev1.PodList
		listOption := (&ctrlClient.ListOptions{}).ApplyOptions([]ctrlClient.ListOption{
			ctrlClient.MatchingLabelsSelector{
				Selector: labels.SelectorFromSet(d.Spec.Selector.MatchLabels),
			},
		})
		if err := h.Client.List(ctx, &pods, listOption); err != nil {
			return false, err
		}
		if len(pods.Items) == 0 {
			return false, nil
		}
		return pods.Items[0].Status.Phase == "Running", nil
	}
}

func (h *Unidler) removeCodeFromIngress(ctx context.Context, ns string, opLog logr.Logger) {
	// get the ingresses in the namespace
	listOption := (&ctrlClient.ListOptions{}).ApplyOptions([]ctrlClient.ListOption{
		ctrlClient.InNamespace(ns),
	})
	ingresses := &networkv1.IngressList{}
	if err := h.Client.List(ctx, ingresses, listOption); err != nil {
		opLog.Info(fmt.Sprintf("Unable to get any ingress - %s", ns))
		return
	}
	for _, ingress := range ingresses.Items {
		// if the nginx.ingress.kubernetes.io/custom-http-errors annotation is set
		// then strip out the 503 error code that is there so that
		// users will see their application errors rather than the loading page
		var ingressCodes, traefikMiddlewares *string
		patch := false
		ingressValue, ok := ingress.Annotations["nginx.ingress.kubernetes.io/custom-http-errors"]
		if ok {
			ingressCodes = removeStatusCode(ingressValue, "503")
			patch = true
		}
		traefikValue, ok := ingress.Annotations["traefik.ingress.kubernetes.io/router.middlewares"]
		if ok {
			traefikMiddlewares = removeStatusCode(traefikValue, fmt.Sprintf("%s-aergia@kubernetescrd", ingress.Namespace))
			patch = true
		}
		if patch {
			// if the 503 code was removed from the annotation
			// then patch it
			annotations := map[string]interface{}{
				"idling.amazee.io/idled-at": nil,
			}
			if ingressCodes == nil || *ingressCodes != ingressValue {
				annotations["nginx.ingress.kubernetes.io/custom-http-errors"] = ingressCodes
			}
			if traefikMiddlewares == nil || *traefikMiddlewares != traefikValue {
				annotations["traefik.ingress.kubernetes.io/router.middlewares"] = traefikMiddlewares
			}
			mergePatch, _ := json.Marshal(map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"idling.amazee.io/idled": "false",
					},
					"annotations": annotations,
				},
			})
			patchIngress := ingress.DeepCopy()
			if err := h.Client.Patch(ctx, patchIngress, ctrlClient.RawPatch(types.MergePatchType, mergePatch)); err != nil {
				// log it but try and patch the rest of the ingressses anyway (some is better than none?)
				opLog.Info(fmt.Sprintf("Error patching custom-http-errors on ingress %s - %s", ingress.Name, ns))
			} else {
				if ingressCodes == nil {
					opLog.Info(fmt.Sprintf("Ingress %s custom-http-errors annotation removed - %s", ingress.Name, ns))
				} else {
					opLog.Info(fmt.Sprintf("Ingress %s custom-http-errors annotation patched with %s - %s", ingress.Name, *ingressCodes, ns))
				}
			}
		}
	}
}
