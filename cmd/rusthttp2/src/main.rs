// SPDX-License-Identifier: AGPL-3.0-or-later

use axum::{
    body::Body,
    extract::Path,
    http::{header, StatusCode},
    response::{IntoResponse, Response},
    routing::get,
    Router,
};
use bytes::Bytes;
use clap::{Parser, Subcommand};
use futures::StreamExt;
use hyper::body::Incoming;
use hyper_util::rt::{TokioExecutor, TokioIo};
use hyper_util::server::conn::auto::Builder;
use rustls::ServerConfig;
use std::net::SocketAddr;
use std::sync::Arc;
use std::time::Instant;
use tokio::net::TcpListener;
use tokio_rustls::TlsAcceptor;
use tower::Service;

/// Size of each zero-filled chunk for streaming (1 MiB).
const CHUNK_SIZE: usize = 1 << 20;

/// Static 1 MiB zero buffer for zero-copy streaming.
static ZEROS: [u8; CHUNK_SIZE] = [0u8; CHUNK_SIZE];

#[derive(Parser)]
#[command(about = "HTTP/2 benchmark tool (Rust)")]
struct Cli {
    #[command(subcommand)]
    command: Command,
}

#[derive(Subcommand)]
enum Command {
    /// Run the HTTP/2 server.
    Serve(ServeArgs),

    /// Run a measurement against the server.
    Measure(MeasureArgs),
}

#[derive(Parser)]
struct ServeArgs {
    /// Listen IP address.
    #[arg(short = 'A', long = "address", default_value = "127.0.0.1")]
    address: String,

    /// TLS certificate file.
    #[arg(long = "cert", default_value = "cert.pem")]
    cert: String,

    /// TLS private key file.
    #[arg(long = "key", default_value = "key.pem")]
    key: String,

    /// Disable TLS (use h2c — HTTP/2 over cleartext).
    #[arg(long = "no-tls")]
    no_tls: bool,

    /// Listen port.
    #[arg(short = 'p', long = "port", default_value = "4443")]
    port: u16,
}

#[derive(Parser)]
struct MeasureArgs {
    /// Server IP address.
    #[arg(short = 'A', long = "address", default_value = "127.0.0.1")]
    address: String,

    /// Number of bytes to transfer.
    #[arg(short = 'n', long = "bytes", default_value_t = 1 << 34)]
    bytes: u64,

    /// CA certificate file to trust.
    #[arg(long = "cert", default_value = "cert.pem")]
    cert: String,

    /// HTTP method: GET for download, PUT for upload.
    #[arg(short = 'X', long = "method", default_value = "GET")]
    method: String,

    /// Disable TLS (use h2c — HTTP/2 over cleartext).
    #[arg(long = "no-tls")]
    no_tls: bool,

    /// Server port.
    #[arg(short = 'p', long = "port", default_value = "4443")]
    port: u16,
}

#[tokio::main]
async fn main() {
    rustls::crypto::ring::default_provider()
        .install_default()
        .expect("failed to install rustls crypto provider");
    let cli = Cli::parse();
    match cli.command {
        Command::Serve(args) => serve(args).await,
        Command::Measure(args) => measure(args).await,
    }
}

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------

async fn serve(args: ServeArgs) {
    let app = Router::new().route("/:size", get(handle_get).put(handle_put));

    // Configure h2 for maximum throughput.
    let mut builder = Builder::new(TokioExecutor::new());
    builder
        .http2()
        .initial_stream_window_size(1 << 30) // 1 GiB
        .initial_connection_window_size(1 << 30) // 1 GiB
        .max_frame_size((1 << 24) - 1); // ~16 MiB (protocol max)

    let addr: SocketAddr = format!("{}:{}", args.address, args.port)
        .parse()
        .expect("invalid address");
    let listener = TcpListener::bind(addr).await.expect("failed to bind");

    let tls_acceptor = if args.no_tls {
        eprintln!("serving h2c at http://{addr}");
        None
    } else {
        let cert_pem = std::fs::read(&args.cert).expect("failed to read cert");
        let key_pem = std::fs::read(&args.key).expect("failed to read key");

        let certs = rustls_pemfile::certs(&mut cert_pem.as_slice())
            .collect::<Result<Vec<_>, _>>()
            .expect("failed to parse cert");
        let key = rustls_pemfile::private_key(&mut key_pem.as_slice())
            .expect("failed to parse key")
            .expect("no private key found");

        let mut tls_config = ServerConfig::builder()
            .with_no_client_auth()
            .with_single_cert(certs, key)
            .expect("failed to build TLS config");
        tls_config.alpn_protocols = vec![b"h2".to_vec()];
        eprintln!("serving h2 at https://{addr}");
        Some(TlsAcceptor::from(Arc::new(tls_config)))
    };

    loop {
        let (tcp_stream, remote_addr) = match listener.accept().await {
            Ok(v) => v,
            Err(e) => {
                eprintln!("accept error: {e}");
                continue;
            }
        };

        let tls_acceptor = tls_acceptor.clone();
        let app = app.clone();
        let builder = builder.clone();

        tokio::spawn(async move {
            eprintln!("connection from {remote_addr}");
            if let Some(tls_acceptor) = tls_acceptor {
                let tls_stream = match tls_acceptor.accept(tcp_stream).await {
                    Ok(v) => v,
                    Err(e) => {
                        eprintln!("TLS handshake error from {remote_addr}: {e}");
                        return;
                    }
                };
                let io = TokioIo::new(tls_stream);
                serve_connection(builder, io, app).await;
            } else {
                let io = TokioIo::new(tcp_stream);
                serve_connection(builder, io, app).await;
            }
        });
    }
}

