package runner

import (
	logging "github.com/inconshreveable/log15"

	"boscoin.io/sebak/lib/ballot"
	"boscoin.io/sebak/lib/block"
	"boscoin.io/sebak/lib/errors"
	"boscoin.io/sebak/lib/metrics"
	"boscoin.io/sebak/lib/storage"
	"boscoin.io/sebak/lib/transaction"
	"boscoin.io/sebak/lib/transaction/operation"
)

func finishBallot(nr *NodeRunner, b ballot.Ballot, log logging.Logger) (*block.Block, []*transaction.Transaction, error) {
	var err error
	if err = insertMissingTransaction(nr, b); err != nil {
		log.Debug("failed to get the missing transactions of ballot", "error", err)
		return nil, nil, err
	}

	var bs *storage.LevelDBBackend
	if bs, err = nr.Storage().OpenBatch(); err != nil {
		return nil, nil, err
	}

	proposedTxs, err := getProposedTransactions(
		bs,
		b.B.Proposed.Transactions,
		nr.TransactionPool,
	)
	if err != nil {
		return nil, nil, err
	}

	var blk *block.Block
	blk, err = finishBallotWithProposedTxs(
		bs,
		b,
		proposedTxs,
		log,
		nr.Log(),
	)

	if err != nil {
		bs.Discard()
		log.Error("failed to finish ballot", "error", err)
		return nil, nil, err
	}

	if err = bs.Commit(); err != nil {
		if err != errors.NotCommittable {
			bs.Discard()
			return nil, nil, err
		}
	}

	return blk, proposedTxs, nil
}

func finishBallotWithProposedTxs(st *storage.LevelDBBackend, b ballot.Ballot, proposedTransactions []*transaction.Transaction, log, infoLog logging.Logger) (*block.Block, error) {
	var err error
	var isValid bool
	if isValid, err = isValidRound(st, b.VotingBasis(), infoLog); err != nil || !isValid {
		return nil, err
	}

	var nOps int
	for _, tx := range proposedTransactions {
		nOps += len(tx.B.Operations)
	}

	r := b.VotingBasis()
	r.Height++                                      // next block
	r.TotalTxs += uint64(len(b.Transactions()) + 1) // + 1 for ProposerTransaction
	r.TotalOps += uint64(nOps + len(b.ProposerTransaction().B.Operations))

	blk := block.NewBlock(
		b.Proposer(),
		r,
		b.ProposerTransaction().GetHash(),
		b.Transactions(),
		b.ProposerConfirmed(),
	)

	if err = blk.Save(st); err != nil {
		log.Error("failed to create new block", "block", blk, "error", err)
		return nil, err
	}

	log.Debug("NewBlock created", "block", blk)
	infoLog.Info("NewBlock created",
		"height", blk.Height,
		"round", blk.Round,
		"confirmed", blk.Confirmed,
		"total-txs", blk.TotalTxs,
		"total-ops", blk.TotalOps,
		"proposer", blk.Proposer,
	)
	metrics.Consensus.SetHeight(blk.Height)
	metrics.Consensus.SetRounds(blk.Round)
	metrics.Consensus.SetTotalTxs(blk.TotalTxs)
	metrics.Consensus.SetTotalOps(blk.TotalOps)

	if err = FinishTransactions(*blk, proposedTransactions, st); err != nil {
		return nil, err
	}

	if err = FinishProposerTransaction(st, *blk, b.ProposerTransaction(), log); err != nil {
		log.Error("failed to finish proposer transaction", "block", blk, "ptx", b.ProposerTransaction(), "error", err)
		return nil, err
	}

	return blk, nil
}

func getProposedTransactions(st *storage.LevelDBBackend, pTxHashes []string, transactionPool *transaction.Pool) ([]*transaction.Transaction, error) {
	proposedTransactions := make([]*transaction.Transaction, 0, len(pTxHashes))
	var err error
	for _, hash := range pTxHashes {
		tx, found := transactionPool.Get(hash)
		if !found {
			var tp block.TransactionPool
			if tp, err = block.GetTransactionPool(st, hash); err != nil {
				return nil, errors.TransactionNotFound
			}
			tx = tp.Transaction()
		}
		proposedTransactions = append(proposedTransactions, &tx)
	}
	return proposedTransactions, nil
}

func FinishTransactions(blk block.Block, transactions []*transaction.Transaction, st *storage.LevelDBBackend) (err error) {
	for _, tx := range transactions {
		bt := block.NewBlockTransactionFromTransaction(blk.Hash, blk.Height, blk.ProposedTime, *tx)
		if err = bt.Save(st); err != nil {
			return
		}
		for _, op := range tx.B.Operations {
			if err = finishOperation(st, tx.B.Source, op, log); err != nil {
				log.Error("failed to finish operation", "block", blk, "bt", bt, "op", op, "error", err)
				return err
			}
		}

		var baSource *block.BlockAccount
		if baSource, err = block.GetBlockAccount(st, tx.B.Source); err != nil {
			err = errors.BlockAccountDoesNotExists
			return
		}

		if err = baSource.Withdraw(tx.TotalAmount(true)); err != nil {
			return
		}

		if err = baSource.Save(st); err != nil {
			return
		}
	}

	return
}

