package runner

import (
	"testing"

	"boscoin.io/sebak/lib/common"
	"github.com/stretchr/testify/require"
)

func TestConnectionManagerBroadcaster(t *testing.T) {
	conf := common.NewTestConfig()

	recv := make(chan struct{})
	nr, _, cm := createNodeRunnerForTesting(3, conf, recv)

	nr.startStateManager()
	defer nr.StopStateManager()

	<-recv
	require.Equal(t, 1, len(cm.Messages()))
}
