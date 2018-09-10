// We can check that the `TransitISAACState()` call transitions the state.
package sebak

import (
	"testing"
	"time"

	"boscoin.io/sebak/lib/common"

	"github.com/stretchr/testify/require"
)

// 1. All 3 Nodes.
// 2. Not proposer itself.
// 3. When `ISAACStateManager` starts, the node waits for a proposed ballot.
// 4. TransitISAACState(SIGN) method is called.
// 5. ISAACState is changed to `SIGN`.
// 6. TimeoutSIGN is a millisecond.
// 7. After timeout, the node broadcasts B(`ACCEPT`, `EXP`).
func TestStateTransitFromTimeoutInitToAccept(t *testing.T) {
	nodeRunners := createTestNodeRunner(3)
	nr := nodeRunners[0]

	recvTransit := make(chan struct{})
	nr.isaacStateManager.SetTransitSignal(func() {
		recvTransit <- struct{}{}
	})

	recvBroadcast := make(chan struct{})
	b := NewTestBroadcastor(recvBroadcast)
	nr.SetBroadcastor(b)
	nr.SetProposerCalculator(TheOtherProposerCalculator{})

	nr.Consensus().SetLatestConsensusedBlock(genesisBlock)

	conf := NewISAACConfiguration()
	conf.TimeoutINIT = time.Hour
	conf.TimeoutSIGN = time.Millisecond
	conf.TimeoutACCEPT = time.Millisecond
	conf.TimeoutALLCONFIRM = time.Millisecond

	nr.SetConf(conf)

	nr.StartStateManager()
	<-recvTransit
	require.Equal(t, common.BallotStateINIT, nr.isaacStateManager.state.ballotState)

	nr.TransitISAACState(nr.isaacStateManager.State().round, common.BallotStateSIGN)
	<-recvTransit
	require.Equal(t, common.BallotStateSIGN, nr.isaacStateManager.state.ballotState)

	<-recvBroadcast
	require.Equal(t, 1, len(b.Messages))

	for _, message := range b.Messages {
		ballot, ok := message.(Ballot)
		require.True(t, ok)
		require.Equal(t, nr.localNode.Address(), ballot.Proposer())
		require.Equal(t, common.BallotStateACCEPT, ballot.State())
		require.Equal(t, common.VotingEXP, ballot.Vote())
	}
}

// 1. All 3 Nodes.
// 1. Proposer itself.
// 1. When `ISAACStateManager` starts, the node proposes a ballot.
// 1. ISAACState is changed to `SIGN`.
// 1. TransitISAACState(ACCEPT) method is called.
// 1. ISAACState is changed to `ACCEPT`.
// 1. TimeoutACCEPT is a millisecond.
// 1. After timeout, ISAACState is back to `INIT`
func TestStateTransitFromTimeoutSignToAccept(t *testing.T) {
	nodeRunners := createTestNodeRunner(3)
	nr := nodeRunners[0]

	recv := make(chan struct{})
	b := NewTestBroadcastor(recv)
	nr.SetBroadcastor(b)
	nr.SetProposerCalculator(SelfProposerCalculator{})

	nr.Consensus().SetLatestConsensusedBlock(genesisBlock)

	conf := NewISAACConfiguration()
	conf.TimeoutINIT = time.Hour
	conf.TimeoutSIGN = time.Hour
	conf.TimeoutACCEPT = time.Millisecond
	conf.TimeoutALLCONFIRM = time.Millisecond

	nr.SetConf(conf)

	nr.StartStateManager()
	<-recv

	require.Equal(t, 1, len(b.Messages))
	for _, message := range b.Messages {
		ballot, ok := message.(Ballot)
		require.True(t, ok)
		require.Equal(t, nr.localNode.Address(), ballot.Proposer())
		require.Equal(t, common.BallotStateINIT, ballot.State())
		require.Equal(t, common.VotingYES, ballot.Vote())
	}

	nr.TransitISAACState(nr.isaacStateManager.State().round, common.BallotStateACCEPT)
	<-recv
	require.Equal(t, 2, len(b.Messages))
	for _, message := range b.Messages {
		ballot, ok := message.(Ballot)
		require.True(t, ok)
		require.Equal(t, nr.localNode.Address(), ballot.Proposer())
		require.Equal(t, common.BallotStateINIT, ballot.State())
		require.Equal(t, common.VotingYES, ballot.Vote())
	}
}