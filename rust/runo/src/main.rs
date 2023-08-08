use axum::{
    routing::{get}, Router,
};
use deno_core::{JsRuntime, RuntimeOptions};
use std::{net::SocketAddr, vec};
use anyhow::Result;
use deno_core::{op, Extension, Op};

fn runjs(js: &'static str) {
    let mut runtime = JsRuntime::new(RuntimeOptions {
        extensions: vec![kv_ext().unwrap()],
        ..Default::default()
    });

    let _ = runtime.execute_script_static("main.js", js).unwrap();
}

#[tokio::main]
async fn main() {
    // runjs(r#"Deno.core.print("Hello, world!\n");"#);
    let app = Router::new().route("/", get(root));

    axum::Server::bind(&SocketAddr::from(([127, 0, 0, 1], 3000)))
        .serve(app.into_make_service())
        .await
        .unwrap();
}

async fn root() -> &'static str {
    // this handler doesn't do anything exciting
    "Hello, World!"
}

#[op]
fn kv_store_set(_key: String, _value: String) -> Result<()> {
    Ok(())
}

#[op]
fn kv_store_get(_key: String) -> Result<Option<String>> {
    Ok(None)
}

pub fn kv_ext() -> Result<Extension> {
    let kv_ext = Extension {
        name: "kv_ext",
        ops: std::borrow::Cow::Borrowed(&[kv_store_get::DECL, kv_store_set::DECL]),
        ..Default::default()
    };

    Ok(kv_ext)
}

const RUNTIME_BOOTSTRAP: &str = r#"
((globalThis) => {
  const core = Deno.core;

  function argsToMessage(...args) {
    return args.map((arg) => JSON.stringify(arg)).join(" ");
  }

  globalThis.console = {
    log: (...args) => {
      core.print(`[out]: ${argsToMessage(...args)}\n`, false);
    },
    error: (...args) => {
      core.print(`[err]: ${argsToMessage(...args)}\n`, true);
    },
  };
})(globalThis);
"#;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_deno_core() {
        runjs(
            r#"
            Deno.core.print("Hello, world!\n");
            "#,
        );
    }

    #[test]
    fn test_kvstore() {
        runjs(
            r#"
            TODO: impl
            "#,
        );
    }
}
