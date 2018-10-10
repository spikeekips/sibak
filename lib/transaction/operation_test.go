package transaction

import (
	"testing"

	"boscoin.io/sebak/lib/common"

	"encoding/json"
	"github.com/stellar/go/keypair"
	"github.com/stretchr/testify/require"
)

func TestMakeHashOfOperationBodyPayment(t *testing.T) {
	kp := keypair.Master("find me")

	opb := OperationBodyPayment{
		Target: kp.Address(),
		Amount: common.Amount(100),
	}
	op := Operation{
		H: OperationHeader{Type: OperationPayment},
		B: opb,
	}
	hashed := op.MakeHashString()

	expected := "8AALKhfgCu2w3ZtbESXHG5ko93Jb1L1yCmFopoJubQh9"
	require.Equal(t, hashed, expected)
}

func TestIsWellFormedOperation(t *testing.T) {
	op := TestMakeOperation(-1)
	err := op.IsWellFormed(networkID)
	require.Nil(t, err)
}

func TestIsWellFormedOperationLowerAmount(t *testing.T) {
	obp := TestMakeOperationBodyPayment(0)
	err := obp.IsWellFormed(networkID)
	require.NotNil(t, err)
}

func TestSerializeOperation(t *testing.T) {
	op := TestMakeOperation(-1)
	b, err := op.Serialize()
	require.Nil(t, err)
	require.Equal(t, len(b) > 0, true)

	var o Operation
	err = json.Unmarshal(b, &o)
	require.Nil(t, err)
}

func TestOperationBodyCongressVoting(t *testing.T) {
	opb := OperationBodyCongressVoting{
		Contract: []byte("dummy contract"),
		Voting: struct {
			Start uint64
			End   uint64
		}{
			Start: 1,
			End:   100,
		},
	}
	op := Operation{
		H: OperationHeader{Type: OperationCongressVoting},
		B: opb,
	}
	hashed := op.MakeHashString()

	expected := "2skQu73zDSRvBF5CYhKkJLuK2QBqBkPDTcp3qAx7XvgA"
	require.Equal(t, hashed, expected)

	err := op.IsWellFormed(networkID)
	require.Nil(t, err)

}

func TestOperationBodyCongressVotingResult(t *testing.T) {
	opb := OperationBodyCongressVotingResult{
		BallotStamps: struct {
			Hash string
			Urls []string
		}{
			Hash: string(common.MakeHash([]byte("dummydummy"))),
			Urls: []string{"http://www.boscoin.io/1", "http://www.boscoin.io/2"},
		},
		Voters: struct {
			Hash string
			Urls []string
		}{
			Hash: string(common.MakeHash([]byte("dummydummy"))),
			Urls: []string{"http://www.boscoin.io/3", "http://www.boscoin.io/4"},
		},
		Result: struct {
			Count uint64
			Yes   uint64
			No    uint64
			ABS   uint64
		}{
			Count: 9,
			Yes:   2,
			No:    3,
			ABS:   4,
		},
	}
	op := Operation{
		H: OperationHeader{Type: OperationCongressVotingResult},
		B: opb,
	}
	hashed := op.MakeHashString()

	expected := "HHEKRf4Q4Hzvaz8NMvvsKV57neTvgkppYBg5Up39JB8k"
	require.Equal(t, hashed, expected)

	err := op.IsWellFormed(networkID)
	require.Nil(t, err)

}