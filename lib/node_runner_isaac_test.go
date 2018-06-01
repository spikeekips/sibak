package sebak

import (
	"context"
	"sync"
	"testing"

	"github.com/owlchain/sebak/lib/common"
	"github.com/owlchain/sebak/lib/error"
	"github.com/owlchain/sebak/lib/network"
)

// TestNodeRunnerConsensus checks, all the validators can get consensus.
func TestNodeRunnerConsensus(t *testing.T) {
	defer sebaknetwork.CleanUpMemoryNetwork()

	numberOfNodes := 3
	nodeRunners := createNodeRunnersWithReady(numberOfNodes)
	for _, nr := range nodeRunners {
		defer nr.Stop()
	}

	var wg sync.WaitGroup

	wg.Add(numberOfNodes)

	var saved []string
	var handleBallotCheckerFuncs = []sebakcommon.CheckerFunc{
		CheckNodeRunnerHandleBallotIsWellformed,
		CheckNodeRunnerHandleBallotCheckIsNew,
		CheckNodeRunnerHandleBallotReceiveBallot,
		CheckNodeRunnerHandleBallotHistory,
		//CheckNodeRunnerHandleBallotStore,
		func(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
			vs, ok := ctx.Value("vs").(VotingStateStaging)
			if !ok {
				return ctx, nil
			}

			if !vs.IsStorable() || !vs.IsClosed() {
				return ctx, nil
			}

			nr := target.(*NodeRunner)
			ballot, _ := ctx.Value("ballot").(Ballot)
			if _, found := sebakcommon.InStringArray(saved, ballot.MessageHash()); found {
				nr.Log().Error("", "error", sebakerror.ErrorAlreadySaved)
				return ctx, sebakcommon.CheckerErrorStop{"got consensus"}
			}

			saved = append(saved, ballot.MessageHash())
			nr.Log().Debug("got consensus", "ballot", ballot.MessageHash(), "votingResultStaging", vs)

			return ctx, sebakcommon.CheckerErrorStop{"got consensus"}
		},
		CheckNodeRunnerHandleBallotIsBroadcastable,
		func(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
			return ctx, nil
		},
		CheckNodeRunnerHandleBallotBroadcast,
	}

	var dones []VotingStateStaging
	var deferFunc sebakcommon.DeferFunc = func(n int, f sebakcommon.CheckerFunc, ctx context.Context, err error) {
		if err == nil {
			return
		}

		if vs, ok := ctx.Value("vs").(VotingStateStaging); ok && vs.IsClosed() {
			if _, ok := err.(sebakcommon.CheckerErrorStop); ok {
				dones = append(dones, vs)
				wg.Done()
			}
		}

	}

	ctx := context.WithValue(context.Background(), "deferFunc", deferFunc)
	for _, nr := range nodeRunners {
		nr.SetHandleBallotCheckerFuncs(ctx, handleBallotCheckerFuncs...)
	}

	nr0 := nodeRunners[0]

	client := nr0.Network().GetClient(nr0.Node().Endpoint())

	tx := makeTransaction(nr0.Node().Keypair())
	client.SendMessage(tx)

	wg.Wait()

	for _, done := range dones {
		if done.State != sebakcommon.BallotStateALLCONFIRM {
			t.Error("failed to get consensus")
			return
		}
		if done.MessageHash != tx.GetHash() {
			t.Error("failed to get consensus; found invalid message")
			return
		}
	}
}

