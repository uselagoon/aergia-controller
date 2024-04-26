package unidler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=list;get;watch;patch;update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=list;get;watch
// +kubebuilder:rbac:groups=*,resources=ingresses,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=*,resources=ingress/status,verbs=get;update;patch

const (
	defaultPollDuration = 1 * time.Second
	defaultPollTimeout  = 90 * time.Second
)

// Unidler is the client structure for http handlers.
type Unidler struct {
	Client            client.Client
	Log               logr.Logger
	RefreshInterval   int
	UnidlerHTTPPort   int
	Debug             bool
	VerifiedUnidling  bool
	VerifiedSecret    string
	RequestCount      *prometheus.CounterVec
	RequestDuration   *prometheus.HistogramVec
	Locks             sync.Map
	AllowedUserAgents []string
	BlockedUserAgents []string
	AllowedIPs        []string
	BlockedIPs        []string
}

type pageData struct {
	RefreshInterval int
	FormatHeader    string
	CodeHeader      string
	ContentType     string
	OriginalURI     string
	Namespace       string
	IngressName     string
	ServiceName     string
	ServicePort     string
	RequestID       string
	ErrorCode       string
	ErrorMessage    string
	Verifier        string
}

const (
	// FormatHeader name of the header used to extract the format
	FormatHeader = "X-Format"
	// CodeHeader name of the header used as source of the HTTP status code to return
	CodeHeader = "X-Code"
	// ContentType name of the header that defines the format of the reply
	ContentType = "Content-Type"
	// OriginalURI name of the header with the original URL from NGINX
	OriginalURI = "X-Original-URI"
	// Namespace name of the header that contains information about the Ingress namespace
	Namespace = "X-Namespace"
	// IngressName name of the header that contains the matched Ingress
	IngressName = "X-Ingress-Name"
	// ServiceName name of the header that contains the matched Service in the Ingress
	ServiceName = "X-Service-Name"
	// ServicePort name of the header that contains the matched Service port in the Ingress
	ServicePort = "X-Service-Port"
	// RequestID is a unique ID that identifies the request - same as for backend service
	RequestID = "X-Request-ID"
	// AergiaHeader name of the header that contains if this has been served by aergia
	AergiaHeader = "X-Aergia"
	// CacheControl name of the header that defines the cache control config
	CacheControl = "Cache-Control"
	// ErrFilesPathVar is the name of the environment variable indicating
	// the location on disk of files served by the handler.
	ErrFilesPathVar = "ERROR_FILES_PATH"
)

var (
	favicon = "data:image/x-icon;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
)

// Run runs the http server.
func Run(h *Unidler, setupLog logr.Logger) {
	errFilesPath := "/www"
	if os.Getenv(ErrFilesPathVar) != "" {
		errFilesPath = os.Getenv(ErrFilesPathVar)
	}

	r := http.NewServeMux()
	r.HandleFunc("/favicon.ico", faviconHandler)
	r.HandleFunc("/", h.ingressHandler(errFilesPath))
	http.Handle("/", r)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", h.UnidlerHTTPPort),
		Handler: r,
	}
	err := httpServer.ListenAndServe()
	if err != nil {
		setupLog.Error(err, "unable to start http server")
		os.Exit(1)
	}
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(AergiaHeader, "true")
	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "public, max-age=7776000")
	fmt.Fprintf(w, "%s\n", favicon)
}

func (h *Unidler) convertOldLabels(ctx context.Context, namespace *corev1.Namespace, opLog logr.Logger) {
	labelRequirements1, _ := labels.NewRequirement("idling.amazee.io/watch", selection.Exists, []string{})
	listOption := (&client.ListOptions{}).ApplyOptions([]client.ListOption{
		client.InNamespace(namespace.Name),
		client.MatchingLabelsSelector{
			Selector: labels.NewSelector().Add(*labelRequirements1),
		},
	})
	deployments := &appsv1.DeploymentList{}
	if err := h.Client.List(ctx, deployments, listOption); err != nil {
		opLog.Error(err, fmt.Sprintf("Error getting deployments for namespace %s", namespace.ObjectMeta.Name))
	} else {
		for _, deployment := range deployments.Items {
			deploymentPatchAnnotations := map[string]interface{}{}
			deploymentPatchLabels := map[string]interface{}{}
			if val, ok := deployment.ObjectMeta.Labels["idling.amazee.io/watch"]; ok {
				deploymentPatchLabels["idling.lagoon.sh/watch"] = val
				deploymentPatchLabels["idling.amazee.io/watch"] = nil
			}
			if val, ok := deployment.ObjectMeta.Labels["idling.amazee.io/idled"]; ok {
				deploymentPatchLabels["idling.lagoon.sh/idled"] = val
				deploymentPatchLabels["idling.amazee.io/idled"] = nil
			}
			if val, ok := deployment.ObjectMeta.Annotations["idling.amazee.io/idled"]; ok {
				deploymentPatchAnnotations["idling.lagoon.sh/idled"] = val
				deploymentPatchAnnotations["idling.amazee.io/idled"] = nil
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
				patchDeployment := deployment.DeepCopy()
				deploymentMergePatch, _ := json.Marshal(map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels":      deploymentPatchLabels,
						"annotations": deploymentPatchAnnotations,
					},
				})
				opLog.Info(fmt.Sprintf("Patching deployment %s with converted annotations and labels", patchDeployment.Name))
				if err := h.Client.Patch(ctx, patchDeployment, client.RawPatch(types.MergePatchType, deploymentMergePatch)); err != nil {
					// log it but try and scale the rest of the deployments anyway (some idled is better than none?)
					opLog.Info(fmt.Sprintf("Error patching deployment %s -%v", patchDeployment.Name, err))
				}
				time.Sleep(500 * time.Microsecond)
			}
		}
	}
}

