use anyhow::{Context, Result, ensure};
use serde::Serialize;
use std::{io::Write, net::TcpStream, time::Duration};
use url::Url;

#[derive(Debug, Serialize)]
pub(crate) struct Entry<'a> {
    pub name: &'a str,
    pub outcome: &'a str,
    pub message: &'a str,
}

pub(crate) struct Summary(Option<TcpStream>);

impl Summary {
    pub fn open(uri: Option<&str>) -> Result<Self> {
        let Some(uri) = uri else {
            return Ok(Self(None));
        };
        let uri = Url::parse(uri).context("invalid summary URI")?;
        ensure!(
            uri.username().is_empty()
                && uri.password().is_none()
                && uri.query().is_none()
                && uri.fragment().is_none(),
            "invalid summary URI components"
        );
        ensure!(uri.scheme() == "tcp", "summary URI must use TCP");
        use std::net::ToSocketAddrs;
        ensure!(
            uri.path().is_empty() || uri.path() == "/",
            "TCP summary URI must not have a path"
        );
        let host = uri
            .host_str()
            .context("summary TCP host missing")?
            .trim_matches(['[', ']']);
        let port = uri.port().context("summary TCP port missing")?;
        let addresses = (host, port).to_socket_addrs()?;
        let mut connected = None;
        let mut last_error = None;
        for address in addresses {
            match TcpStream::connect_timeout(&address, Duration::from_secs(10)) {
                Ok(stream) => {
                    connected = Some(stream);
                    break;
                }
                Err(error) => last_error = Some(error),
            }
        }
        let stream = connected.with_context(|| format!("connect summary TCP: {last_error:?}"))?;
        stream.set_write_timeout(Some(Duration::from_secs(10)))?;
        Ok(Self(Some(stream)))
    }

    pub fn write(&mut self, entry: &Entry<'_>) -> Result<()> {
        if let Some(writer) = &mut self.0 {
            serde_json::to_writer(&mut *writer, entry)?;
            writer.write_all(b"\n")?;
            writer.flush()?;
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn tcp_stream_has_complete_json_lines_and_eof() {
        use std::{io::Read, net::TcpListener};
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let reader = std::thread::spawn(move || {
            let (mut stream, _) = listener.accept().unwrap();
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .unwrap();
            let mut data = String::new();
            stream.read_to_string(&mut data).unwrap();
            data
        });
        let mut summary = Summary::open(Some(&format!("tcp://{address}"))).unwrap();
        summary
            .write(&Entry {
                name: "a/b",
                outcome: "FAILED",
                message: "line\nquoted\"",
            })
            .unwrap();
        drop(summary);
        let data = reader.join().unwrap();
        assert!(data.ends_with('\n'));
        assert_eq!(data.lines().count(), 1);
        let value: serde_json::Value = serde_json::from_str(&data).unwrap();
        assert_eq!(value["name"], "a/b");
        assert_eq!(value["message"], "line\nquoted\"");
    }
}
