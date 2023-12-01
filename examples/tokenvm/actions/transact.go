package actions

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"unsafe"

	"github.com/manojkgorle/hyper-wasm/storage"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

var _ chain.Action = (*Transact)(nil)

type Transact struct {
	FunctionName    []byte `json:"functionName"`
	Input           []byte `json:"input"` // hex encoded string -> []bytes
	MemoryWrite     []byte `json:"memoryWrite"`
	ContractAddress ids.ID `json:"contractAddress"`
	MsgValue        uint64 `json:"msgValue"`
	TouchAddress    []byte `json:"touchAddress"` // total # of contracts accessed during execution of transaction(including worst cases)

}

// we give users a defined pointer and ask them to write whatever data they wanted to write to it
// we do it using allocate_ptr
type ChainStruct struct {
	timestamp int64
	msgValue  uint64
	//@todo deal with this --> ptr & size need to be passed ✅
	msgSenderPtr uint32
	msgSenderLen uint32
}

func (*Transact) GetTypeID() uint8 {
	return deployContractID
}

func (t *Transact) StateKeys(_ chain.Auth, txID ids.ID) []string {
	// we are assigning 2^5 storage keys for a contract.
	// statekeys are needed to be slot(i) --> slot0, slot1, slot2, ....
	touchAddress := t.TouchAddress
	len := len(touchAddress) / 32
	stateKeysArr := make([]string, len*32)
	for i := 0; i < len; i++ {
		touchId := ids.ID(touchAddress[i*32 : i*32+32])
		for j := 0; j < 32; {
			varName := fmt.Sprint("slot", j)
			storageKey := string(storage.StateStorageKey(touchId, varName))
			stateKeysArr = append(stateKeysArr, storageKey)
		}
	}
	return stateKeysArr

}

func (*Transact) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.AssetChunks}
}

func (*Transact) OutputsWarpMessage() bool {
	return false
}

func (t *Transact) Marshal(p *codec.Packer) {
	p.PackBytes(t.FunctionName)
	p.PackBytes(t.Input)
	p.PackBytes(t.MemoryWrite)
	p.PackID(t.ContractAddress)
	p.PackUint64(t.MsgValue)
	p.PackBytes(t.TouchAddress)
}

func (*Transact) MaxComputeUnits(chain.Rules) uint64 {
	return TransactMaxComputeUnits
}

func (t *Transact) Size() int {
	// @todo add small bytes (smaller int prefix)
	return 10
}

func (*Transact) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

// @todo what is this, and how does this work??
func UnmarshalTransact(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var transact Transact
	p.UnpackBytes(int(math.MaxUint32), true, &transact.FunctionName)
	p.UnpackBytes(int(math.MaxUint32), true, &transact.Input)
	p.UnpackBytes(int(math.MaxUint32), true, &transact.MemoryWrite)
	p.UnpackID(true, &transact.ContractAddress)
	transact.MsgValue = p.UnpackUint64(true)
	p.UnpackBytes(int(math.MaxUint32), true, &transact.TouchAddress)
	return &transact, p.Err()
}

