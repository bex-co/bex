use std::ffi::{c_char, CStr};

extern "C" {
    fn bex_probe_checksum(text: *const u8, size: u32) -> u32;
    fn bex_probe_arithmetic(left: i64, right: i64) -> i64;
    fn bex_probe_header() -> *const c_char;
    fn bex_probe_library() -> *const c_char;
}

fn main() {
    let text = "Bex 雪 λ🙂".as_bytes();
    unsafe {
        println!("rust-native|{:08x}|{}|{}|{}",
            bex_probe_checksum(text.as_ptr(), text.len() as u32),
            bex_probe_arithmetic(63, 41),
            CStr::from_ptr(bex_probe_header()).to_str().expect("header version"),
            CStr::from_ptr(bex_probe_library()).to_str().expect("library version"));
    }
}
