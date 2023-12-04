// lib.rs
extern crate alloc;
extern crate core;
extern crate wee_alloc;

pub mod state;
pub mod tx;
pub mod utils;
pub mod allocator;
pub mod external_call;
// use crate::state::store_uint;

/// Rules:
/// - Declare public functions as extern with C layout, e.g., pub extern "C" fn simplefunction()
/// - Use #[no_mangle] macro for external functions
/// - Ensure structs follow C layout, i.e., #[repr(C)]
/// - Public functions should use u/i/f/32/64 for input.
/// - Hyper-WASM communicates with Rust contracts using pointers due to WASM's limited support for data types (limited to u/i/f/32/64)

/// During Output:
/// - Output must always be u64
/// - Left 32 bits represent the pointer, while the rest represent the size

/// Function Inputs:
/// - 0 inputs: Implies tx::info is not passed during the function call.
/// - 1 input: Implies tx::info is passed during the function call.
/// - 2 inputs: Implies tx::info is passed along with an input struct during the function call.

/// During input:
/// - For non-supported data types, provide memoryHex
/// - Hyper-WASM writes it to contract memory and returns the pointer
/// - Use the pointer to access the desired data type

/// Example for mutable and non-mutable structs sent during input:
/*
    Immutable Struct:
        #[no_mangle]
        pub extern "C" fn immutable_struct_function(tx_info_ptr: *const tx::Info){
            let tx_info = unsafe { &*tx_info_ptr };
        }
    
    Mutable Struct:
        #[no_mangle]
        pub extern "C" fn mutable_struct_function(tx_info_ptr: u32){
            // Safety: Ensure that the input is not null
            if input.is_null() {
                //take necessary action or return  any appropriate error code
            }
            // Safety: Dereference the pointer to access the struct
            let tx_info = unsafe { &mut *(tx_info_ptr as *mut tx::Info }; 
        }
*/

/// Example for output:
/// 
/// Ptr & size for a struct:
/// 
///     Example:
///         - Get pointer to MyStruct: 
///                let struct_ptr: *const MyStruct = &my_struct;
///         - Extract u32 value from the pointer: 
///                let u32_value: u32 = struct_ptr as *const _ as u32;
///         - Calculate size of the struct:   
///                let size_of_struct = mem::size_of_val(&my_struct);
/// 
/// Output Packing:
/// 
///     std::mem::forget(msg_sender);
///     return ((ptr as u64) << 32) | len as u64;

#[no_mangle]
pub extern "C" fn simpleFunction(chain_struct_ptr: *const tx::Info, input_ptr: u32) -> u64 {
    let chain_struct = unsafe { &*chain_struct_ptr };
    let msg_sender =    chain_struct.msg_sender();
    let (ptr, len) = unsafe{
        utils::string_to_ptr(&msg_sender)
       };

    std::mem::forget(msg_sender);
    unsafe {
        state::store_uint(3, 3)
    }
        
    return ((ptr as u64) << 32) | len as u64;
}

#[no_mangle]
pub extern "C" fn testStoreUint() {
    unsafe {state::store_uint(1, 10)}
}

#[no_mangle]
pub extern "C" fn testStoreInt() {
    unsafe { state::store_int(2, -10)}
}

#[no_mangle]
pub extern "C" fn testStoreFloat() {
    unsafe {state::store_float(3, 4.6)}
}

#[no_mangle]
pub extern "C" fn testStoreString() {
    
    let test_string = String::from("Hello world");
    unsafe {
        let (ptr, len) = utils::string_to_ptr(&test_string);
        state::store_string(4,ptr, len)
    }
}

#[no_mangle]
pub extern "C" fn testStoreBytes() {

    let data: Vec<u8> = vec![1, 2, 3, 4, 5];
    // Input is received as u8, but communication with runtime is limited to u32.
    let ptr = data.as_ptr() as u32;
    let len = data.len() as u32;
    unsafe {state::store_bytes(5, ptr, len)}

}

