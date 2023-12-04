package actions

import (
	"context"
	"errors"
	"fmt"

	"github.com/manojkgorle/hyper-wasm/storage"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"

	"github.com/tetratelabs/wazero"
	_ "github.com/tetratelabs/wazero/api"
)

var _ chain.Action = (*Transact2)(nil)

type Transact2 struct {
	FunctionName    string `json:"functionName"`
	ContractAddress ids.ID `json:"contractAddress"`
	Input           []byte `json:"input"`
	MsgValue        uint64 `json:"msgValue"`
	TestBytes       []byte `json:"testBytes"`
	TouchAddress    []byte `json:"touchAddress"`
}

// we give users a defined pointer and ask them to write whatever data they wanted to write to it
// we do it using allocate_ptr

func (*Transact2) GetTypeID() uint8 {
	return txID2
}

func (t *Transact2) StateKeys(_ chain.Auth, txID ids.ID) []string {
	// we are assigning 2^5 storage keys for a contract.
	// statekeys are needed to be slot(i) --> slot0, slot1, slot2, ....
	// key := storage.ContractKey(t.ContractAddress)
	// for i := 0; i < 32; i++ {

	// }
	// return []string{string(key)}
	touchAddress := t.TouchAddress
	len := len(touchAddress) / 32

	var stateKeysArr []string
	stateKeysArr = append(stateKeysArr, string(storage.ContractKey(t.ContractAddress)))
	for i := 0; i < len; i++ {
		touchId, err := ids.ToID(touchAddress[i*32 : i*32+32])
		if err != nil {
			return []string{}
		}
		for j := 0; j < 32; j++ {
			varName := fmt.Sprint("slot", j)
			storageKey := string(storage.StateStorageKey(touchId, varName))
			stateKeysArr = append(stateKeysArr, storageKey)
		}
	}
	return stateKeysArr
}

func (*Transact2) StateKeysMaxChunks() []uint16 {
	return []uint16{65535}
}

func (*Transact2) OutputsWarpMessage() bool {
	return false
}

func (t *Transact2) Marshal(p *codec.Packer) {
	p.PackString(t.FunctionName)
	p.PackID(t.ContractAddress)
	p.PackBytes(t.Input)
	p.PackUint64(t.MsgValue)
	p.PackBytes(t.TestBytes)
	p.PackBytes(t.TouchAddress)
}

func (*Transact2) MaxComputeUnits(chain.Rules) uint64 {
	return TransactMaxComputeUnits
}

func (t *Transact2) Size() int {
	return codec.StringLen(t.FunctionName) + consts.IDLen + codec.BytesLen(t.Input) + consts.Uint64Len + codec.BytesLen(t.TestBytes) + codec.BytesLen(t.TouchAddress)
}

func (*Transact2) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

// @todo what is this, and how does this work??
func UnmarshalTransact2(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var Transact2 Transact2
	Transact2.FunctionName = p.UnpackString(true)
	p.UnpackID(true, &Transact2.ContractAddress)
	p.UnpackBytes(1024, false, &Transact2.Input)
	Transact2.MsgValue = p.UnpackUint64(true)
	p.UnpackBytes(1024, true, &Transact2.TestBytes)
	p.UnpackBytes(1024, false, &Transact2.TouchAddress)
	return &Transact2, p.Err()
}