// finishOperation do finish the task after consensus by the type of each operation.
func finishOperation(st *storage.LevelDBBackend, source string, op operation.Operation, log logging.Logger) (err error) {
	switch op.H.Type {
	case operation.TypeCreateAccount:
		pop, ok := op.B.(operation.CreateAccount)
		if !ok {
			return errors.UnknownOperationType
		}
		return finishCreateAccount(st, source, pop, log)
	case operation.TypePayment:
		pop, ok := op.B.(operation.Payment)
		if !ok {
			return errors.UnknownOperationType
		}
		return finishPayment(st, source, pop, log)
	case operation.TypeCongressVoting, operation.TypeCongressVotingResult:
		//Nothing to do
		return
	case operation.TypeUnfreezingRequest:
		pop, ok := op.B.(operation.UnfreezeRequest)
		if !ok {
			return errors.UnknownOperationType
		}
		return finishUnfreezeRequest(st, source, pop, log)
	case operation.TypeInflationPF:
		pop, ok := op.B.(operation.InflationPF)
		if !ok {
			return errors.UnknownOperationType
		}
		return finishInflationPF(st, source, pop, log)

	default:
		err = errors.UnknownOperationType
		return
	}
}

func finishCreateAccount(st *storage.LevelDBBackend, source string, op operation.CreateAccount, log logging.Logger) (err error) {
	if _, err = block.GetBlockAccount(st, source); err != nil {
		err = errors.BlockAccountDoesNotExists
		return
	}

	var baTarget *block.BlockAccount
	if baTarget, err = block.GetBlockAccount(st, op.TargetAddress()); err == nil {
		err = errors.BlockAccountAlreadyExists
		return
	} else {
		err = nil
	}

	baTarget = block.NewBlockAccountLinked(
		op.TargetAddress(),
		op.GetAmount(),
		op.Linked,
	)
	if err = baTarget.Save(st); err != nil {
		return
	}

	return
}

func finishPayment(st *storage.LevelDBBackend, source string, op operation.Payment, log logging.Logger) (err error) {
	if _, err = block.GetBlockAccount(st, source); err != nil {
		err = errors.BlockAccountDoesNotExists
		return
	}

	var baTarget *block.BlockAccount
	if baTarget, err = block.GetBlockAccount(st, op.TargetAddress()); err != nil {
		err = errors.BlockAccountDoesNotExists
		return
	}

	if err = baTarget.Deposit(op.GetAmount()); err != nil {
		return
	}
	if err = baTarget.Save(st); err != nil {
		return
	}

	return
}

func finishUnfreezeRequest(st *storage.LevelDBBackend, source string, opb operation.UnfreezeRequest, log logging.Logger) (err error) {
	return
}

func finishInflationPF(st *storage.LevelDBBackend, source string, opb operation.InflationPF, log logging.Logger) (err error) {

	if opb.Amount < 1 {
		return
	}

	var commonAccount *block.BlockAccount
	if commonAccount, err = block.GetBlockAccount(st, opb.FundingAddress); err != nil {
		return
	}

	if err = commonAccount.Deposit(opb.GetAmount()); err != nil {
		return
	}

	if err = commonAccount.Save(st); err != nil {
		return
	}

	return
}

func FinishProposerTransaction(st *storage.LevelDBBackend, blk block.Block, ptx ballot.ProposerTransaction, log logging.Logger) (err error) {
	{
		var opb operation.CollectTxFee
		if opb, err = ptx.CollectTxFee(); err != nil {
			return
		}
		if err = finishCollectTxFee(st, opb, log); err != nil {
			return
		}
	}

	{
		var opb operation.Inflation
		if opb, err = ptx.Inflation(); err != nil {
			return
		}
		if err = finishInflation(st, opb, log); err != nil {
			return
		}
	}

	bt := block.NewBlockTransactionFromTransaction(blk.Hash, blk.Height, blk.ProposedTime, ptx.Transaction)
	if err = bt.Save(st); err != nil {
		return
	}

	if _, err = block.SaveTransactionPool(st, ptx.Transaction); err != nil {
		return
	}
	return
}

func finishCollectTxFee(st *storage.LevelDBBackend, opb operation.CollectTxFee, log logging.Logger) (err error) {
	if opb.Amount < 1 {
		return
	}

	var commonAccount *block.BlockAccount
	if commonAccount, err = block.GetBlockAccount(st, opb.TargetAddress()); err != nil {
		return
	}

	if err = commonAccount.Deposit(opb.GetAmount()); err != nil {
		return
	}

	if err = commonAccount.Save(st); err != nil {
		return
	}

	return
}

func finishInflation(st *storage.LevelDBBackend, opb operation.Inflation, log logging.Logger) (err error) {
	if opb.Amount < 1 {
		return
	}

	var commonAccount *block.BlockAccount
	if commonAccount, err = block.GetBlockAccount(st, opb.TargetAddress()); err != nil {
		return
	}

	if err = commonAccount.Deposit(opb.GetAmount()); err != nil {
		return
	}

	if err = commonAccount.Save(st); err != nil {
		return
	}

	return
}
