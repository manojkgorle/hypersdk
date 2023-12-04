package extcaller

import (
	"context"
	"errors"
	"log"
	"math"
	"strconv"
	"unsafe"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/state"
	"github.com/manojkgorle/hyper-wasm/storage"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

const (
	TempComputeUnits = 10
)

// @todo we are still pending with this
type Ext_call_struct struct {
	Ctx       context.Context
	Cr        chain.Rules
	Mu        state.Mutable
	Timestamp int64
	Auth      chain.Auth
	TxID      ids.ID

	FunctionName    string
	ContractAddress ids.ID
	Input           []byte
	MsgValue        uint64
	MsgSender       ids.ID
}

type ChainStruct struct {
	timestamp    int64
	msgValue     uint64
	msgSenderPtr uint32
	msgSenderLen uint32
}

func (e *Ext_call_struct) External_call() (bool /*success/fail*/, uint64 /*compute units*/, []byte /*output*/, error) {
	ctx := e.Ctx
	mu := e.Mu
	function := e.FunctionName
	inputBytes := e.Input
	contractAddress := e.ContractAddress
	msgValue := e.MsgValue
	msgSender := e.MsgSender
	msgSenderLen := 32
	// txOrigin := e.Auth.Actor() // @todo later add support for this
	// chainInputStruct := ChainStruct{timestamp: e.Timestamp, msgValue: msgValue, }
	deployedCodeAtContractAddress, err := storage.GetContract(ctx, mu, contractAddress)
	if err != nil {
		return false, TempComputeUnits, nil, errors.New("NO Code")
	}
	// Choose the context to use for function calls.
	ctxWasm := context.Background()

	// Create a new WebAssembly Runtime.
	r := wazero.NewRuntime(ctxWasm)
	defer r.Close(ctxWasm) // This closes everything this Runtime created.

	//@todo check for function existence
	compiledMod, err := r.CompileModule(ctxWasm, deployedCodeAtContractAddress)

	if err != nil {
		return false, TempComputeUnits, nil, errors.New("compile module failed")
	}

	expFunc := compiledMod.ExportedFunctions()
	checkFunc := expFunc[function]

	if checkFunc == nil {
		return false, TempComputeUnits, nil, errors.New("funtion doesnot exist")
	}

	inputParamCount := len(checkFunc.ParamTypes())
	outputParamCount := len(checkFunc.ResultTypes())

	//@todo inner functions
	stateStoreUintInner := func(ctxInner context.Context, m api.Module, i uint32 /* slot number*/, x uint64) {
		slot := "slot" + strconv.Itoa(int(i))
		storage.SetUint(ctx, mu, contractAddress, slot, x)
	}

	stateStoreIntInner := func(ctxInner context.Context, m api.Module, i uint32, x int64) {
		//convert int to uint to []byte and store
		slot := "slot" + strconv.Itoa(int(i))
		storage.SetUint(ctx, mu, contractAddress, slot, uint64(x))
	}

	stateStoreFloatInner := func(ctxInner context.Context, m api.Module, i uint32, x float64) {
		slot := "slot" + strconv.Itoa(int(i))
		storage.SetUint(ctx, mu, contractAddress, slot, math.Float64bits(x))
	}

	stateStoreStringInner := func(ctxInner context.Context, m api.Module, i uint32, ptr uint32, size uint32) {
		slot := "slot" + strconv.Itoa(int(i))
		if bytes, ok := m.Memory().Read(ptr, size); !ok {
			log.Panicln(ok)
		} else {
			storage.SetBytes(ctx, mu, contractAddress, slot, bytes)
		}
	}
	// Bytes inner and string inner are same:
	stateStoreBytesInner := func(ctxInner context.Context, m api.Module, i uint32, ptr uint32, size uint32) {
		slot := "slot" + strconv.Itoa(int(i))
		if bytes, ok := m.Memory().Read(ptr, size); !ok {
			log.Panicln(ok)
		} else {
			storage.SetBytes(ctx, mu, contractAddress, slot, bytes)
		}
	}

	stateAppendArrayInner := func(ctxInner context.Context, m api.Module, i uint32, ptr uint32, size uint32) {
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

	statePopArrayInner := func(ctxInner context.Context, m api.Module, i uint32, position uint32 /*index*/, size uint32) {
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

	stateInsertArrayInner := func(ctxInner context.Context, m api.Module, i uint32, ptr uint32, size uint32, position uint32 /*index*/) {
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

	stateReplaceArrayInner := func(ctxInner context.Context, m api.Module, i uint32, ptr uint32, size uint32, position uint32 /*index*/) {
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

	stateDeleteArrayInner := func(ctxInner context.Context, m api.Module, i uint32) {
		slot := "slot" + strconv.Itoa(int(i))
		storage.SetBytes(ctx, mu, contractAddress, slot, []byte{})
	}

	stateGetUintInner := func(ctxInner context.Context, m api.Module, i uint32) uint64 {
		slot := "slot" + strconv.Itoa(int(i))
		result, _ := storage.GetUint(ctx, mu, contractAddress, slot)
		return result
	}

	stateGetIntInner := func(ctxInner context.Context, m api.Module, i uint32) int64 {
		slot := "slot" + strconv.Itoa(int(i))
		// convert []byte to uint to int and return
		result, _ := storage.GetUint(ctx, mu, contractAddress, slot)
		return int64(result)
	}

	stateGetFloatInner := func(ctxInner context.Context, m api.Module, i uint32) float64 {
		slot := "slot" + strconv.Itoa(int(i))
		result, _ := storage.GetUint(ctx, mu, contractAddress, slot)
		return math.Float64frombits(result)
	}

	//@todo Warning:⚠️ memory values given must be read immediately
	stateGetStringInner := func(ctxInner context.Context, m api.Module, i uint32) uint64 {
		slot := "slot" + strconv.Itoa(int(i))
		//@todo these values cant be passed back, they can only get passed as pointers
		// and we need to deal with those pointers
		result, _ := storage.GetBytes(ctx, mu, contractAddress, slot)
		offset := uint32(111111)
		// @todo need to work out something for access/scope types in go lang.
		// we need to actually call allocate and allocate memory to string, instead of arbitarily choosing the memory location
		m.Memory().Write(offset, result)
		return uint64(offset)<<32 | uint64(len(result))
	}

	stateGetBytesInner := func(ctxInner context.Context, m api.Module, i uint32) uint64 {
		slot := "slot" + strconv.Itoa(int(i))
		result, _ := storage.GetBytes(ctx, mu, contractAddress, slot)
		offset := uint32(222222)
		m.Memory().Write(offset, result)
		return uint64(offset)<<32 | uint64(len(result))
	}

	stateGetArrayAtIndexInner := func(ctxInner context.Context, m api.Module, i uint32, size uint32, position uint32) uint64 {
		slot := "slot" + strconv.Itoa(int(i))
		result, _ := storage.GetBytes(ctx, mu, contractAddress, slot)
		lenA := len(result)
		if lenA >= int(position)*int(size) {
			result2 := make([]byte, size)
			copy(result2[:], result[position*size:(position+1)*size])
			offset := uint32(333333)
			m.Memory().Write(offset, result2)
			return uint64(offset)<<32 | uint64(len(result))
		}
		return 0 // ⚠️under progress, no errors or reverts are thrown for array out of bound
	}

	// @todo CALL
	CALLInner := func(ctxInner context.Context, m api.Module) {}

	// @todo DELEGATECALL
	DELEGATECALLInner := func(ctxInner context.Context, m api.Module) {}
	// // Instantiate a Go-defined module named "env" that exports a function to
	// // log to the console.
	// // @todo import all external functions

	_, errWasm := r.NewHostModuleBuilder("env").NewFunctionBuilder().WithFunc(stateStoreUintInner).Export("stateStoreUint").
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
		log.Panicln(errWasm)
		return false, TempComputeUnits, nil, errWasm
	}

	mod, err := r.Instantiate(ctxWasm, deployedCodeAtContractAddress)
	if err != nil {
		return false, TempComputeUnits, nil, err
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
		return false, TempComputeUnits, nil, err
	}
	addressPtr := results[0]
	chainInputStruct := ChainStruct{timestamp: e.Timestamp, msgValue: msgValue, msgSenderPtr: uint32(addressPtr), msgSenderLen: uint32(msgSenderLen)}
	chainInputBytes := structToBytes(chainInputStruct)
	defer deallocate_ptr.Call(ctxWasm, addressPtr, uint64(msgSenderLen))

	results, err = allocate_chain_struct.Call(ctxWasm)
	if err != nil {
		return false, TempComputeUnits, nil, err
	}

	chainStructPtr := results[0]
	defer deallocate_chain_struct.Call(ctxWasm, chainStructPtr)

	inputSize := uint64(unsafe.Sizeof(inputBytes))
	results, err = allocate_ptr.Call(ctxWasm, inputSize)
	if err != nil {
		return false, TempComputeUnits, nil, err
	}
	inputPtr := results[0]
	defer deallocate_ptr.Call(ctxWasm, inputPtr, inputSize)

	//@todo no memory write for ext_call
	// these should not fail
	mod.Memory().Write(uint32(addressPtr), msgSender[:])
	mod.Memory().Write(uint32(inputPtr), inputBytes)
	mod.Memory().Write(uint32(chainStructPtr), chainInputBytes)

	/// end of struct infusion and memory allocation

	//@todo transfer balance to contract's address before calling the contract's function
	// if msgValue != 0 {
	// storage.SubBalance(ctx, mu, auth.Actor(), ids.Empty, msgValue)
	// storage.AddBalance() //@todo add a way to retrieve contract balances
	// }

	//@todo actual call && result processing is undone still --> tune in for inputParamCount & outputParamCount
	//@todo use switch cases
	if inputParamCount == 2 {
		result, err := txFunction.Call(ctxWasm, chainStructPtr, inputPtr)
		if err != nil {
			return false, TempComputeUnits, nil, err
		}
		if outputParamCount == 1 {
			// @todo we receive a pointer--> retrieve the value underlying the pointer and pass as output
			outPtr := uint32(result[0] >> 32)
			outPtrSize := uint32(result[0])
			output, ok := mod.Memory().Read(outPtr, outPtrSize) // @todo test it
			if !ok {
				return false, TempComputeUnits, nil, errors.New("cant read from memory")
			}
			return true, 0, output, nil
		}
		if outputParamCount == 0 {
			return true, 0, nil, nil
		}
	}
	if inputParamCount == 1 {
		result, err := txFunction.Call(ctxWasm, chainStructPtr)
		if err != nil {
			return false, TempComputeUnits, nil, err
		}
		if outputParamCount == 1 {
			// @todo we receive a pointer--> retrieve the value underlying the pointer and pass as output
			outPtr := uint32(result[0] >> 32)
			outPtrSize := uint32(result[0])
			output, ok := mod.Memory().Read(outPtr, outPtrSize) // @todo test it
			if !ok {
				return false, TempComputeUnits, nil, errors.New("cant read from memory")
			}
			return true, 0, output, nil
		}
		if outputParamCount == 0 {
			return true, 0, nil, nil
		}
	}
	if inputParamCount == 0 {
		result, err := txFunction.Call(ctxWasm)
		if err != nil {
			return false, TempComputeUnits, nil, err
		}
		if outputParamCount == 1 {
			// @todo we receive a pointer--> retrieve the value underlying the pointer and pass as output
			outPtr := uint32(result[0] >> 32)
			outPtrSize := uint32(result[0])
			output, ok := mod.Memory().Read(outPtr, outPtrSize) // @todo test it
			if !ok {
				return false, TempComputeUnits, nil, errors.New("cant read from memory")
			}
			return true, 0, output, nil
		}
		if outputParamCount == 0 {
			return true, 0, nil, nil
		}
	}
	return false, TempComputeUnits, nil, nil //@todo if we reach here, abi is not implimented by the function
}

func structToBytes(c ChainStruct) []byte {
	// creates array of length 2^30 and access the memory at struct c to have enough space for all the struct.
	// [:size:size] slices array to size and fixes array size as size
	size := unsafe.Sizeof(c)
	bytes := (*[1 << 30]byte)(unsafe.Pointer(&c))[:size:size]
	return bytes
}
