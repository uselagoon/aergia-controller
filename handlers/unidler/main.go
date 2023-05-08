package unidler

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
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
	Client          ctrlClient.Client
	Log             logr.Logger
	RefreshInterval int
	Debug           bool
	RequestCount    *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	Locks           sync.Map
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
	r.HandleFunc("/", h.errorHandler(errFilesPath))
	http.Handle("/", r)

	httpServer := &http.Server{
		Addr:    ":5000",
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
	fmt.Fprintln(w, fmt.Sprintf("%s\n", favicon))
}

func (h *Unidler) errorHandler(path string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		opLog := h.Log.WithValues("custom-default-backend", "request")
		start := time.Now()
		ext := "html"
		// if debug is enabled, then set the headers in the response too
		if os.Getenv("DEBUG") == "true" {
			w.Header().Set(FormatHeader, r.Header.Get(FormatHeader))
			w.Header().Set(CodeHeader, r.Header.Get(CodeHeader))
			w.Header().Set(ContentType, r.Header.Get(ContentType))
			w.Header().Set(OriginalURI, r.Header.Get(OriginalURI))
			w.Header().Set(Namespace, r.Header.Get(Namespace))
			w.Header().Set(IngressName, r.Header.Get(IngressName))
			w.Header().Set(ServiceName, r.Header.Get(ServiceName))
			w.Header().Set(ServicePort, r.Header.Get(ServicePort))
			w.Header().Set(RequestID, r.Header.Get(RequestID))
		}

		format := r.Header.Get(FormatHeader)
		if format == "" {
			format = "text/html"
			// log.Printf("format not specified. Using %v", format)
		}

		cext, err := mime.ExtensionsByType(format)
		if err != nil {
			// log.Printf("unexpected error reading media type extension: %v. Using %v", err, ext)
			format = "text/html"
		} else if len(cext) == 0 {
			// log.Printf("couldn't get media type extension. Using %v", ext)
		} else {
			ext = cext[0]
		}
		w.Header().Set(ContentType, format)
		w.Header().Set(AergiaHeader, "true")
		w.Header().Set(CacheControl, "private,no-store")

		errCode := r.Header.Get(CodeHeader)
		code, err := strconv.Atoi(errCode)
		if err != nil {
			code = 404
			// log.Printf("unexpected error reading return code: %v. Using %v", err, code)
		}
		w.WriteHeader(code)
		ns := r.Header.Get(Namespace)
		// @TODO: check for code 503 specifically, or just any request that has the namespace in it will be "unidled" if a request comes in for
		// that ingress and the
		if ns != "" {
			// if a namespace exists, it means that the custom-http-errors code is defined in the ingress object
			// so do something with that here, like kickstart the idler process to unidle targets
			opLog.Info(fmt.Sprintf("Got request in namespace %s", ns))

			file := fmt.Sprintf("%v/unidle.html", path)
			forceScaled := h.checkForceScaled(ctx, ns, opLog)
			if forceScaled {
				// if this has been force scaled, return the force scaled landing page
				file = fmt.Sprintf("%v/forced.html", path)
			} else {
				// only unidle environments that aren't force scaled
				// actually do the unidling here, lock to prevent multiple unidle operations from running
				_, ok := h.Locks.Load(ns)
				if !ok {
					_, _ = h.Locks.LoadOrStore(ns, ns)
					go h.UnIdle(ctx, ns, opLog)
				}
			}
			if h.Debug == true {
				opLog.Info(fmt.Sprintf("Serving custom error response for code %v and format %v from file %v", code, format, file))
			}
			// then return the unidle template to the user
			tmpl := template.Must(template.ParseFiles(file))
			tmpl.ExecuteTemplate(w, "base", pageData{
				ErrorCode:       strconv.Itoa(code),
				FormatHeader:    r.Header.Get(FormatHeader),
				CodeHeader:      r.Header.Get(CodeHeader),
				ContentType:     r.Header.Get(ContentType),
				OriginalURI:     r.Header.Get(OriginalURI),
				Namespace:       r.Header.Get(Namespace),
				IngressName:     r.Header.Get(IngressName),
				ServiceName:     r.Header.Get(ServiceName),
				ServicePort:     r.Header.Get(ServicePort),
				RequestID:       r.Header.Get(RequestID),
				RefreshInterval: h.RefreshInterval,
			})
		} else {
			// otherwise just handle the generic http responses here
			if !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}
			file := fmt.Sprintf("%v/error.html", path)
			if h.Debug == true {
				opLog.Info(fmt.Sprintf("Serving custom error response for code %v and format %v from file %v", code, format, file))
			}
			tmpl := template.Must(template.ParseFiles(file))
			tmpl.ExecuteTemplate(w, "base", pageData{
				ErrorCode:       strconv.Itoa(code),
				ErrorMessage:    http.StatusText(code),
				FormatHeader:    r.Header.Get(FormatHeader),
				CodeHeader:      r.Header.Get(CodeHeader),
				ContentType:     r.Header.Get(ContentType),
				OriginalURI:     r.Header.Get(OriginalURI),
				Namespace:       r.Header.Get(Namespace),
				IngressName:     r.Header.Get(IngressName),
				ServiceName:     r.Header.Get(ServiceName),
				ServicePort:     r.Header.Get(ServicePort),
				RequestID:       r.Header.Get(RequestID),
				RefreshInterval: h.RefreshInterval,
			})
		}

		duration := time.Now().Sub(start).Seconds()

		proto := strconv.Itoa(r.ProtoMajor)
		proto = fmt.Sprintf("%s.%s", proto, strconv.Itoa(r.ProtoMinor))

		h.RequestCount.WithLabelValues(proto).Inc()
		h.RequestDuration.WithLabelValues(proto).Observe(duration)
	}
}

