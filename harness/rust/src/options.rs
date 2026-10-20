use anyhow::{Context, Result, bail, ensure};
use clap::Parser;
use percent_encoding::percent_decode_str;
use std::{str::FromStr, time::Duration};
use temporalio_client::{ClientTlsOptions, ConnectionOptions, HttpConnectProxyOptions, TlsOptions};
use url::{Host, Url};

#[derive(Clone, Debug, PartialEq, Eq)]
pub(crate) struct Selection {
    pub name: String,
    pub task_queue: String,
    pub nexus_endpoint: Option<String>,
}

impl FromStr for Selection {
    type Err = anyhow::Error;

    fn from_str(value: &str) -> Result<Self> {
        let mut fields = value.splitn(3, ':');
        let name = fields.next().unwrap_or_default();
        let task_queue = fields.next().unwrap_or_default();
        let nexus_endpoint = fields.next();
        ensure!(
            !name.is_empty() && !task_queue.is_empty() && nexus_endpoint != Some(""),
            "expected feature/path:task-queue[:nexus-endpoint], got {value:?}"
        );
        Ok(Self {
            name: name.into(),
            task_queue: task_queue.into(),
            nexus_endpoint: nexus_endpoint.map(Into::into),
        })
    }
}

#[derive(Parser)]
#[command(disable_help_flag = true)]
pub(crate) struct Options {
    #[arg(long, value_parser = clap::builder::NonEmptyStringValueParser::new())]
    server: String,
    #[arg(long, value_parser = clap::builder::NonEmptyStringValueParser::new())]
    pub namespace: String,
    #[arg(long)]
    client_cert_path: Option<String>,
    #[arg(long)]
    client_key_path: Option<String>,
    #[arg(long)]
    ca_cert_path: Option<String>,
    #[arg(long)]
    tls_server_name: Option<String>,
    #[arg(long)]
    http_proxy_url: Option<String>,
    #[arg(required = true)]
    pub selections: Vec<Selection>,
    #[arg(long)]
    pub summary_uri: Option<String>,
    #[arg(long)]
    pub no_history_check: bool,
}

impl Options {
    pub fn parse(args: impl Iterator<Item = String>) -> Result<Self> {
        Ok(Self::try_parse_from(
            std::iter::once("program".to_owned()).chain(args),
        )?)
    }

    pub fn connection_options(&self) -> Result<ConnectionOptions> {
        let cert = self.client_cert_path.as_deref();
        let key = self.client_key_path.as_deref();
        ensure!(
            cert.is_some() == key.is_some(),
            "client certificate and key must be supplied together"
        );
        let ca = self.ca_cert_path.as_deref();
        let domain = self.tls_server_name.as_deref();
        let explicit_tls = cert.is_some() || ca.is_some() || domain.is_some();
        let server = &self.server;
        let target = if server.contains("://") {
            Url::parse(server)?
        } else {
            Url::parse(&format!(
                "{}://{server}",
                if explicit_tls { "https" } else { "http" }
            ))?
        };
        ensure!(
            matches!(target.scheme(), "http" | "https"),
            "unsupported server scheme"
        );
        ensure!(
            target.host().is_some()
                && target.username().is_empty()
                && target.password().is_none()
                && target.query().is_none()
                && target.fragment().is_none()
                && target.path() == "/",
            "server must be an HTTP(S) authority without credentials, path, query, or fragment"
        );
        ensure!(
            !explicit_tls || target.scheme() == "https",
            "TLS options require an HTTPS server"
        );
        let tls = if explicit_tls || target.scheme() == "https" {
            let read =
                |path: &str| std::fs::read(path).with_context(|| format!("read TLS file {path}"));
            let client_tls = match (cert, key) {
                (Some(cert), Some(key)) => Some(
                    ClientTlsOptions::builder()
                        .client_cert(read(cert)?)
                        .client_private_key(read(key)?)
                        .build(),
                ),
                _ => None,
            };
            Some(
                TlsOptions::builder()
                    .maybe_server_root_ca_cert(ca.map(read).transpose()?)
                    .maybe_domain(domain.map(str::to_owned))
                    .maybe_client_tls_options(client_tls)
                    .build(),
            )
        } else {
            None
        };
        let proxy = self
            .http_proxy_url
            .as_deref()
            .map(proxy_options)
            .transpose()?;
        let mut options = ConnectionOptions::new(target)
            .maybe_tls_options(tls)
            .connect_timeout(Duration::from_secs(15))
            .build();
        if proxy.is_some() {
            options.dns_load_balancing = None;
            options.http_connect_proxy = proxy;
        }
        Ok(options)
    }
}

