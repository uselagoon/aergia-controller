package idler

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

func newCronjob(name string, suspended bool) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns", Labels: map[string]string{"lagoon.sh/project": "p"}},
		Spec:       batchv1.CronJobSpec{Suspend: &suspended},
	}
}

func TestIdleCronjobs(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = batchv1.AddToScheme(scheme)
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(newCronjob("active", false), newCronjob("user-suspended", true)).Build()
	h := &Idler{Client: c, Selectors: &Data{}}
	h.idleCronjobs(context.Background(), logr.Discard(), "ns", true, false)

	active := &batchv1.CronJob{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "ns", Name: "active"}, active); err != nil {
		t.Fatal(err)
	}
	if !*active.Spec.Suspend || active.Labels["idling.amazee.io/idled"] != "true" || active.Labels["idling.amazee.io/force-idled"] != "true" {
		t.Errorf("active cronjob not suspended and labelled: %+v", active.ObjectMeta)
	}
	user := &batchv1.CronJob{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "ns", Name: "user-suspended"}, user); err != nil {
		t.Fatal(err)
	}
	if user.Labels["idling.amazee.io/idled"] != "" {
		t.Errorf("user suspended cronjob should not be labelled: %+v", user.Labels)
	}
}
