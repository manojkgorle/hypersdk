use crate::utils;

#[repr(C)]
pub struct Info {
    pub time_stamp: i64,
    pub msg_value: u64,
    pub msg_sender_ptr: u32,
    pub msg_sender_len: u32,
}

impl Info{
    pub fn msg_sender(&self) -> String{
       unsafe { utils::ptr_to_string(self.msg_sender_ptr, self.msg_sender_len)}
    }
}


