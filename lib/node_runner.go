//
// Struct that bridges together components of a node
//
// NodeRunner bridges together the connection, storage and `LocalNode`.
// In this regard, it can be seen as a single node, and is used as such
// in unit tests.
//
package sebak

import (
	"time"

	"boscoin.io/sebak/lib/common"
	"boscoin.io/sebak/lib/network"
	"boscoin.io/sebak/lib/node"
	"boscoin.io/sebak/lib/storage"
	logging "github.com/inconshreveable/log15"
)

type NodeRunner struct {
	networkID         []byte
	localNode         *sebaknode.LocalNode
	policy            sebakcommon.VotingThresholdPolicy
	network           sebaknetwork.Network
	consensus         Consensus
	connectionManager *sebaknetwork.ConnectionManager
	storage           *sebakstorage.LevelDBBackend

	handleTransactionCheckerFuncs []sebakcommon.CheckerFunc
	handleBallotCheckerFuncs      []sebakcommon.CheckerFunc

	handleTransactionCheckerDeferFunc sebakcommon.CheckerDeferFunc
	handleBallotCheckerDeferFunc      sebakcommon.CheckerDeferFunc

	log logging.Logger
}

func NewNodeRunner(
	networkID string,
	localNode *sebaknode.LocalNode,
	policy sebakcommon.VotingThresholdPolicy,
	network sebaknetwork.Network,
	consensus Consensus,
	storage *sebakstorage.LevelDBBackend,
) *NodeRunner {
	nr := &NodeRunner{
		networkID: []byte(networkID),
		localNode: localNode,
		policy:    policy,
		network:   network,
		consensus: consensus,
		storage:   storage,
		log:       log.New(logging.Ctx{"node": localNode.Alias()}),
	}

	nr.connectionManager = sebaknetwork.NewConnectionManager(
		nr.localNode,
		nr.network,
		nr.policy,
		nr.localNode.GetValidators(),
	)
	nr.network.AddWatcher(nr.connectionManager.ConnectionWatcher)

	nr.SetHandleTransactionCheckerFuncs(nil, DefaultHandleTransactionCheckerFuncs...)
	nr.SetHandleBallotCheckerFuncs(nil, DefaultHandleBallotCheckerFuncs...)

	return nr
}

func (nr *NodeRunner) Ready() {
	nodeHandler := NetworkHandlerNode{
		localNode: nr.localNode,
		network:   nr.network,
	}

	nr.network.AddHandler(sebaknetwork.UrlPathPrefixNode+"/", nodeHandler.NodeInfoHandler)
	nr.network.AddHandler(sebaknetwork.UrlPathPrefixNode+"/connect", nodeHandler.ConnectHandler)
	nr.network.AddHandler(sebaknetwork.UrlPathPrefixNode+"/message", nodeHandler.MessageHandler)
	nr.network.AddHandler(sebaknetwork.UrlPathPrefixNode+"/ballot", nodeHandler.BallotHandler)

	apiHandler := NetworkHandlerAPI{
		localNode: nr.localNode,
		network:   nr.network,
		storage:   nr.storage,
	}

	nr.network.AddHandler(
		sebaknetwork.UrlPathPrefixAPI+GetAccountHandlerPattern,
		apiHandler.GetAccountHandler,
	).Methods("GET")
	nr.network.AddHandler(
		sebaknetwork.UrlPathPrefixAPI+GetAccountTransactionsHandlerPattern,
		apiHandler.GetAccountTransactionsHandler,
	).Methods("GET")
	nr.network.AddHandler(
		sebaknetwork.UrlPathPrefixAPI+GetAccountOperationsHandlerPattern,
		apiHandler.GetAccountOperationsHandler,
	).Methods("GET")
	nr.network.AddHandler(
		sebaknetwork.UrlPathPrefixAPI+GetTransactionsHandlerPattern,
		apiHandler.GetTransactionsHandler,
	).Methods("GET")
	nr.network.AddHandler(
		sebaknetwork.UrlPathPrefixAPI+GetTransactionByHashHandlerPattern,
		apiHandler.GetTransactionByHashHandler,
	).Methods("GET")

	nr.network.Ready()
}

func (nr *NodeRunner) Start() (err error) {
	nr.Ready()

	go nr.handleMessage()
	go nr.ConnectValidators()

	if err = nr.network.Start(); err != nil {
		return
	}

	return
}

func (nr *NodeRunner) Stop() {
	nr.network.Stop()
}

func (nr *NodeRunner) Node() *sebaknode.LocalNode {
	return nr.localNode
}

func (nr *NodeRunner) NetworkID() []byte {
	return nr.networkID
}

func (nr *NodeRunner) Network() sebaknetwork.Network {
	return nr.network
}

func (nr *NodeRunner) Consensus() Consensus {
	return nr.consensus
}

func (nr *NodeRunner) ConnectionManager() *sebaknetwork.ConnectionManager {
	return nr.connectionManager
}

func (nr *NodeRunner) Storage() *sebakstorage.LevelDBBackend {
	return nr.storage
}

func (nr *NodeRunner) Policy() sebakcommon.VotingThresholdPolicy {
	return nr.policy
}

func (nr *NodeRunner) Log() logging.Logger {
	return nr.log
}

