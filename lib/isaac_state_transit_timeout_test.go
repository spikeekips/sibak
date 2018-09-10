// We can test that a node broadcast propose ballot or B(`EXP`) in ISAACStateManager.
// when the timeout is expired,
package sebak

import (
	"testing"
	"time"

	"boscoin.io/sebak/lib/common"

	"github.com/stretchr/testify/require"
)

// 1. All 3 Nodes.
// 2. Proposer itself.
// 3. When `ISAACStateManager` starts, the node proposes ballot to validators.
func TestStateINITProposer(t *testing.T) {
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
	conf.TimeoutACCEPT = time.Hour
	conf.TimeoutALLCONFIRM = time.Hour

	nr.SetConf(conf)

	nr.StartStateManager()

	<-recv
	require.Equal(t, 1, len(b.Messages))
	for _, message := range b.Messages {
		// This message must be proposed ballot
		ballot, ok := message.(Ballot)
		require.True(t, ok)
		require.Equal(t, nr.localNode.Address(), ballot.Proposer())
	}
}

// 1. All 3 Nodes.
// 2. Not proposer itself.
// 3. When `ISAACStateManager` starts, the node waits a ballot by proposer.
// 4. But TimeoutINIT is an hour, so it doesn't broadcast anything.
func TestStateINITNotProposer(t *testing.T) {
	nodeRunners := createTestNodeRunner(3)

	nr := nodeRunners[0]

	b := NewTestBroadcastor(nil)
	nr.SetBroadcastor(b)
	nr.SetProposerCalculator(TheOtherProposerCalculator{})

	nr.Consensus().SetLatestConsensusedBlock(genesisBlock)

	conf := NewISAACConfiguration()
	conf.TimeoutINIT = time.Hour
	conf.TimeoutSIGN = time.Hour
	conf.TimeoutACCEPT = time.Hour
	conf.TimeoutALLCONFIRM = time.Hour

	nr.SetConf(conf)

	nr.StartStateManager()
	time.Sleep(1 * time.Second)

	require.Equal(t, 0, len(b.Messages))
}

// 1. All 3 Nodes.
// 2. Not proposer itself.
// 3. When `ISAACStateManager` starts, the node waits a ballot by proposer.
// 4. But TimeoutINIT is a millisecond.
// 5. After timeout, the node broadcasts B(`SIGN`, `EXP`)
func TestStateINITTimeoutNotProposer(t *testing.T) {
	nodeRunners := createTestNodeRunner(3)

	nr := nodeRunners[0]

	recv := make(chan struct{})
	b := NewTestBroadcastor(recv)
	nr.SetBroadcastor(b)
	nr.SetProposerCalculator(TheOtherProposerCalculator{})
	proposer := nr.CalculateProposer(0, 0)

	require.NotEqual(t, nr.localNode.Address(), proposer)

	nr.Consensus().SetLatestConsensusedBlock(genesisBlock)

	conf := NewISAACConfiguration()
	conf.TimeoutINIT = 1 * time.Millisecond
	conf.TimeoutSIGN = time.Hour
	conf.TimeoutACCEPT = time.Hour
	conf.TimeoutALLCONFIRM = time.Hour

	nr.SetConf(conf)

	nr.StartStateManager()
	require.Equal(t, common.BallotStateINIT, nr.isaacStateManager.state.ballotState)

	<-recv
	require.Equal(t, common.BallotStateSIGN, nr.isaacStateManager.state.ballotState)
	require.Equal(t, 1, len(b.Messages))
	init, sign, accept := 0, 0, 0
	for _, message := range b.Messages {
		ballot, ok := message.(Ballot)
		require.True(t, ok)
		require.Equal(t, 0, len(ballot.Transactions()))
		switch ballot.State() {
		case common.BallotStateINIT:
			init++
		case common.BallotStateSIGN:
			sign++
			require.Equal(t, common.VotingEXP, ballot.Vote())
		case common.BallotStateACCEPT:
			accept++
		}
	}
	require.Equal(t, 0, init)
	require.Equal(t, 1, sign)
	require.Equal(t, 0, accept)
}

// 1. All 3 Nodes.
// 2. Proposer itself.
// 3. When `ISAACStateManager` starts, the node proposes B(`INIT`, `YES`) to validators.
// 4. Then ISAACState will be changed to `SIGN`.
// 5. But TimeoutSIGN is a millisecond.
// 6. After timeout, the node broadcasts B(`ACCEPT`, `EXP`)
func TestStateSIGNTimeoutProposer(t *testing.T) {
	nodeRunners := createTestNodeRunner(3)

	nr := nodeRunners[0]

	recv := make(chan struct{})
	b := NewTestBroadcastor(recv)
	nr.SetBroadcastor(b)
	nr.SetProposerCalculator(SelfProposerCalculator{})
	proposer := nr.CalculateProposer(0, 0)

	require.Equal(t, nr.localNode.Address(), proposer)

	nr.Consensus().SetLatestConsensusedBlock(genesisBlock)

	conf := NewISAACConfiguration()
	conf.TimeoutINIT = time.Hour
	conf.TimeoutSIGN = time.Millisecond
	conf.TimeoutACCEPT = time.Hour
	conf.TimeoutALLCONFIRM = time.Hour

	nr.SetConf(conf)

	nr.StartStateManager()

	require.Equal(t, common.BallotStateINIT, nr.isaacStateManager.state.ballotState)

	<-recv
	require.Equal(t, 1, len(b.Messages))

	<-recv
	require.Equal(t, common.BallotStateACCEPT, nr.isaacStateManager.state.ballotState)

	require.Equal(t, 2, len(b.Messages))

	init, sign, accept := 0, 0, 0
	for _, message := range b.Messages {
		ballot, ok := message.(Ballot)
		require.True(t, ok)
		require.Equal(t, 0, len(ballot.Transactions()))
		switch ballot.State() {
		case common.BallotStateINIT:
			init++
			require.Equal(t, common.VotingYES, ballot.Vote())
		case common.BallotStateSIGN:
			sign++
			require.Equal(t, common.VotingEXP, ballot.Vote())
		case common.BallotStateACCEPT:
			accept++
			require.Equal(t, common.VotingEXP, ballot.Vote())
		}
	}
	require.Equal(t, 1, init)
	require.Equal(t, 0, sign)
	require.Equal(t, 1, accept)
}

