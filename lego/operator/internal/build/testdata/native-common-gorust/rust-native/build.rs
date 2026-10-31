use std::{env, path::PathBuf, process::Command};

fn main() {
    let output = PathBuf::from(env::var_os("OUT_DIR").expect("Cargo OUT_DIR"));
    let object = output.join("probe.o");
    let archive = output.join("libbex_probe.a");
    assert!(Command::new("cc").args(["-std=c11", "-Wall", "-Werror", "-c", "probe.c", "-o"])
        .arg(&object).status().expect("C compiler").success());
    assert!(Command::new("ar").arg("rcs").arg(&archive).arg(&object)
        .status().expect("static archiver").success());
    println!("cargo:rustc-link-search=native={}", output.display());
    println!("cargo:rustc-link-lib=static=bex_probe");
    println!("cargo:rustc-link-lib=z");
    println!("cargo:rerun-if-changed=probe.c");
}
