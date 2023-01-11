package monitoring

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestNewDatadog(t *testing.T) {
	os.Setenv("DD_TAGS", "tag")
	os.Setenv("DD_NAMESPACE", "namespace")

	actual := newDatadog()

	assert.Equal(t, "namespace", actual.client.Namespace)
	assert.Equal(t, []string{"tag"}, actual.client.Tags)

	os.Unsetenv("DD_TAGS")
	os.Unsetenv("DD_NAMESPACE")

	actual = newDatadog()
	assert.Empty(t, actual.client.Namespace)
	assert.Equal(t, []string{}, actual.client.Tags)
}

func TestisSubscriptionSuppressed(t *testing.T) {
	tests := []struct {
		Name                   string
		annotations            map[string]string
		suppressAnnotationName string
		expected               bool
	}{
		{
			"No annotations",
			map[string]string{
				"kube-job-notifier/foo": "bar",
			},
			"kube-job-notifier/suppress-success-datadog-subscription",
			false,
		},
		{
			"Annotation not true",
			map[string]string{
				"kube-job-notifier/suppress-success-datadog-subscription": "false",
			},
			"kube-job-notifier/suppress-success-datadog-subscription",
			false,
		},
		{
			"Annotation true",
			map[string]string{
				"kube-job-notifier/suppress-success-datadog-subscription": "true",
			},
			"kube-job-notifier/suppress-success-datadog-subscription",
			true,
		},
		{
			"Annotation not true",
			map[string]string{
				"kube-job-notifier/suppress-failed-datadog-subscription": "false",
			},
			"kube-job-notifier/suppress-failed-datadog-subscription",
			false,
		},
		{
			"Annotation true",
			map[string]string{
				"kube-job-notifier/suppress-failed-datadog-subscription": "true",
			},
			"kube-job-notifier/suppress-failed-datadog-subscription",
			true,
		},
		{
			"Nil annotation not break",
			nil,
			"kube-job-notifier/suppress-success-datadog-subscription",
			false,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			result := isSubscriptionSuppressed(test.annotations, test.suppressAnnotationName)

			assert.Equal(t, test.expected, result)
		})
	}
}
