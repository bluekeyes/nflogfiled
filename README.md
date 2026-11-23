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

Download the latest release from the [releases page][].

[releases page]: https://github.com/bluekeyes/nflogfiled/releases

### Packages

For Debian and Fedora systems, a `.deb` and `.rpm` package are provided
respectively. These packages install a systemd unit for the daemon along with
an example configuration, but leave the daemon disabled.

Edit the `/etc/nflogfiled/flags` file to specify an appropriate configuration
(at least one `-map` flag is required), then start the daemon:

    systemctl enable nflogfiled.service
    systemctl start nflogfiled.service

The systemd unit restricts the capabilities of the daemon. This means it may
not write logs files to `/usr`, `/boot`, `/etc`, or home directories. The
default log directory is `/var/log/nflog`, which systemd creates if it does not
exist. The `/tmp` directory is also private to the service.

### Archive

The `.tar.gz` archive includes the daemon binary and the `packaging` directory
with example systemd units and configuration. Extract the archive and configure
the daemon using the method of your choice.

## Usage

Start the daemon with at least one `-map` flag to map nflog groups to paths:

    nflogfiled -map=1:/var/log/nflog/forward.log

This writes all messages sent to group 1 to the `/var/log/nflog/forward.log`
file. Specify multiple groups by separating them with commas or by using
ranges.

The default output uses a text format. Use `-format=json` to switch to JSON
format, with one JSON-encoded message per line.

Use the `-fields` flag to change which fields are logged for each message.
The help output (`-help`) shows the default fields and all available fields.

To enable Prometheus metrics, use the `-metrics-address` flag, giving an
address in `host:port` form. The daemon exposes metrics via plain HTTP at the
`/metrics` on this address.

When passing multiple flags, you may also pass them using a flag file. This is
a text file that contains one flag per line in `-flag=value` format, ignoring
empty lines and lines starting with `#`. Pass the file path prefixed with `@`
in place of the flags on the command line:

    nflogfiled -format=json @/path/to/extra/flags

See `nflogfiled -help` for a description of all options.

## Limitations

The primary purpose of `nflogfiled` is handling logs from `ip`, `ip6`, and
`inet` chains in `nftables` or `iptables`. It may be missing features needed to
log messages from other sources. Please file an issue if there's anything
missing that would be useful and I'll consider adding it if it doesn't add
significant complexity.

## License

MIT