// TestNodeRunnerConsensusWithVotingNO checks, consensus will be ended with
// VotingNO over threshold.
func TestNodeRunnerConsensusWithVotingNO(t *testing.T) {
	defer sebaknetwork.CleanUpMemoryNetwork()

	numberOfNodes := 3
	nodeRunners := createNodeRunnersWithReady(numberOfNodes)

	for _, nr := range nodeRunners {
		defer nr.Stop()
	}

	for _, nr := range nodeRunners {
		nr.Policy().Reset(sebakcommon.BallotStateINIT, 100)
	}

	say_no_validators := []string{
		//nodeRunners[0].Node().Address(),
		nodeRunners[1].Node().Address(),
		nodeRunners[2].Node().Address(),
	}

	var wg sync.WaitGroup
	wg.Add(numberOfNodes)

	var handleBallotCheckerFuncs = []sebakcommon.CheckerFunc{
		CheckNodeRunnerHandleBallotIsWellformed,
		CheckNodeRunnerHandleBallotCheckIsNew,
		CheckNodeRunnerHandleBallotReceiveBallot,
		// this will manipulate the VotingHole for `say_no_validators`
		func(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
			nr := target.(*NodeRunner)
			if _, found := sebakcommon.InStringArray(say_no_validators, nr.Node().Address()); !found {
				return ctx, nil
			}

			ballot := ctx.Value("ballot").(Ballot)
			if ballot.State() != sebakcommon.BallotStateINIT {
				return ctx, nil
			}

			ballot.Vote(VotingNO)
			ballot.Sign(nr.Node().Keypair(), nr.NetworkID())

			ctx = context.WithValue(ctx, "ballot", ballot)

			return ctx, nil
		},

		CheckNodeRunnerHandleBallotStore,
		CheckNodeRunnerHandleBallotBroadcast,
	}

	var dones []VotingStateStaging
	var deferFunc sebakcommon.DeferFunc = func(n int, f sebakcommon.CheckerFunc, ctx context.Context, err error) {
		if err == nil {
			return
		}

		if vs, ok := ctx.Value("vs").(VotingStateStaging); ok && vs.IsClosed() {
			if _, ok := err.(sebakcommon.CheckerErrorStop); ok {
				dones = append(dones, vs)
				wg.Done()
			}
		}

	}

	ctx := context.WithValue(context.Background(), "deferFunc", deferFunc)
	for _, nr := range nodeRunners {
		nr.SetHandleBallotCheckerFuncs(ctx, handleBallotCheckerFuncs...)
	}
	nr0 := nodeRunners[0]

	client := nr0.Network().GetClient(nr0.Node().Endpoint())

	tx := makeTransaction(nr0.Node().Keypair())
	client.SendMessage(tx)

	wg.Wait()

	for _, vs := range dones {
		if vs.State != sebakcommon.BallotStateINIT {
			t.Error("the final state must be `BallotStateINIT`.")
			return
		}
		if vs.VotingHole != VotingNO {
			t.Error("the final VotingHole must be `VotingNO`.")
			return
		}
	}
}

// TestNodeRunnerConsensusStoreInHistoryIncomingTxMessage checks, the incoming tx message will be
// saved in 'BlockTransactionHistory'.
func TestNodeRunnerConsensusStoreInHistoryIncomingTxMessage(t *testing.T) {
	defer sebaknetwork.CleanUpMemoryNetwork()

	numberOfNodes := 3
	nodeRunners := createNodeRunnersWithReady(numberOfNodes)

	var wg sync.WaitGroup
	wg.Add(1)

	var handleMessageFromClientCheckerFuncs = []sebakcommon.CheckerFunc{
		CheckNodeRunnerHandleMessageTransactionUnmarshal,
		CheckNodeRunnerHandleMessageHistory,
		func(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
			defer wg.Done()

			return ctx, nil
		},
	}

	for _, nr := range nodeRunners {
		nr.SetHandleMessageFromClientCheckerFuncs(nil, handleMessageFromClientCheckerFuncs...)
	}

	nr0 := nodeRunners[0]

	client := nr0.Network().GetClient(nr0.Node().Endpoint())
	tx := makeTransaction(nr0.Node().Keypair())
	client.SendMessage(tx)

	wg.Wait()

	if nr0.Consensus().HasMessageByHash(tx.GetHash()) {
		t.Error("failed to close consensus; message still in consensus")
		return
	}

	{
		history, err := GetBlockTransactionHistory(nr0.Storage(), tx.GetHash())
		if err != nil {
			t.Error(err)
			return
		}
		if history.Hash != tx.GetHash() {
			t.Error("saved invalid hash")
			return
		}
	}
}

// TestNodeRunnerConsensusStoreInHistoryNewBallot checks, the incoming new
// ballot will be saved in 'BlockTransactionHistory'.
func TestNodeRunnerConsensusStoreInHistoryNewBallot(t *testing.T) {
	defer sebaknetwork.CleanUpMemoryNetwork()

	numberOfNodes := 3
	nodeRunners := createNodeRunnersWithReady(numberOfNodes)

	var wg sync.WaitGroup
	wg.Add(2)

	var handleBallotCheckerFuncs = []sebakcommon.CheckerFunc{
		CheckNodeRunnerHandleBallotIsWellformed,
		CheckNodeRunnerHandleBallotCheckIsNew,
		CheckNodeRunnerHandleBallotReceiveBallot,
		CheckNodeRunnerHandleBallotHistory,
		func(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
			if !ctx.Value("isNew").(bool) {
				return ctx, nil
			}

			wg.Done()
			return ctx, nil
		},
	}

	for _, nr := range nodeRunners {
		nr.SetHandleBallotCheckerFuncs(nil, handleBallotCheckerFuncs...)
	}

	nr0 := nodeRunners[0]

	client := nr0.Network().GetClient(nr0.Node().Endpoint())

	tx := makeTransaction(nr0.Node().Keypair())
	client.SendMessage(tx)

	wg.Wait()

	for _, nr := range nodeRunners {
		if nr.Node() == nr0.Node() {
			continue
		}

		history, err := GetBlockTransactionHistory(nr.Storage(), tx.GetHash())
		if err != nil {
			t.Error(err)
			return
		}
		if history.Hash != tx.GetHash() {
			t.Error("saved invalid hash")
			return
		}
	}
}
