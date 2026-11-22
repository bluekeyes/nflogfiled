# nflogfiled

Minimal daemon to to write nftable/nflog logs to files. An alternative to
[ulogd][] if you don't need the extra features of that tool or your platform
doesn't provide a `ulogd` package.

[ulogd]: https://www.netfilter.org/projects/ulogd/

## Features

* Map nflog groups to different files or standard out
* Format logs as text or JSON
* Reopen files on signal for integration with `logrotate`
* Optional Prometheus metrics endpoint
* Single static Go binary

## Installation

TODO

## Usage

TODO

## Limitations

The primary purpose of `nflogfiled` is handling logs from `ip`, `ip6`, and
`inet` chains in `nftables` or `iptables`. It may be missing features needed to
log messages from other sources. Please file an issue if there's anything
missing that would be useful and I'll consider adding it if it doesn't add
significant complexity.

## License

MIT
