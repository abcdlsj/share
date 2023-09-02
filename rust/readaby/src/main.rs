use clap::Parser;
use pretty_env_logger;
use readability::extractor;
#[macro_use]
extern crate log;

#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
struct Args {
    /// URL to extract content from
    #[arg(short, long, required = true)]
    link: String,
}

fn main() {
    pretty_env_logger::init();
    let args = Args::parse();

    info!("Link: {}", args.link);

    let ret = extractor::scrape(&args.link).unwrap();

    println!("{}", render(&ret.title, &ret.content));
}

fn render(title: &str, content: &str) -> String {
    format!(
        r#"<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style> body {{ font-family: sans-serif; }}
img {{ max-width: 100%; }}
iframe {{ max-width: 100%; }}
pre {{ white-space: pre-wrap;
word-wrap: break-word;
}}
pre code {{ white-space: pre-wrap;
word-wrap: break-word;
}}
pre code span {{ white-space: pre-wrap;
word-wrap: break-word;
}}
}} </style>
<title>{}</title>
</head>
<body>
{}
</body>
</html>
"#,
        title, content
    )
}
