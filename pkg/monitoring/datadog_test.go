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
	assert.Empty(t, actual.client.Tags)
}