func (h *Unidler) Unidle(ctx context.Context, namespace *corev1.Namespace, opLog logr.Logger) {
	defer h.Locks.Delete(namespace.Name)

	// convert any old labels/annotations on deployments first
	h.convertOldLabels(ctx, namespace, opLog)

	// get the deployments in the namespace if they have the `watch=true` label

	labelRequirements1, _ := labels.NewRequirement("idling.lagoon.sh/watch", selection.Equals, []string{"true"})
	listOption := (&client.ListOptions{}).ApplyOptions([]client.ListOption{
		client.InNamespace(namespace.Name),
		client.MatchingLabelsSelector{
			Selector: labels.NewSelector().Add(*labelRequirements1),
		},
	})
	deployments := &appsv1.DeploymentList{}
	if err := h.Client.List(ctx, deployments, listOption); err != nil {
		opLog.Info(fmt.Sprintf("Unable to get any deployments - %s", namespace.Name))
		return
	}
	for _, deploy := range deployments.Items {
		// if the idled annotation is true
		opLog.Info(fmt.Sprintf("Unable to get any deployments - %s", namespace.Name))
		lv, lok := deploy.ObjectMeta.Labels["idling.lagoon.sh/idled"]
		if lok && lv == "true" {
			opLog.Info(fmt.Sprintf("Deployment %s - Replicas %v - %s", deploy.ObjectMeta.Name, *deploy.Spec.Replicas, namespace.Name))
			if *deploy.Spec.Replicas == 0 {
				// default to scaling to 1 replica
				newReplicas := 1
				if value, ok := deploy.ObjectMeta.Annotations["idling.lagoon.sh/unidle-replicas"]; ok {
					// but if the value of the annotation is greater than 0, use what is in the annotation instead
					unidleReplicas, err := strconv.Atoi(value)
					if err == nil {
						if unidleReplicas > 0 {
							newReplicas = unidleReplicas
						}
					}
				}
				labels := map[string]interface{}{
					"idling.lagoon.sh/idled":        "false",
					"idling.lagoon.sh/force-idled":  nil,
					"idling.lagoon.sh/force-scaled": nil,
				}
				annotations := map[string]interface{}{
					"idling.lagoon.sh/idled-at": nil,
				}
				mergePatch, _ := json.Marshal(map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": newReplicas,
					},
					"metadata": map[string]interface{}{
						"labels":      labels,
						"annotations": annotations,
					},
				})
				scaleDepConf := deploy.DeepCopy()
				if err := h.Client.Patch(ctx, scaleDepConf, client.RawPatch(types.MergePatchType, mergePatch)); err != nil {
					// log it but try and scale the rest of the deployments anyway (some idled is better than none?)
					opLog.Info(fmt.Sprintf("Error scaling deployment %s - %s", deploy.ObjectMeta.Name, namespace.Name))
				} else {
					opLog.Info(fmt.Sprintf("Deployment %s scaled to %d - %s", deploy.ObjectMeta.Name, newReplicas, namespace.Name))
				}
			}
		}
	}
	// now wait for the pods of these deployments to be ready
	// this could still result in 503 for users until the resulting services/endpoints are active and receiving traffic
	for _, deploy := range deployments.Items {
		opLog.Info(fmt.Sprintf("Waiting for %s to be running - %s", deploy.ObjectMeta.Name, namespace.Name))
		wait.PollUntilContextTimeout(ctx, defaultPollDuration, defaultPollTimeout, true, h.hasRunningPod(ctx, namespace.Name, deploy.Name))
	}
	// remove the 503 code from any ingress objects that have it in this namespace
	h.removeCodeFromIngress(ctx, namespace.Name, opLog)
	// label the namespace to indicate it is idled
	namespaceCopy := namespace.DeepCopy()
	mergePatch, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				"idling.lagoon.sh/idled": "false",
				"idling.amazee.io/idled": nil,
			},
		},
	})
	unidleEvents.Inc()
	if err := h.Client.Patch(ctx, namespaceCopy, client.RawPatch(types.MergePatchType, mergePatch)); err != nil {
		opLog.Info(fmt.Sprintf("Error patching namespace %s", namespace.Name))
	}
}
