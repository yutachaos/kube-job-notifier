package main

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
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

func TestGetLogMode(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		annotation  string
		expected    logMode
	}{
		{
			name: "owner container",
			annotations: map[string]string{
				"foo": "bar",
			},
			annotation: "",
			expected:   ownerContainer,
		},
		{
			name: "owner container",
			annotations: map[string]string{
				"foo":                     "bar",
				"logging.k8s.io/log-mode": "OwnerContainer",
			},
			annotation: "logging.k8s.io/log-mode",
			expected:   ownerContainer,
		},
		{
			name: "pod only",
			annotations: map[string]string{
				"foo":                     "bar",
				"logging.k8s.io/log-mode": "PodOnly",
			},
			annotation: "logging.k8s.io/log-mode",
			expected:   podOnly,
		},
		{
			name: "pod containers",
			annotations: map[string]string{
				"foo":                     "bar",
				"logging.k8s.io/log-mode": "PodContainers",
			},
			annotation: "logging.k8s.io/log-mode",
			expected:   podContainers,
		},
		{
			name: "invalid",
			annotations: map[string]string{
				"foo":                     "bar",
				"logging.k8s.io/log-mode": "invalid",
			},
			annotation: "logging.k8s.io/log-mode",
			expected:   ownerContainer,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := getLogMode(test.annotations, test.annotation)
			if actual != test.expected {
				t.Errorf("expected log mode %d, but got %d", test.expected, actual)
			}
		})
	}
}

func TestGetJobLogs(t *testing.T) {
	type args struct {
		clientset   kubernetes.Interface
		pod         corev1.Pod
		cronJobName string
		mode        logMode
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "get logs from pod with one container",
			args: args{
				clientset: fake.NewSimpleClientset(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "test-container"},
						},
					},
				}),
				pod: corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "test-container"},
						},
					},
				},
				cronJobName: "",
				mode:        podContainers,
			},
			want: "fake logs",
		},
		{
			name: "get logs from pod with multiple containers",
			args: args{
				clientset: fake.NewSimpleClientset(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "test-container1"},
							{Name: "test-container2"},
						},
					},
				}),
				pod: corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "test-container1"},
							{Name: "test-container2"},
						},
					}},
				cronJobName: "",
				mode:        podContainers,
			},
			want: "Container test-container1 logs:\r\nfake logs\r\nContainer test-container2 logs:\r\nfake logs\r\n",
		},
		{
			name: "get logs from pod only",
			args: args{
				clientset: fake.NewSimpleClientset(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
				}),
				pod:         corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"}},
				cronJobName: "",
				mode:        podOnly,
			},
			want: "fake logs",
		},
		{
			name: "get logs from cron job",
			args: args{
				clientset:   fake.NewSimpleClientset(),
				pod:         corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"}},
				cronJobName: "test-cronjob",
				mode:        ownerContainer,
			},
			want: "fake logs",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if got := getJobLogs(tt.args.clientset, tt.args.pod, tt.args.cronJobName, tt.args.mode); got != tt.want {
				t.Errorf("getJobLogs() = %v, want %v", got, tt.want)
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
