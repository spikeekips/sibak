package common

const (
	BlockPrefixHash                       = string(0x00)
	BlockPrefixConfirmed                  = string(0x01)
	BlockPrefixHeight                     = string(0x02)
	BlockPrefixTime                       = string(0x03)
	BlockTransactionPrefixHash            = string(0x10)
	BlockTransactionPrefixSource          = string(0x11)
	BlockTransactionPrefixConfirmed       = string(0x12)
	BlockTransactionPrefixAccount         = string(0x13)
	BlockTransactionPrefixBlock           = string(0x14)
	BlockOperationPrefixHash              = string(0x20)
	BlockOperationPrefixTxHash            = string(0x21)
	BlockOperationPrefixSource            = string(0x22)
	BlockOperationPrefixTarget            = string(0x23)
	BlockOperationPrefixPeers             = string(0x24)
	BlockOperationPrefixTypeSource        = string(0x25)
	BlockOperationPrefixTypeTarget        = string(0x26)
	BlockOperationPrefixTypePeers         = string(0x27)
	BlockOperationPrefixCreateFrozen      = string(0x28)
	BlockOperationPrefixFrozenLinked      = string(0x29)
	BlockAccountPrefixAddress             = string(0x30)
	BlockAccountPrefixCreated             = string(0x31)
	BlockAccountSequenceIDPrefix          = string(0x32)
	BlockAccountSequenceIDByAddressPrefix = string(0x33)
	BlockAccountPrefixFrozen              = string(0x34)
	TransactionPoolPrefix                 = string(0x40)
)
