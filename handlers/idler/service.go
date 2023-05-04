package idler

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups="",resources=services,verbs=list;get;watch;patch
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=list;get;watch;patch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=list;get;watch;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list;get;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=list;get;watch;patch
// +kubebuilder:rbac:groups=*,resources=ingresses,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=*,resources=ingress/status,verbs=get;update;patch

type serviceIdler struct {
	podInterval int
	esInterval  string
}

// ServiceIdler will run the Service idler process.
func (h *Idler) ServiceIdler() {
	ctx := context.Background()

	opLog := h.Log.WithName("aergia-controller").WithName("ServiceIdler")

	listOption := &client.ListOptions{}
	// in kubernetes, we can reliably check for the existence of this label so that
	// we only check namespaces that have been deployed by a lagoon at one point
	labelRequirements := generateLabelRequirements(h.Selectors.Service.Namespace)
	listOption = (&client.ListOptions{}).ApplyOptions([]client.ListOption{
		client.MatchingLabelsSelector{
			Selector: labels.NewSelector().Add(labelRequirements...),
		},
	})
	// get the namespaces in the cluster
	namespaces := &corev1.NamespaceList{}
	if err := h.Client.List(ctx, namespaces, listOption); err != nil {
		opLog.Info(fmt.Sprintf("unable to get any namespaces: %v", err))
		return
	}
	// loop over the namespaces
	for _, namespace := range namespaces.Items {

		projectAutoIdle, ok1 := namespace.ObjectMeta.Labels[h.Selectors.NamespaceSelectorsLabels.ProjectIdling]
		environmentAutoIdle, ok2 := namespace.ObjectMeta.Labels[h.Selectors.NamespaceSelectorsLabels.EnvironmentIdling]
		environmentType, ok3 := namespace.ObjectMeta.Labels[h.Selectors.NamespaceSelectorsLabels.EnvironmentType]
		if ok1 && ok2 && ok3 {
			if environmentType == "development" && environmentAutoIdle == "1" && projectAutoIdle == "1" {
				envOpLog := opLog.WithValues("namespace", namespace.ObjectMeta.Name).
					WithValues("project", namespace.ObjectMeta.Labels[h.Selectors.NamespaceSelectorsLabels.ProjectName]).
					WithValues("environment", namespace.ObjectMeta.Labels[h.Selectors.NamespaceSelectorsLabels.EnvironmentName]).
					WithValues("dry-run", h.DryRun)
				envOpLog.Info(fmt.Sprintf("Checking namespace"))
				h.KubernetesServiceIdler(ctx, envOpLog, namespace, namespace.ObjectMeta.Labels[h.Selectors.NamespaceSelectorsLabels.ProjectName], false, false)
			} else {
				if h.Debug {
					opLog.Info(fmt.Sprintf("skipping namespace %s; type is %s, autoidle values are env:%s proj:%s",
						namespace.ObjectMeta.Name,
						environmentType,
						environmentAutoIdle,
						projectAutoIdle))
				}
			}
		} else {
			if h.Debug {
				opLog.Info(fmt.Sprintf("skipping namespace %s; not in lagoon",
					namespace.ObjectMeta.Name))
			}
		}
	}
}