// 1. All 3 Nodes.
// 2. Not proposer itself.
// 3. When `ISAACStateManager` starts, the node waits a ballot by proposer.
// 4. TimeoutINIT is a millisecond.
// 5. After milliseconds, the node broadcasts B(`SIGN`, `EXP`).
// 6. ISAACState is changed to `SIGN`.
// 7. TimeoutSIGN is a millisecond.
// 8. After timeout, the node broadcasts B(`ACCEPT`, `EXP`).
func TestStateSIGNTimeoutNotProposer(t *testing.T) {
	nodeRunners := createTestNodeRunner(3)

	nr := nodeRunners[0]

	recv := make(chan struct{})
	b := NewTestBroadcastor(recv)
	nr.SetBroadcastor(b)
	nr.SetProposerCalculator(TheOtherProposerCalculator{})
	proposer := nr.CalculateProposer(0, 0)

	require.NotEqual(t, nr.localNode.Address(), proposer)

	nr.Consensus().SetLatestConsensusedBlock(genesisBlock)

	conf := NewISAACConfiguration()
	conf.TimeoutINIT = time.Millisecond
	conf.TimeoutSIGN = time.Millisecond
	conf.TimeoutACCEPT = time.Hour
	conf.TimeoutALLCONFIRM = time.Hour

	nr.SetConf(conf)

	nr.StartStateManager()

	<-recv
	require.Equal(t, 1, len(b.Messages))

	<-recv
	require.Equal(t, 2, len(b.Messages))

	init, sign, accept := 0, 0, 0
	for _, message := range b.Messages {
		ballot, ok := message.(Ballot)
		require.True(t, ok)
		require.Equal(t, 0, len(ballot.Transactions()))
		switch ballot.State() {
		case common.BallotStateINIT:
			init++
			require.Equal(t, common.VotingYES, ballot.Vote())
		case common.BallotStateSIGN:
			sign++
			require.Equal(t, common.VotingEXP, ballot.Vote())
		case common.BallotStateACCEPT:
			accept++
			require.Equal(t, common.VotingEXP, ballot.Vote())
		}
	}
	require.Equal(t, 0, init)
	require.Equal(t, 1, sign)
	require.Equal(t, 1, accept)
}

// 1. All 3 Nodes.
// 2. Proposer itself at round 0.
// 3. When `ISAACStateManager` starts, the node proposes a ballot.
// 4. ISAACState is changed to `SIGN`.
// 5. TimeoutSIGN is a millisecond.
// 6. After timeout, the node broadcasts B(`ACCEPT`, `EXP`).
func TestStateACCEPTTimeoutProposerThenNotProposer(t *testing.T) {
	nodeRunners := createTestNodeRunner(3)

	nr := nodeRunners[0]

	recv := make(chan struct{})
	b := NewTestBroadcastor(recv)
	nr.SetBroadcastor(b)
	nr.SetProposerCalculator(&SelfProposerThenNotProposer{})

	proposer := nr.CalculateProposer(0, 0)
	require.Equal(t, nr.localNode.Address(), proposer)

	proposer = nr.CalculateProposer(0, 1)
	require.NotEqual(t, nr.localNode.Address(), proposer)

	nr.Consensus().SetLatestConsensusedBlock(genesisBlock)

	conf := NewISAACConfiguration()
	conf.TimeoutINIT = time.Hour
	conf.TimeoutSIGN = time.Millisecond
	conf.TimeoutACCEPT = time.Millisecond
	conf.TimeoutALLCONFIRM = time.Hour

	nr.SetConf(conf)

	nr.StartStateManager()

	<-recv
	require.Equal(t, 1, len(b.Messages))

	<-recv
	require.Equal(t, 2, len(b.Messages))
	init, sign, accept := 0, 0, 0
	for _, message := range b.Messages {
		ballot, ok := message.(Ballot)
		require.True(t, ok)
		require.Equal(t, 0, len(ballot.Transactions()))
		switch ballot.State() {
		case common.BallotStateINIT:
			init++
			require.Equal(t, ballot.Vote(), common.VotingYES)
		case common.BallotStateSIGN:
			sign++
		case common.BallotStateACCEPT:
			accept++
			require.Equal(t, ballot.Vote(), common.VotingEXP)
		}
	}

	require.Equal(t, 1, init)
	require.Equal(t, 0, sign)
	require.Equal(t, 1, accept)
}