fn proxy_options(value: &str) -> Result<HttpConnectProxyOptions> {
    let url = Url::parse(value).context("invalid HTTP proxy URL")?;
    ensure!(
        url.scheme() == "http",
        "HTTP CONNECT proxy requires an http:// URL"
    );
    ensure!(
        url.path() == "/" && url.query().is_none() && url.fragment().is_none(),
        "HTTP proxy URL must not contain a path, query, or fragment"
    );
    let host = match url.host().context("HTTP proxy URL requires a host")? {
        Host::Ipv6(ip) => format!("[{ip}]"),
        host => host.to_string(),
    };
    let address = format!(
        "{host}:{}",
        url.port_or_known_default().context("missing proxy port")?
    );
    let auth = if !url.username().is_empty() || url.password().is_some() {
        let decode = |value: &str| -> Result<String> {
            Ok(percent_decode_str(value)
                .decode_utf8()
                .context("proxy credentials are not UTF-8")?
                .into_owned())
        };
        if url.username().is_empty() {
            bail!("proxy password requires a username");
        }
        Some((
            decode(url.username())?,
            decode(url.password().unwrap_or_default())?,
        ))
    } else {
        None
    };
    Ok(HttpConnectProxyOptions::new(address)
        .maybe_basic_auth(auth)
        .build())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn parse(extra: &[&str]) -> Result<Options> {
        Options::parse(
            ["--server", "localhost:7233", "--namespace", "default"]
                .into_iter()
                .chain(extra.iter().copied())
                .map(str::to_owned),
        )
    }

    #[test]
    fn selection_preserves_endpoint_and_rejects_empty_fields() {
        assert_eq!(
            "a/b:q:endpoint:part".parse::<Selection>().unwrap(),
            Selection {
                name: "a/b".into(),
                task_queue: "q".into(),
                nexus_endpoint: Some("endpoint:part".into())
            }
        );
        for invalid in ["a", ":q", "a:", "a:q:"] {
            assert!(invalid.parse::<Selection>().is_err());
        }
    }

    #[test]
    fn transport_configuration_is_explicit() {
        let plain = parse(&["a:q"]).unwrap().connection_options().unwrap();
        assert!(plain.tls_options.is_none());
        let tls = parse(&["--tls-server-name", "temporal.test", "a:q"])
            .unwrap()
            .connection_options()
            .unwrap();
        assert_eq!(tls.target.scheme(), "https");
        assert_eq!(
            tls.tls_options.unwrap().domain.as_deref(),
            Some("temporal.test")
        );
        let proxy = parse(&["--http-proxy-url", "http://a%40b:p%3Ass@[::1]:8080", "a:q"])
            .unwrap()
            .connection_options()
            .unwrap();
        assert!(proxy.dns_load_balancing.is_none());
        let proxy = proxy.http_connect_proxy.unwrap();
        assert_eq!(proxy.basic_auth, Some(("a@b".into(), "p:ss".into())));
        assert!(
            parse(&["--client-cert-path", "/missing", "a:q"])
                .unwrap()
                .connection_options()
                .is_err()
        );
        assert!(
            parse(&["--http-proxy-url", "https://proxy", "a:q"])
                .unwrap()
                .connection_options()
                .is_err()
        );
        assert!(parse(&["--unknown", "value", "a:q"]).is_err());
        assert!(
            parse(&["--no-history-check", "a:q"])
                .unwrap()
                .no_history_check
        );
        assert!(Options::parse(["a:q"].into_iter().map(str::to_owned)).is_err());
    }
}
