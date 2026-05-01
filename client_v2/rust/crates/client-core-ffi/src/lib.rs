use std::{
    ffi::{c_char, CString},
    ptr,
};

use client_core::ClientViewState;

#[no_mangle]
pub extern "C" fn client_core_v2_version() -> *mut c_char {
    string_to_ptr(env!("CARGO_PKG_VERSION").to_string())
}

#[no_mangle]
pub extern "C" fn client_core_v2_default_state_json() -> *mut c_char {
    match serde_json::to_string(&ClientViewState::default()) {
        Ok(value) => string_to_ptr(value),
        Err(_) => ptr::null_mut(),
    }
}

#[no_mangle]
pub extern "C" fn client_core_v2_free_string(value: *mut c_char) {
    if value.is_null() {
        return;
    }
    unsafe {
        drop(CString::from_raw(value));
    }
}

fn string_to_ptr(value: String) -> *mut c_char {
    match CString::new(value) {
        Ok(value) => value.into_raw(),
        Err(_) => ptr::null_mut(),
    }
}