func (nr *NodeRunner) ConnectValidators() {
	ticker := time.NewTicker(time.Millisecond * 5)
	for _ = range ticker.C {
		if !nr.network.IsReady() {
			continue
		}

		ticker.Stop()
		break
	}
	nr.log.Debug("current node is ready")
	nr.log.Debug("trying to connect to the validators", "validators", nr.localNode.GetValidators())

	nr.log.Debug("initializing connectionManager for validators")
	nr.connectionManager.Start()
}

var DefaultHandleTransactionCheckerFuncs = []sebakcommon.CheckerFunc{
	CheckNodeRunnerHandleMessageTransactionUnmarshal,
	CheckNodeRunnerHandleMessageTransactionHasSameSource,
	CheckNodeRunnerHandleMessageHistory,
	CheckNodeRunnerHandleMessageISAACReceiveMessage,
	CheckNodeRunnerHandleMessageSignBallot,
	CheckNodeRunnerHandleMessageBroadcast,
}

var DefaultHandleBallotCheckerFuncs = []sebakcommon.CheckerFunc{
	CheckNodeRunnerHandleBallotIsWellformed,
	CheckNodeRunnerHandleBallotNotFromKnownValidators,
	CheckNodeRunnerHandleBallotCheckIsNew,
	CheckNodeRunnerHandleBallotReceiveBallot,
	CheckNodeRunnerHandleBallotHistory,
	CheckNodeRunnerHandleBallotStore,
	CheckNodeRunnerHandleBallotIsBroadcastable,
	CheckNodeRunnerHandleBallotVotingHole,
	CheckNodeRunnerHandleBallotBroadcast,
}

func (nr *NodeRunner) SetHandleTransactionCheckerFuncs(
	deferFunc sebakcommon.CheckerDeferFunc,
	f ...sebakcommon.CheckerFunc,
) {
	if len(f) > 0 {
		nr.handleTransactionCheckerFuncs = f
	}

	if deferFunc == nil {
		deferFunc = sebakcommon.DefaultDeferFunc
	}

	nr.handleTransactionCheckerDeferFunc = deferFunc
}

func (nr *NodeRunner) SetHandleBallotCheckerFuncs(
	deferFunc sebakcommon.CheckerDeferFunc,
	f ...sebakcommon.CheckerFunc,
) {
	if len(f) > 0 {
		nr.handleBallotCheckerFuncs = f
	}

	if deferFunc == nil {
		deferFunc = sebakcommon.DefaultDeferFunc
	}

	nr.handleBallotCheckerDeferFunc = deferFunc
}

func (nr *NodeRunner) SetHandleBallotCheckerDeferFuncs(deferFunc sebakcommon.CheckerDeferFunc) {
	if deferFunc == nil {
		deferFunc = sebakcommon.DefaultDeferFunc
	}

	nr.handleBallotCheckerDeferFunc = deferFunc
}

func (nr *NodeRunner) handleMessage() {
	var err error
	for message := range nr.network.ReceiveMessage() {
		switch message.Type {
		case sebaknetwork.ConnectMessage:
			if _, err := sebaknode.NewValidatorFromString(message.Data); err != nil {
				nr.log.Error("invalid validator data was received", "data", message.Data)
				continue
			}
		case sebaknetwork.TransactionMessage:
			if message.IsEmpty() {
				nr.log.Error("got empty transaction`")
				continue
			}

			nr.log.Debug("got transaction`", "message", message.Head(50))

			checker := &NodeRunnerHandleMessageChecker{
				DefaultChecker: sebakcommon.DefaultChecker{Funcs: nr.handleTransactionCheckerFuncs},
				NodeRunner:     nr,
				LocalNode:      nr.localNode,
				NetworkID:      nr.networkID,
				Message:        message,
			}

			if err = sebakcommon.RunChecker(checker, nr.handleTransactionCheckerDeferFunc); err != nil {
				if _, ok := err.(sebakcommon.CheckerErrorStop); ok {
					continue
				}
				nr.log.Error("failed to handle transaction", "error", err)
				continue
			}
		case sebaknetwork.BallotMessage:
			if message.IsEmpty() {
				nr.log.Error("got empty ballot message`")
				continue
			}
			nr.log.Debug("got ballot", "message", message.Head(50))

			checker := &NodeRunnerHandleBallotChecker{
				DefaultChecker: sebakcommon.DefaultChecker{Funcs: nr.handleBallotCheckerFuncs},
				NodeRunner:     nr,
				LocalNode:      nr.localNode,
				NetworkID:      nr.networkID,
				Message:        message,
				VotingHole:     VotingNOTYET,
			}
			if err = sebakcommon.RunChecker(checker, nr.handleBallotCheckerDeferFunc); err != nil {
				if _, ok := err.(sebakcommon.CheckerErrorStop); !ok {
					nr.log.Error("failed to handle ballot", "error", err)
				}
			}
			nr.closeConsensus(checker)
		default:
			nr.log.Error("got unknown", "message", message.Head(50))
		}
	}
}

func (nr *NodeRunner) closeConsensus(c sebakcommon.Checker) (err error) {
	checker := c.(*NodeRunnerHandleBallotChecker)

	if checker.VotingStateStaging.IsEmpty() {
		return
	}
	if !checker.VotingStateStaging.IsClosed() {
		return
	}

	if err = nr.Consensus().CloseConsensus(checker.Ballot); err != nil {
		nr.Log().Error("new failed to close consensus", "error", err)
		return
	}

	nr.Log().Debug("consensus closed")
	return
}
