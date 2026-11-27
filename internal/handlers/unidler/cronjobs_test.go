package unidler

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUnsuspendCronjobs(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = batchv1.AddToScheme(scheme)
	yes, no := true, true
	idled := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "idled", Namespace: "ns", Labels: map[string]string{
			"lagoon.sh/project": "p", "idling.amazee.io/watch": "true", "idling.amazee.io/idled": "true",
		}},
		Spec: batchv1.CronJobSpec{Suspend: &yes},
	}
	userSuspended := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "user", Namespace: "ns", Labels: map[string]string{"lagoon.sh/project": "p"}},
		Spec:       batchv1.CronJobSpec{Suspend: &no},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(idled, userSuspended).Build()
	h := &Unidler{Client: c}
	h.unsuspendCronjobs(context.Background(), "ns", logr.Discard())

	got := &batchv1.CronJob{}
	_ = c.Get(context.Background(), types.NamespacedName{Namespace: "ns", Name: "idled"}, got)
	if *got.Spec.Suspend || got.Labels["idling.amazee.io/idled"] != "false" {
		t.Errorf("idled cronjob not unsuspended: %+v", got)
	}
	_ = c.Get(context.Background(), types.NamespacedName{Namespace: "ns", Name: "user"}, got)
	if !*got.Spec.Suspend {
		t.Errorf("user suspended cronjob should stay suspended")
	}
}
