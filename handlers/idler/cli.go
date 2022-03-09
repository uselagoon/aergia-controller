package idler

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=list;get;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list;get;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=list;get;watch;patch

// CLIIdler will run the CLI idler process.
func (h *Handler) CLIIdler() {
	ctx := context.Background()
	opLog := h.Log.WithName("aergia-controller").WithName("CLIIdler")
	listOption := &client.ListOptions{}
	// in kubernetes, we can reliably check for the existence of this label so that
	// we only check namespaces that have been deployed by a lagoon at one point
	labelRequirements := generateLabelRequirements(h.Selectors.CLI.Namespace)
	listOption = (&client.ListOptions{}).ApplyOptions([]client.ListOption{
		client.MatchingLabelsSelector{
			Selector: labels.NewSelector().Add(labelRequirements...),
		},
	})
	namespaces := &corev1.NamespaceList{}
	if err := h.Client.List(ctx, namespaces, listOption); err != nil {
		opLog.Error(err, fmt.Sprintf("unable to get any namespaces"))
		return
	}
	for _, namespace := range namespaces.Items {
		projectAutoIdle, ok1 := namespace.ObjectMeta.Labels[h.Selectors.NamespaceSelectorsLabels.ProjectIdling]
		environmentAutoIdle, ok2 := namespace.ObjectMeta.Labels[h.Selectors.NamespaceSelectorsLabels.EnvironmentIdling]
		if ok1 && ok2 {
			if environmentAutoIdle == "1" && projectAutoIdle == "1" {
				envOpLog := opLog.WithValues("namespace", namespace.ObjectMeta.Name).
					WithValues("project", namespace.ObjectMeta.Labels[h.Selectors.NamespaceSelectorsLabels.ProjectName]).
					WithValues("environment", namespace.ObjectMeta.Labels[h.Selectors.NamespaceSelectorsLabels.EnvironmentName]).
					WithValues("dry-run", h.DryRun)
				envOpLog.Info(fmt.Sprintf("Checking namespace"))
				h.kubernetesCLI(ctx, envOpLog, namespace)
			} else {
				if h.Debug {
					opLog.Info(fmt.Sprintf("skipping namespace %s; autoidle values are env:%s proj:%s",
						namespace.ObjectMeta.Name,
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
