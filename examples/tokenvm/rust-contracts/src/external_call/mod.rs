use std::io::Bytes;

use crate::utils::string_to_ptr;
use crate::state::CALL;
#[repr(C)]
pub struct ExtCallInfo {
    pub function_name_ptr: u32,
    pub function_name_len: u32,
    pub contract_ptr: u32,
    pub contract_len: u32,
    pub msg_value: u64,
    pub input_ptr: u32,
    pub input_len: u32,
}

pub fn make_ext_call<T>(function_name: &String, contract_address: &String, msg_value: u64, input: Bytes<T>){

}