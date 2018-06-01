package sebak

import (
	"context"
	"errors"
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	"github.com/owlchain/sebak/lib/common"
	"github.com/owlchain/sebak/lib/error"
	"github.com/owlchain/sebak/lib/network"
	"github.com/stellar/go/keypair"
)

func CheckTransactionSource(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	if _, err := keypair.Parse(target.(Transaction).B.Source); err != nil {
		return ctx, sebakerror.ErrorBadPublicAddress
	}

	return ctx, nil
}

func CheckTransactionBaseFee(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	if int64(target.(Transaction).B.Fee) < BaseFee {
		return ctx, sebakerror.ErrorInvalidFee
	}

	return ctx, nil
}

func CheckTransactionOperation(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	tx := target.(Transaction)
	networkID := ctx.Value("networkID").([]byte)

	var hashes []string
	for _, op := range tx.B.Operations {
		if tx.B.Source == op.B.TargetAddress() {
			return ctx, sebakerror.ErrorInvalidOperation
		}
		if err := op.IsWellFormed(networkID); err != nil {
			return ctx, err
		}
		// if there are multiple operations which has same 'Type' and same
		// 'TargetAddress()', this transaction will be invalid.
		u := fmt.Sprintf("%s-%s", op.H.Type, op.B.TargetAddress())
		if _, found := sebakcommon.InStringArray(hashes, u); found {
			return ctx, sebakerror.ErrorDuplicatedOperation
		}

		hashes = append(hashes, u)
	}

	return ctx, nil
}

func CheckTransactionVerifySignature(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	tx := target.(Transaction)

	kp, err := keypair.Parse(tx.B.Source)
	if err != nil {
		return ctx, err
	}
	networkID := ctx.Value("networkID").([]byte)
	err = kp.Verify(append(networkID, []byte(tx.H.Hash)...), base58.Decode(tx.H.Signature))
	if err != nil {
		return ctx, err
	}
	return ctx, nil
}

func CheckTransactionHashMatch(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	tx := target.(Transaction)
	if tx.H.Hash != tx.B.MakeHashString() {
		return ctx, sebakerror.ErrorHashDoesNotMatch
	}

	return ctx, nil
}

func CheckNodeRunnerHandleMessageTransactionUnmarshal(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	message, ok := args[0].(sebaknetwork.Message)
	if !ok {
		return ctx, errors.New("found invalid transaction message")
	}

	var tx Transaction
	var err error
	if tx, err = NewTransactionFromJSON(message.Data); err != nil {
		return ctx, err
	}

	networkID := ctx.Value("networkID").([]byte)
	if err = tx.IsWellFormed(networkID); err != nil {
		return ctx, err
	}

	nr := target.(*NodeRunner)
	nr.Log().Debug("message is transaction")
	return context.WithValue(ctx, "transaction", tx), nil
}

func CheckNodeRunnerHandleMessageHistory(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	message, _ := args[0].(sebaknetwork.Message)
	tx := ctx.Value("transaction").(Transaction)

	nr := target.(*NodeRunner)
	bt := NewTransactionHistoryFromTransaction(tx, message.Data)
	if err := bt.Save(nr.Storage()); err != nil {
		return ctx, err
	}

	nr.Log().Debug("saved in history", "transction", tx.GetHash())
	return ctx, nil
}

func CheckNodeRunnerHandleMessageISAACReceiveMessage(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	nr := target.(*NodeRunner)
	tx := ctx.Value("transaction").(Transaction)

	var err error
	var ballot Ballot
	if ballot, err = nr.consensus.ReceiveMessage(tx); err != nil {
		return ctx, err
	}

	return context.WithValue(ctx, "ballot", ballot), nil
}

func CheckNodeRunnerHandleMessageSignBallot(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	nr := target.(*NodeRunner)
	ballot := ctx.Value("ballot").(Ballot)
	currentNode := ctx.Value("currentNode").(*sebakcommon.Validator)

	// self-sign
	ballot.Vote(VotingYES)
	ballot.UpdateHash()
	ballot.Sign(currentNode.Keypair(), nr.NetworkID())

	return context.WithValue(ctx, "ballot", ballot), nil
}

func CheckNodeRunnerHandleMessageBroadcast(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	nr := target.(*NodeRunner)
	ballot := ctx.Value("ballot").(Ballot)

	nr.Log().Debug("ballot from client will be broadcasted", "ballot", ballot.MessageHash())
	nr.ConnectionManager().Broadcast(ballot)

	return ctx, nil
}

// TODO check the ballot from known validators

func CheckNodeRunnerHandleBallotIsWellformed(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	message, ok := args[0].(sebaknetwork.Message)
	if !ok {
		return ctx, errors.New("found invalid transaction message")
	}

	var err error
	var ballot Ballot
	if ballot, err = NewBallotFromJSON(message.Data); err != nil {
		return ctx, err
	}

	return context.WithValue(ctx, "ballot", ballot), nil
}

func CheckNodeRunnerHandleBallotCheckIsNew(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	nr := target.(*NodeRunner)
	ballot, ok := ctx.Value("ballot").(Ballot)
	if !ok {
		return ctx, errors.New("found invalid ballot")
	}

	isNew := !nr.consensus.HasMessageByHash(ballot.MessageHash())

	return context.WithValue(ctx, "isNew", isNew), nil
}