func (t *Transact) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	timestamp int64,
	auth chain.Auth,
	txID ids.ID,
	_ bool) (bool, uint64, []byte, *warp.UnsignedMessage, error) {

	function := string(t.FunctionName)   // the funcition we need to invoke, during the transaction
	inputBytes := t.Input                // the struct input to be passed to wasm runtime
	contractAddress := t.ContractAddress // the contract to be loaded
	msgValue := t.MsgValue
	memoryWriteBytes := t.MemoryWrite
	msgSender := auth.Actor()
	msgSenderLen := 33
	// @todo contractAddress should be converted to ID.id

	deployedCodeAtContractAddress, err := storage.GetContract(ctx, mu, contractAddress)
	if err != nil {
		log.Panicln(err)
	}
	// Choose the context to use for function calls.
	ctxWasm := context.Background()

	// Create a new WebAssembly Runtime.
	r := wazero.NewRuntime(ctxWasm)
	defer r.Close(ctxWasm) // This closes everything this Runtime created.

	//@todo inner functions
	stateStoreUintInner := func(ctxInner context.Context, m api.Module, i uint16 /* slot number*/, x uint64) {
		slot := "slot" + strconv.Itoa(int(i))
		storage.SetUint(ctx, mu, contractAddress, slot, x)
	}

	stateStoreIntInner := func(ctxInner context.Context, m api.Module, i uint16, x int64) {
		//convert int to uint to []byte and store
		slot := "slot" + strconv.Itoa(int(i))
		storage.SetUint(ctx, mu, contractAddress, slot, uint64(x))
	}

	stateStoreFloatInner := func(ctxInner context.Context, m api.Module, i uint16, x float64) {
		slot := "slot" + strconv.Itoa(int(i))
		storage.SetUint(ctx, mu, contractAddress, slot, math.Float64bits(x))
	}

	stateStoreStringInner := func(ctxInner context.Context, m api.Module, i uint16, ptr uint32, size uint32) {
		slot := "slot" + strconv.Itoa(int(i))
		if bytes, ok := m.Memory().Read(ptr, size); !ok {
			log.Panicln(ok)
		} else {
			storage.SetBytes(ctx, mu, contractAddress, slot, bytes)
		}
	}
	// Bytes inner and string inner are same:
	stateStoreBytesInner := func(ctxInner context.Context, m api.Module, i uint16, ptr uint32, size uint32) {
		slot := "slot" + strconv.Itoa(int(i))
		if bytes, ok := m.Memory().Read(ptr, size); !ok {
			log.Panicln(ok)
		} else {
			storage.SetBytes(ctx, mu, contractAddress, slot, bytes)
		}
	}

	stateAppendArrayInner := func(ctxInner context.Context, m api.Module, i uint16, ptr uint32, size uint32) {
		bytes, _ := m.Memory().Read(ptr, size)
		slot := "slot" + strconv.Itoa(int(i))
		arrayBytes, err := storage.GetBytes(ctx, mu, contractAddress, slot)
		lenA := len(arrayBytes)
		if err != nil {
			storage.SetBytes(ctx, mu, contractAddress, slot, bytes)
		} else {
			arrayBytes2 := make([]byte, lenA+int(size))
			copy(arrayBytes2[0:lenA], arrayBytes[:])
			copy(arrayBytes2[lenA:], bytes)
			storage.SetBytes(ctx, mu, contractAddress, slot, arrayBytes2)
		}
	}

	statePopArrayInner := func(ctxInner context.Context, m api.Module, i uint16, position uint32 /*index*/, size uint32) {
		slot := "slot" + strconv.Itoa(int(i))
		// fetch the array, remove the specified part and set the array to storage
		arrayBytes, _ := storage.GetBytes(ctx, mu, contractAddress, slot) // decided to ignore the error, instead of reverting (if array did not exist --> but we need to revert for such a thing)
		lenA := len(arrayBytes)
		if lenA >= int(position)*int(size) {
			arrayBytes2 := make([]byte, lenA-int(size))
			copy(arrayBytes2[0:position*size], arrayBytes[0:position*size])
			copy(arrayBytes2[(position)*size:], arrayBytes[(position+1)*size:])
			storage.SetBytes(ctx, mu, contractAddress, slot, arrayBytes2)
		}
	}

	stateInsertArrayInner := func(ctxInner context.Context, m api.Module, i uint16, ptr uint32, size uint32, position uint32 /*index*/) {
		slot := "slot" + strconv.Itoa(int(i))
		arrayBytes, _ := storage.GetBytes(ctx, mu, contractAddress, slot) // decided to ignore the error, instead of reverting (if array did not exist --> but we need to revert for such a thing)
		lenA := len(arrayBytes)
		if lenA >= int(position)*int(size) {
			bytes, _ := m.Memory().Read(ptr, size)
			arrayBytes2 := make([]byte, lenA+int(size))
			copy(arrayBytes2[0:position*size], arrayBytes[0:position*size])
			copy(arrayBytes2[position*size:(position+1)*size], bytes[:])
			copy(arrayBytes2[(position+1)*size:], arrayBytes[(position+1)*size:])
			storage.SetBytes(ctx, mu, contractAddress, slot, arrayBytes2)
		} // ⚠️under progress, no errors or reverts are thrown for array out of bound errors
	}

	stateReplaceArrayInner := func(ctxInner context.Context, m api.Module, i uint16, ptr uint32, size uint32, position uint32 /*index*/) {
		slot := "slot" + strconv.Itoa(int(i))
		arrayBytes, _ := storage.GetBytes(ctx, mu, contractAddress, slot) // decided to ignore the error, instead of reverting (if array did not exist --> but we need to revert for such a thing)
		lenA := len(arrayBytes)
		if lenA >= int(position)*int(size) {
			bytes, _ := m.Memory().Read(ptr, size)
			arrayBytes2 := make([]byte, lenA)
			copy(arrayBytes2[0:position*size], arrayBytes[0:position*size])
			copy(arrayBytes2[position*size:(position+1)*size], bytes[:])
			copy(arrayBytes2[(position+1)*size:], arrayBytes[(position+1)*size:])
			storage.SetBytes(ctx, mu, contractAddress, slot, arrayBytes2)
		} // ⚠️under progress, no errors or reverts are thrown for array out of bound
	}

	stateDeleteArrayInner := func(ctxInner context.Context, m api.Module, i uint16) {
		slot := "slot" + strconv.Itoa(int(i))
		storage.SetBytes(ctx, mu, contractAddress, slot, []byte{})
	}

	stateGetUintInner := func(ctxInner context.Context, m api.Module, i uint16) uint64 {
		slot := "slot" + strconv.Itoa(int(i))
		result, _ := storage.GetUint(ctx, mu, contractAddress, slot)
		return result
	}

	stateGetIntInner := func(ctxInner context.Context, m api.Module, i uint16) int64 {
		slot := "slot" + strconv.Itoa(int(i))
		// convert []byte to uint to int and return
		result, _ := storage.GetUint(ctx, mu, contractAddress, slot)
		return int64(result)
	}

	stateGetFloatInner := func(ctxInner context.Context, m api.Module, i uint16) float64 {
		slot := "slot" + strconv.Itoa(int(i))
		result, _ := storage.GetUint(ctx, mu, contractAddress, slot)
		return math.Float64frombits(result)
	}

	//@todo Warning:⚠️ memory values given must be read immediately
	stateGetStringInner := func(ctxInner context.Context, m api.Module, i uint16) (uint32 /*ptr*/, uint32 /*size*/) {
		slot := "slot" + strconv.Itoa(int(i))
		//@todo these values cant be passed back, they can only get passed as pointers
		// and we need to deal with those pointers
		result, _ := storage.GetBytes(ctx, mu, contractAddress, slot)
		offset := uint32(111111)
		// @todo need to work out something for access/scope types in go lang.
		// we need to actually call allocate and allocate memory to string, instead of arbitarily choosing the memory location
		m.Memory().Write(offset, result)
		return offset, uint32(len(result))
	}

	stateGetBytesInner := func(ctxInner context.Context, m api.Module, i uint16) (uint32 /*ptr*/, uint32 /*size*/) {
		slot := "slot" + strconv.Itoa(int(i))
		result, _ := storage.GetBytes(ctx, mu, contractAddress, slot)
		offset := uint32(222222)
		m.Memory().Write(offset, result)
		return offset, uint32(len(result))
	}

	stateGetArrayAtIndexInner := func(ctxInner context.Context, m api.Module, i uint16, size uint32, position uint32) (uint32 /*ptr*/, uint32 /*size*/) {
		slot := "slot" + strconv.Itoa(int(i))
		result, _ := storage.GetBytes(ctx, mu, contractAddress, slot)
		lenA := len(result)
		if lenA >= int(position)*int(size) {
			result2 := make([]byte, size)
			copy(result2[:], result[position*size:(position+1)*size])
			offset := uint32(333333)
			m.Memory().Write(offset, result2)
			return offset, uint32(len(result2))
		}
		return 0, 0 // ⚠️under progress, no errors or reverts are thrown for array out of bound
	}

	// @todo CALL
	CALLInner := func() {}

	// @todo DELEGATECALL
	DELEGATECALLInner := func() {}
	// Instantiate a Go-defined module named "env" that exports a function to
	// log to the console.
	// @todo import all external functions

	_, errWasm := r.NewHostModuleBuilder("env").NewFunctionBuilder().WithFunc(logI).Export("log").
		NewFunctionBuilder().WithFunc(stateStoreUintInner).Export("stateStoreUint").
		NewFunctionBuilder().WithFunc(stateStoreIntInner).Export("stateStoreInt").
		NewFunctionBuilder().WithFunc(stateStoreFloatInner).Export("stateStoreFloat").
		NewFunctionBuilder().WithFunc(stateStoreStringInner).Export("stateStoreString").
		NewFunctionBuilder().WithFunc(stateStoreBytesInner).Export("stateStoreBytes").
		NewFunctionBuilder().WithFunc(stateAppendArrayInner).Export("stateAppendArray").
		NewFunctionBuilder().WithFunc(statePopArrayInner).Export("statePopArray").
		NewFunctionBuilder().WithFunc(stateInsertArrayInner).Export("stateInsertArray").
		NewFunctionBuilder().WithFunc(stateReplaceArrayInner).Export("stateReplaceArray").
		NewFunctionBuilder().WithFunc(stateDeleteArrayInner).Export("stateDeleteArray").
		NewFunctionBuilder().WithFunc(stateGetUintInner).Export("stateGetUint").
		NewFunctionBuilder().WithFunc(stateGetIntInner).Export("stateGetInt").
		NewFunctionBuilder().WithFunc(stateGetStringInner).Export("stateGetString").
		NewFunctionBuilder().WithFunc(stateGetBytesInner).Export("stateGetBytes").
		NewFunctionBuilder().WithFunc(stateGetFloatInner).Export("stateGetFloat").
		NewFunctionBuilder().WithFunc(stateGetArrayAtIndexInner).Export("stateGetArrayAtIndex").
		NewFunctionBuilder().WithFunc(CALLInner).Export("CALL").
		NewFunctionBuilder().WithFunc(DELEGATECALLInner).Export("DELEGATECALL").Instantiate(ctxWasm)
	if errWasm != nil {
		return false, TempComputeUnits, utils.ErrBytes(errWasm), nil, nil
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
	txFunction := mod.ExportedFunction(function)

	// @todo start of struct infusion and memory allocation

	// codec.Address -> AddressLen -> 33bytes
	results, err := allocate_ptr.Call(ctxWasm, uint64(msgSenderLen))
	if err != nil {
		return false, TempComputeUnits, utils.ErrBytes(err), nil, nil
	}
	addressPtr := results[0]
	chainInputStruct := ChainStruct{timestamp: timestamp, msgValue: msgValue, msgSenderPtr: uint32(addressPtr), msgSenderLen: uint32(msgSenderLen)}
	chainInputBytes := structToBytes(chainInputStruct)
	defer deallocate_ptr.Call(ctxWasm, addressPtr, uint64(msgSenderLen))

	results, err = allocate_chain_struct.Call(ctxWasm)
	if err != nil {
		return false, TempComputeUnits, utils.ErrBytes(err), nil, nil
	}
	chainStructPtr := results[0]
	defer deallocate_chain_struct.Call(ctxWasm, chainStructPtr)

	inputSize := uint64(unsafe.Sizeof(inputBytes))
	results, err = allocate_ptr.Call(ctxWasm, inputSize)
	if err != nil {
		return false, TempComputeUnits, utils.ErrBytes(err), nil, nil
	}
	inputPtr := results[0]
	defer deallocate_ptr.Call(ctxWasm, inputPtr, inputSize)

	// @todo we need deterministic pointer here
	// @todo above is done, check & test for consequences
	memoryWritePtr := uint64(0)

	// these should not fail
	mod.Memory().Write(uint32(addressPtr), msgSender[:])
	mod.Memory().Write(uint32(inputPtr), inputBytes)
	mod.Memory().Write(uint32(chainStructPtr), chainInputBytes)
	mod.Memory().Write(uint32(memoryWritePtr), memoryWriteBytes)
	/// end of struct infusion and memory allocation

	//@todo transfer balance to contract's address before calling the contract's function
	// if msgValue != 0 {
	// storage.SubBalance(ctx, mu, auth.Actor(), ids.Empty, msgValue)
	// storage.AddBalance() //@todo add a way to retrieve contract balances
	// }

	//@todo actual call && result processing is undone still
	result, err := txFunction.Call(ctxWasm, chainStructPtr, inputPtr)
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
	return true, 0, output, nil, nil
}

func structToBytes(c ChainStruct) []byte {
	// creates array of length 2^30 and access the memory at struct c to have enough space for all the struct.
	// [:size:size] slices array to size and fixes array size as size
	size := unsafe.Sizeof(c)
	bytes := (*[1 << 30]byte)(unsafe.Pointer(&c))[:size:size]
	return bytes
}

// @todo need to change this
func logI(ctx context.Context, m api.Module, x int32, y float64) {
	fmt.Println("x", x)
	fmt.Println("y", y)
}

// @todo CALL
func CALL() {}

// @todo DELEGATECALL
func DELEGATECALL() {}

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