async fn serve_connection<I>(builder: Builder<TokioExecutor>, io: I, app: Router)
where
    I: hyper::rt::Read + hyper::rt::Write + Unpin + Send + 'static,
{
    let service = hyper::service::service_fn(move |req: hyper::Request<Incoming>| {
        let mut app = app.clone();
        async move {
            let (parts, body) = req.into_parts();
            let req = hyper::Request::from_parts(parts, Body::new(body));
            app.call(req).await
        }
    });
    if let Err(e) = builder.serve_connection(io, service).await {
        eprintln!("connection error: {e}");
    }
}

/// GET /:size — stream the requested number of zero bytes.
async fn handle_get(Path(size): Path<u64>) -> Response {
    if size == 0 {
        return StatusCode::BAD_REQUEST.into_response();
    }
    let stream = futures::stream::unfold(0u64, move |sent| async move {
        if sent >= size {
            return None;
        }
        let remaining = (size - sent) as usize;
        let chunk_len = remaining.min(CHUNK_SIZE);
        let chunk = Bytes::from_static(&ZEROS[..chunk_len]);
        Some((Ok::<_, std::io::Error>(chunk), sent + chunk_len as u64))
    });
    Response::builder()
        .status(StatusCode::OK)
        .header(header::CONTENT_TYPE, "application/octet-stream")
        .header(header::CONTENT_LENGTH, size)
        .body(Body::from_stream(stream))
        .unwrap()
}

/// PUT /:size — accept and discard the request body.
async fn handle_put(Path(size): Path<u64>, body: Body) -> Response {
    if size == 0 {
        return StatusCode::BAD_REQUEST.into_response();
    }
    let mut stream = body.into_data_stream();
    let mut remaining = size + 1;
    while let Some(Ok(chunk)) = stream.next().await {
        remaining = remaining.saturating_sub(chunk.len() as u64);
        if remaining == 0 {
            break;
        }
    }
    StatusCode::NO_CONTENT.into_response()
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

async fn measure(args: MeasureArgs) {
    assert!(
        args.method == "GET" || args.method == "PUT",
        "method must be GET or PUT"
    );

    let mut client_builder = reqwest::Client::builder()
        .http2_initial_stream_window_size(1 << 30) // 1 GiB
        .http2_initial_connection_window_size(1 << 30) // 1 GiB
        .http2_max_frame_size((1 << 24) - 1); // ~16 MiB

    let scheme = if args.no_tls {
        client_builder = client_builder.http2_prior_knowledge();
        "http"
    } else {
        let ca_pem = std::fs::read(&args.cert).expect("failed to read CA cert");
        let ca_cert = reqwest::Certificate::from_pem(&ca_pem).expect("failed to parse CA cert");
        client_builder = client_builder.add_root_certificate(ca_cert);
        "https"
    };

    let client = client_builder.build().expect("failed to build HTTP client");
    let url = format!("{scheme}://{}:{}/{}", args.address, args.port, args.bytes);

    eprintln!("{}: url={}", args.method, url);

    let start = Instant::now();

    if args.method == "GET" {
        let resp = client.get(&url).send().await.expect("request failed");
        eprintln!(
            "response: status={} version={:?}",
            resp.status(),
            resp.version()
        );
        let mut stream = resp.bytes_stream();
        let mut total: u64 = 0;
        while let Some(result) = stream.next().await {
            let chunk = result.expect("stream error");
            total += chunk.len() as u64;
        }
        let elapsed = start.elapsed();
        let speed = total as f64 * 8.0 / elapsed.as_secs_f64();
        eprintln!(
            "download: bytes={total} elapsed={:.1}ms speed={:.1} Gbit/s",
            elapsed.as_secs_f64() * 1000.0,
            speed / 1e9
        );
    } else {
        let size = args.bytes;
        let stream = futures::stream::unfold(0u64, move |sent| async move {
            if sent >= size {
                return None;
            }
            let chunk_len = ((size - sent) as usize).min(CHUNK_SIZE);
            let chunk = Bytes::from_static(&ZEROS[..chunk_len]);
            Some((Ok::<_, std::io::Error>(chunk), sent + chunk_len as u64))
        });
        let body = reqwest::Body::wrap_stream(stream);
        let resp = client
            .put(&url)
            .header("content-length", args.bytes)
            .body(body)
            .send()
            .await
            .expect("request failed");
        eprintln!(
            "response: status={} version={:?}",
            resp.status(),
            resp.version()
        );
        let elapsed = start.elapsed();
        let speed = args.bytes as f64 * 8.0 / elapsed.as_secs_f64();
        eprintln!(
            "upload: bytes={} elapsed={:.1}ms speed={:.1} Gbit/s",
            args.bytes,
            elapsed.as_secs_f64() * 1000.0,
            speed / 1e9
        );
    }
}