func (h *Unidler) checkForceScaled(ctx context.Context, ns string, opLog logr.Logger) bool {
	// get the deployments in the namespace if they have the `watch=true` label
	labelRequirements1, _ := labels.NewRequirement("idling.amazee.io/force-scaled", selection.Equals, []string{"true"})
	listOption := (&ctrlClient.ListOptions{}).ApplyOptions([]ctrlClient.ListOption{
		ctrlClient.InNamespace(ns),
		client.MatchingLabelsSelector{
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

func (h *Unidler) UnIdle(ctx context.Context, ns string, opLog logr.Logger) {
	defer h.Locks.Delete(ns)
	// get the deployments in the namespace if they have the `watch=true` label
	labelRequirements1, _ := labels.NewRequirement("idling.amazee.io/watch", selection.Equals, []string{"true"})
	listOption := (&ctrlClient.ListOptions{}).ApplyOptions([]ctrlClient.ListOption{
		ctrlClient.InNamespace(ns),
		client.MatchingLabelsSelector{
			Selector: labels.NewSelector().Add(*labelRequirements1),
		},
	})
	deployments := &appsv1.DeploymentList{}
	if err := h.Client.List(ctx, deployments, listOption); err != nil {
		opLog.Info(fmt.Sprintf("Unable to get any deployments - %s", ns))
		return
	}
	for _, deploy := range deployments.Items {
		// if the idled annotation is true
		av, aok := deploy.ObjectMeta.Annotations["idling.amazee.io/idled"]
		lv, lok := deploy.ObjectMeta.Labels["idling.amazee.io/idled"]
		if aok && av == "true" || lok && lv == "true" {
			opLog.Info(fmt.Sprintf("Deployment %s - Replicas %v - %s", deploy.ObjectMeta.Name, *deploy.Spec.Replicas, ns))
			if *deploy.Spec.Replicas == 0 {
				// default to scaling to 1 replica
				newReplicas := 1
				if value, ok := deploy.ObjectMeta.Annotations["idling.amazee.io/unidle-replicas"]; ok {
					// but if the value of the annotation is greater than 0, use what is in the annotation instead
					unidleReplicas, err := strconv.Atoi(value)
					if err == nil {
						if unidleReplicas > 0 {
							newReplicas = unidleReplicas
						}
					}
				}
				mergePatch, _ := json.Marshal(map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": newReplicas,
					},
					"metadata": map[string]interface{}{
						"labels": map[string]*string{
							"idling.amazee.io/idled":        nil,
							"idling.amazee.io/force-idled":  nil,
							"idling.amazee.io/force-scaled": nil,
						},
						"annotations": map[string]*string{
							"idling.amazee.io/idled-at": nil,
							"idling.amazee.io/idled":    nil,
						},
					},
				})
				scaleDepConf := deploy.DeepCopy()
				if err := h.Client.Patch(ctx, scaleDepConf, ctrlClient.RawPatch(types.MergePatchType, mergePatch)); err != nil {
					// log it but try and scale the rest of the deployments anyway (some idled is better than none?)
					opLog.Info(fmt.Sprintf("Error scaling deployment %s - %s", deploy.ObjectMeta.Name, ns))
				} else {
					opLog.Info(fmt.Sprintf("Deployment %s scaled to %d - %s", deploy.ObjectMeta.Name, newReplicas, ns))
				}
			}
		}
	}
	// now wait for the pods of these deployments to be ready
	// this could still result in 503 for users until the resulting services/endpoints are active and receiving traffic
	for _, deploy := range deployments.Items {
		opLog.Info(fmt.Sprintf("Waiting for %s to be running - %s", deploy.ObjectMeta.Name, ns))
		timeout, cancel := context.WithTimeout(ctx, defaultPollTimeout)
		defer cancel()
		wait.PollUntilWithContext(timeout, defaultPollDuration, h.hasRunningPod(ctx, ns, deploy.Name))
	}
	// remove the 503 code from any ingress objects that have it in this namespace
	h.removeCodeFromIngress(ctx, ns, opLog)
}

func (h *Unidler) removeCodeFromIngress(ctx context.Context, ns string, opLog logr.Logger) {
	// get the ingresses in the namespace
	listOption := (&ctrlClient.ListOptions{}).ApplyOptions([]ctrlClient.ListOption{
		ctrlClient.InNamespace(ns),
	})
	ingresses := &networkv1.IngressList{}
	if err := h.Client.List(ctx, ingresses, listOption); err != nil {
		opLog.Info(fmt.Sprintf("Unable to get any deployments - %s", ns))
		return
	}
	for _, ingress := range ingresses.Items {
		// if the nginx.ingress.kubernetes.io/custom-http-errors annotation is set
		// then strip out the 503 error code that is there so that
		// users will see their application errors rather than the loading page
		if value, ok := ingress.ObjectMeta.Annotations["nginx.ingress.kubernetes.io/custom-http-errors"]; ok {
			newVals := removeStatusCode(value, "503")
			// if the 503 code was removed from the annotation
			// then patch it
			if newVals == nil || *newVals != value {
				mergePatch, _ := json.Marshal(map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]*string{
							"idling.amazee.io/idled": nil,
						},
						"annotations": map[string]*string{
							"nginx.ingress.kubernetes.io/custom-http-errors": newVals,
							"idling.amazee.io/idled-at":                      nil,
							"idling.amazee.io/idled":                         nil,
						},
					},
				})
				patchIngress := ingress.DeepCopy()
				if err := h.Client.Patch(ctx, patchIngress, ctrlClient.RawPatch(types.MergePatchType, mergePatch)); err != nil {
					// log it but try and patch the rest of the ingressses anyway (some is better than none?)
					opLog.Info(fmt.Sprintf("Error patching custom-http-errors on ingress %s - %s", ingress.ObjectMeta.Name, ns))
				} else {
					if newVals == nil {
						opLog.Info(fmt.Sprintf("Ingress %s custom-http-errors annotation removed - %s", ingress.ObjectMeta.Name, ns))
					} else {
						opLog.Info(fmt.Sprintf("Ingress %s custom-http-errors annotation patched with %s - %s", ingress.ObjectMeta.Name, *newVals, ns))
					}
				}
			}
		}
	}
}

func removeStatusCode(codes string, code string) *string {
	newCodes := []string{}
	for _, codeValue := range strings.Split(codes, ",") {
		if codeValue != code {
			newCodes = append(newCodes, codeValue)
		}
	}
	if len(newCodes) == 0 {
		return nil
	}
	returnCodes := strings.Join(newCodes, ",")
	return &returnCodes
}

func (h *Unidler) hasRunningPod(ctx context.Context, namespace, deployment string) wait.ConditionWithContextFunc {
	return func(context.Context) (bool, error) {
		var d appsv1.Deployment
		if err := h.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: deployment}, &d); err != nil {
			return false, err
		}
		var pods corev1.PodList
		listOption := (&ctrlClient.ListOptions{}).ApplyOptions([]ctrlClient.ListOption{
			client.MatchingLabelsSelector{
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
