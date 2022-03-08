package idler

import (
	"bytes"
	"fmt"
	"io"

	"github.com/go-logr/logr"
	prometheusapi "github.com/prometheus/client_golang/api"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/remotecommand"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	config "sigs.k8s.io/controller-runtime/pkg/client/config"
)

// for pod exec and log collection
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=list;get

// IdlerHandler handles idling of cli and services.
type IdlerHandler struct {
	Client                  client.Client
	PodCheckInterval        int
	Log                     logr.Logger
	Scheme                  *runtime.Scheme
	DryRun                  bool
	Debug                   bool
	Selectors               *IdlerData
	PrometheusClient        prometheusapi.Client
	PrometheusCheckInterval string
}

type idlerSelector struct {
	Name     string             `json:"name"`
	Operator selection.Operator `json:"operator"`
	Values   []string           `json:"values,omitempty"`
}

// IdlerData .
type IdlerData struct {
	NamespaceSelectorsLabels struct {
		ProjectName       string `json:"projectName"`
		EnvironmentName   string `json:"environmentName"`
		ProjectIdling     string `json:"projectIdling"`
		EnvironmentIdling string `json:"environmentIdling"`
		EnvironmentType   string `json:"environmentType"`
	}
	ServiceName string `json:"serviceName"`
	CLI         struct {
		SkipBuildCheck   bool            `json:"skipBuildCheck"`
		SkipCronCheck    bool            `json:"skipCronCheck"`
		SkipProcessCheck bool            `json:"skipProcessCheck"`
		Namespace        []idlerSelector `json:"namespace"`
		Builds           []idlerSelector `json:"builds"`
		Deployments      []idlerSelector `json:"deployments"`
		Pods             []idlerSelector `json:"pods"`
	} `json:"cli"`
	Service struct {
		SkipBuildCheck   bool            `json:"skipBuildCheck"`
		SkipHitCheck     bool            `json:"skipHitCheck"`
		SkipIngressPatch bool            `json:"skipIngressPatch"`
		Namespace        []idlerSelector `json:"namespace"`
		Builds           []idlerSelector `json:"builds"`
		Deployments      []idlerSelector `json:"deployments"`
		Pods             []idlerSelector `json:"pods"`
		Ingress          []idlerSelector `json:"ingress"`
	} `json:"service"`
}

func execPod(
	podName, namespace string,
	command []string,
	stdin io.Reader,
	tty bool,
) (string, string, error) {
	restCfg, err := config.GetConfig()
	if err != nil {
		return "", "", fmt.Errorf("unable to get config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return "", "", fmt.Errorf("unable to create client: %v", err)
	}
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return "", "", fmt.Errorf("error adding to scheme: %v", err)
	}
	if len(command) == 0 {
		command = []string{"sh"}
	}
	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command: command,
		Stdin:   stdin != nil,
		Stdout:  true,
		Stderr:  true,
		TTY:     tty,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(restCfg, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("error while creating Executor: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    tty,
	})
	if err != nil {
		return "", "", fmt.Errorf("error in Stream: %v", err)
	}

	return stdout.String(), stderr.String(), nil
}
