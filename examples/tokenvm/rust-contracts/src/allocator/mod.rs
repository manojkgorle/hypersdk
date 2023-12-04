extern crate alloc;
extern crate core;
extern crate wee_alloc;

use std::alloc::{alloc, Layout};
use std::mem::MaybeUninit;
use alloc::vec::Vec;
use crate::tx::Info;

/// WebAssembly export that allocates a pointer (linear memory offset) that can
/// be used for a string.
///
/// This is an ownership transfer, which means the caller must call
/// [`deallocate`] when finished.
#[global_allocator]
static ALLOC: wee_alloc::WeeAlloc = wee_alloc::WeeAlloc::INIT;

/// Allocates size bytes and leaks the pointer where they start.
#[cfg_attr(all(target_arch = "wasm32"), export_name = "allocate_ptr")]
#[no_mangle]
pub extern "C" fn allocate(size: usize) -> *mut u8 {
    // Allocate the amount of bytes needed.
    let vec: Vec<MaybeUninit<u8>> = Vec::with_capacity(size);

    // into_raw leaks the memory to the caller.
    Box::into_raw(vec.into_boxed_slice()) as *mut u8
}


/// WebAssembly export that deallocates a pointer of the given size (linear
/// memory offset, byteCount) allocated by [`allocate`].
/// Retakes the pointer which allows its memory to be freed.
#[cfg_attr(all(target_arch = "wasm32"), export_name = "deallocate_ptr")]
#[no_mangle]
pub unsafe extern "C" fn deallocate(ptr: *mut u8, size: usize) {
    let _ = Vec::from_raw_parts(ptr, 0, size);
}

#[cfg_attr(all(target_arch = "wasm32"), export_name = "allocate_chain_struct")]
#[no_mangle]
pub extern "C" fn allocate_chain_struct() -> *mut Info{

    let layout = Layout::new::<Info>();

    let ptr = unsafe {alloc(layout)};

    if !ptr.is_null() {
        return ptr as *mut Info;
    }

    std::ptr::null_mut()
}

#[cfg_attr(all(target_arch = "wasm32"), export_name = "deallocate_chain_struct")]
#[no_mangle]
pub extern "C" fn deallocate_chain_struct(ptr: *mut Info) {

    if !ptr.is_null() {

        let layout = Layout::new::<Info>();

        unsafe { std::alloc::dealloc(ptr as *mut u8, layout) };
    }
}
