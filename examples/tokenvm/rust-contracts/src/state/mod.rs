#[link(wasm_import_module = "env")]
extern "C" {
    #[link_name = "stateStoreUint"]
    pub fn store_uint(slot: u32, x:u64);
    #[link_name = "stateStoreInt"]
    pub fn store_int(slot: u32, x:i64);
    #[link_name = "stateStoreFloat"]
    pub fn store_float(slot: u32, x:f64);
    #[link_name = "stateStoreString"]
    pub fn store_string(slot:u32, ptr: u32, size: u32);
    #[link_name = "stateStoreBytes"]
    pub fn store_bytes(slot: u32, ptr: u32, size: u32);
    #[link_name = "stateAppendArray"]
    pub fn append_array(slot:u32, ptr: u32, size: u32);
    #[link_name = "statePopArray"]
    pub fn pop_array(slot:u32, position: u32, size: u32);
    #[link_name = "stateInsertArray"]
    pub fn insert_array(slot:u32, ptr: u32, size: u32, position: u32);
    #[link_name = "stateReplaceArray"]
    pub fn replace_array(slot:u32, ptr: u32, size: u32, position: u32);
    #[link_name = "stateDeleteArray"]
    pub fn delete_array(slot: u32);
    #[link_name = "stateGetUint"]
    pub fn get_uint(slot: u32) -> u64;
    #[link_name = "stateGetInt"]
    pub fn get_int(slot: u32) -> i64;
    #[link_name = "stateGetFloat"]
    pub fn get_float(slot: u32) -> f64;
    #[link_name = "stateGetString"]
    pub fn get_string(slot: u32) -> u64;
    #[link_name = "stateGetBytes"]
    pub fn get_bytes(slot: u32) -> u64;
    #[link_name = "stateGetArrayAtIndex"]
    pub fn get_array_at_index(slot: u32, size: u32, position: u32) -> u64;
    #[link_name = "CALL"]
    pub fn CALL(ptr: u32, ptr_len: u32) -> u64; // ⚠️ not supported yet
    #[link_name = "DELEGATECALL"]
    pub fn DELEGATECALL(); // ⚠️ not supported yet
}