func (t *Transact2) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	timestamp int64,
	auth chain.Auth,
	txID ids.ID,
	_ bool) (bool, uint64, []byte, *warp.UnsignedMessage, error) {

	// the struct input to be passed to wasm runtime
	contractAddress := t.ContractAddress // the contract to be loaded
	functionName := t.FunctionName
	msgSender := auth.Actor()
	msgSenderLen := 33

	// @todo contractAddress should be converted to ID.id
	deployedCodeAtContractAddress, err := storage.GetContract(ctx, mu, contractAddress)
	if deployedCodeAtContractAddress == nil {
		return false, TempComputeUnits, []byte("NO Code"), nil, nil
	}
	if err != nil {
		return false, TempComputeUnits, utils.ErrBytes(err), nil, nil
	}

	// Choose the context to use for function calls.
	ctxWasm := context.Background()

	// Create a new WebAssembly Runtime.
	r := wazero.NewRuntime(ctxWasm)
	defer r.Close(ctxWasm) // This closes everything this Runtime created.

	//@todo check for function existence
	compiledMod, err := r.CompileModule(ctxWasm, deployedCodeAtContractAddress)
	if err != nil {
		return false, TempComputeUnits, utils.ErrBytes(errors.New("compile module failed")), nil, nil
	}
	expFunc := compiledMod.ExportedFunctions()
	checkFunc := expFunc[functionName]
	if checkFunc == nil {
		return false, TempComputeUnits, utils.ErrBytes(errors.New("funtion doesnot exist")), nil, nil
	}
	mod, err := r.Instantiate(ctxWasm, deployedCodeAtContractAddress)
	if err != nil {
		return false, TempComputeUnits, utils.ErrBytes(err), nil, nil
	}

	// @todo load all required functions
	allocate_chain_struct := mod.ExportedFunction("allocate_chain_struct")
	deallocate_chain_struct := mod.ExportedFunction("deallocate_chain_struct")
	allocate_ptr := mod.ExportedFunction("allocate_ptr")
	deallocate_ptr := mod.ExportedFunction("deallocate_ptr")
	txFunction := mod.ExportedFunction(functionName)

	// @todo start of struct infusion and memory allocation

	// codec.Address -> AddressLen -> 33bytes
	results, err := allocate_ptr.Call(ctxWasm, uint64(msgSenderLen))
	if err != nil {
		return false, TempComputeUnits, utils.ErrBytes(err), nil, nil
	}
	addressPtr := results[0]
	chainInputStruct := ChainStruct{timestamp: timestamp, msgValue: 0, msgSenderPtr: uint32(addressPtr), msgSenderLen: uint32(msgSenderLen)}
	chainInputBytes := structToBytes(chainInputStruct)
	defer deallocate_ptr.Call(ctxWasm, addressPtr, uint64(msgSenderLen))

	results, err = allocate_chain_struct.Call(ctxWasm)
	if err != nil {
		return false, TempComputeUnits, utils.ErrBytes(err), nil, nil
	}
	chainStructPtr := results[0]
	defer deallocate_chain_struct.Call(ctxWasm, chainStructPtr)

	// @todo we need deterministic pointer here
	// @todo above is done, check & test for consequences

	// these should not fail
	mod.Memory().Write(uint32(addressPtr), msgSender[:])
	mod.Memory().Write(uint32(chainStructPtr), chainInputBytes)
	/// end of struct infusion and memory allocation

	//@todo transfer balance to contract's address before calling the contract's function
	// if msgValue != 0 {
	// storage.SubBalance(ctx, mu, auth.Actor(), ids.Empty, msgValue)
	// storage.AddBalance() //@todo add a way to retrieve contract balances
	// }

	//@todo actual call && result processing is undone still
	result, err := txFunction.Call(ctxWasm, chainStructPtr)
	if err != nil {
		return false, TempComputeUnits, utils.ErrBytes(err), nil, nil
	}
	// @todo we receive a pointer--> retrieve the value underlying the pointer and pass as output
	outPtr := uint32(result[0] >> 32)
	outPtrSize := uint32(result[0])
	output, ok := mod.Memory().Read(outPtr, outPtrSize) // @todo test it
	if !ok {
		return false, TempComputeUnits, []byte("cant read from memory"), nil, nil
	}
	return true, 10, output, nil, nil
}

//@todo steps to follow in completing the programme.
// dealing with inputs --> custom input struct & user sent input struct
// call, delegatecall w same ways
// gas metering for memory
//
//
//
//

//@todo value transfer can be supported using chain.Auth
// we can use auth
