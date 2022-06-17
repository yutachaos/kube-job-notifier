package main

import (
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	utilpointer "k8s.io/utils/pointer"
	"testing"
)

func TestIsCompletedJob(t *testing.T) {

	tests := []struct {
		Name     string
		job      *batchv1.Job
		pods     *v1.PodList
		expected bool
	}{
		{
			"Job is succeeded",
			&batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{UID: "test"},
				Spec:       batchv1.JobSpec{BackoffLimit: utilpointer.Int32Ptr(0)},
				Status:     batchv1.JobStatus{Succeeded: 1},
			},
			&v1.PodList{
				TypeMeta: metav1.TypeMeta{},
				ListMeta: metav1.ListMeta{},
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{searchLabel: "test"}},
						Spec:       v1.PodSpec{},
						Status:     v1.PodStatus{},
					},
				},
			},
			true,
		},
		{
			"Job is completed",
			&batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{UID: "test"},
				Spec:       batchv1.JobSpec{BackoffLimit: utilpointer.Int32Ptr(0)},
				Status:     batchv1.JobStatus{},
			},
			&v1.PodList{
				TypeMeta: metav1.TypeMeta{},
				ListMeta: metav1.ListMeta{},
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{searchLabel: "test"}},
						Spec:       v1.PodSpec{},
						Status:     v1.PodStatus{},
					},
				},
			},
			true,
		},
		{
			"Job is completed when retried",
			&batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{UID: "test"},
				Spec:       batchv1.JobSpec{BackoffLimit: utilpointer.Int32Ptr(3)},
				Status:     batchv1.JobStatus{},
			},
			&v1.PodList{
				TypeMeta: metav1.TypeMeta{},
				ListMeta: metav1.ListMeta{},
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{searchLabel: "test"}},
						Spec:       v1.PodSpec{},
						Status:     v1.PodStatus{},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{searchLabel: "test"}},
						Spec:       v1.PodSpec{},
						Status:     v1.PodStatus{},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{searchLabel: "test"}},
						Spec:       v1.PodSpec{},
						Status:     v1.PodStatus{},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{searchLabel: "test"}},
						Spec:       v1.PodSpec{},
						Status:     v1.PodStatus{},
					},
				},
			}, true,
		},
		{
			"Job is not completed",
			&batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{UID: "test"},
				Spec:       batchv1.JobSpec{BackoffLimit: utilpointer.Int32Ptr(3)},
				Status:     batchv1.JobStatus{},
			},
			&v1.PodList{
				TypeMeta: metav1.TypeMeta{},
				ListMeta: metav1.ListMeta{},
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{searchLabel: "test"}},
						Spec:       v1.PodSpec{},
						Status:     v1.PodStatus{},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{searchLabel: "test"}},
						Spec:       v1.PodSpec{},
						Status:     v1.PodStatus{},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{searchLabel: "test"}},
						Spec:       v1.PodSpec{},
						Status:     v1.PodStatus{},
					},
				},
			}, false,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fakeClient := &fake.Clientset{}
			fakeClient = addListPodsReactor(fakeClient, test.pods)
			job := test.job

			result := isCompletedJob(fakeClient, job)
			if result != test.expected {
				t.Errorf("In test case %s, expext:%v, got  %v", test.Name, test.expected, result)
			}

		})
	}
}

func addListPodsReactor(fakeClient *fake.Clientset, obj runtime.Object) *fake.Clientset {
	fakeClient.AddReactor("list", "pods", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, obj, nil
	})
	return fakeClient
}
