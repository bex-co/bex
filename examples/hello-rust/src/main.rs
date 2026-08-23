// The "agent": answers 200 OK on every GET. MESSAGE is overridable so a redeploy
// can visibly change the response; PORT is injected by the platform.
use std::env;
use std::io::{Read, Write};
use std::net::TcpListener;

fn main() {
    let msg = env::var("MESSAGE").unwrap_or_else(|_| "OK".to_string());
    let port = env::var("PORT").unwrap_or_else(|_| "3000".to_string());
    let listener = TcpListener::bind(format!("0.0.0.0:{port}")).expect("bind");
    println!("hello-rust listening on {port}: {msg:?}");
    for stream in listener.incoming() {
        let Ok(mut stream) = stream else { continue };
        let _ = stream.read(&mut [0u8; 1024]);
        let response = format!(
            "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
            msg.len(),
            msg
        );
        let _ = stream.write_all(response.as_bytes());
    }
}
