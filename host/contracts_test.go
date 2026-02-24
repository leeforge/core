package host

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCoreOptions_DefaultsAreSafe(t *testing.T) {
	opts := CoreOptions{}
	require.False(t, opts.SkipMigrate)
	require.Nil(t, opts.PluginRegistrar)
}