func CheckNodeRunnerHandleBallotReceiveBallot(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	nr := target.(*NodeRunner)
	ballot, ok := ctx.Value("ballot").(Ballot)
	if !ok {
		return ctx, errors.New("found invalid ballot")
	}

	var err error
	var vs VotingStateStaging
	if vs, err = nr.consensus.ReceiveBallot(ballot); err != nil {
		return ctx, err
	}

	return context.WithValue(ctx, "vs", vs), nil
}

func CheckNodeRunnerHandleBallotHistory(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	isNew := ctx.Value("isNew").(bool)
	if !isNew {
		return ctx, nil
	}

	ballot, _ := ctx.Value("ballot").(Ballot)
	tx := ballot.Data().Data.(Transaction)
	raw, err := ballot.Data().Serialize()
	if err != nil {
		return ctx, err
	}

	nr := target.(*NodeRunner)
	bt := NewTransactionHistoryFromTransaction(tx, raw)
	if err := bt.Save(nr.Storage()); err != nil {
		return ctx, err
	}

	nr.Log().Debug("saved in history from ballot", "transction", tx.GetHash())
	return ctx, nil
}

func CheckNodeRunnerHandleBallotStore(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	vs, ok := ctx.Value("vs").(VotingStateStaging)
	if !ok {
		return ctx, nil
	}

	if !vs.IsStorable() || !vs.IsClosed() {
		return ctx, nil
	}

	ballot, _ := ctx.Value("ballot").(Ballot)
	tx := ballot.Data().Data.(Transaction)
	nr := target.(*NodeRunner)
	if err := FinishTransaction(nr.Storage(), ballot, tx); err != nil {
		return ctx, err
	}

	nr.Log().Debug("got consensus", "ballot", ballot.MessageHash(), "votingResultStaging", vs)

	return ctx, sebakcommon.CheckerErrorStop{"got consensus"}
}

func CheckNodeRunnerHandleBallotIsBroadcastable(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	vs := ctx.Value("vs").(VotingStateStaging)
	if vs.IsClosed() {
		return ctx, sebakcommon.CheckerErrorStop{"VotingResult is already closed"}
	}

	var willBroadcast bool

	isNew := ctx.Value("isNew").(bool)
	if isNew || vs.IsChanged() {
		willBroadcast = true
	}

	ctx = context.WithValue(ctx, "willBroadcast", willBroadcast)
	return ctx, nil
}

func CheckNodeRunnerHandleBallotVotingHole(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	// validate transaction, so decide say VotingYES or VotingNO

	// checkpoint is valid in BallotStateSIGN
	vs := ctx.Value("vs").(VotingStateStaging)
	if !vs.IsChanged() {
		return ctx, nil
	}
	if vs.State != sebakcommon.BallotStateSIGN {
		return ctx, nil
	}

	nr := target.(*NodeRunner)
	ballot, _ := ctx.Value("ballot").(Ballot)
	tx := ballot.Data().Data.(Transaction)

	votingHole := VotingYES

	if tx.B.Fee < Amount(BaseFee) {
		votingHole = VotingNO
	}

	// NOTE(CheckNodeRunnerHandleBallotVotingHole): if BlockTransaction was
	// not found by tx.B.Checkpoint, it will be genesis block.
	if votingHole == VotingYES {
		bt, err := GetBlockTransactionByCheckpoint(nr.Storage(), tx.B.Checkpoint)
		if err == nil {
			// compare the stored BlockTransaction with tx
			if votingHole == VotingYES && tx.B.Source != bt.Source {
				votingHole = VotingNO
			}
		}
	}

	if votingHole == VotingYES {
		if ba, err := GetBlockAccount(nr.Storage(), tx.B.Source); err != nil {
			votingHole = VotingNO
		} else if tx.B.Checkpoint != ba.Checkpoint {
			votingHole = VotingNO
		} else if tx.TotalAmount(true) > MustAmountFromString(ba.Balance) {
			votingHole = VotingNO
		}
	}
	ctx = context.WithValue(ctx, "votingHole", votingHole)

	return ctx, nil
}

func CheckNodeRunnerHandleBallotBroadcast(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
	var willBroadcast bool

	nr := target.(*NodeRunner)
	ballot, _ := ctx.Value("ballot").(Ballot)
	vs := ctx.Value("vs").(VotingStateStaging)
	isNew := ctx.Value("isNew").(bool)
	if vs.IsClosed() {
		if err := nr.Consensus().CloseConsensus(ballot); err != nil {
			nr.Log().Error("failed to close consensus", "error", err)
		}
		return ctx, sebakcommon.CheckerErrorStop{"VotingResult is already closed"}
	} else if vs.IsChanged() {
		willBroadcast = true
	} else if isNew {
		willBroadcast = true
	}

	if !willBroadcast {
		return ctx, nil
	}

	var newBallot Ballot
	newBallot = ballot.Clone()

	state := ballot.State()
	votingHole := ballot.B.VotingHole
	if vs.IsChanged() {
		state = vs.State
		votingHole = vs.VotingHole
	}

	if vt, ok := ctx.Value("votingHole").(VotingHole); ok {
		votingHole = vt
	}

	newBallot.SetState(state)
	newBallot.Vote(votingHole)
	newBallot.Sign(nr.Node().Keypair(), nr.NetworkID())

	nr.Consensus().AddBallot(newBallot)

	nr.Log().Debug("ballot will be broadcasted", "ballot", newBallot.MessageHash(), "isNew", isNew)
	nr.ConnectionManager().Broadcast(newBallot)

	return ctx, nil
}
