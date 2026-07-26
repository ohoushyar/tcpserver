# tcpserver Specification

This document describes the **current behavior** of `tcpserver`. It is a behavioral spec, not a redesign proposal.

## Overview

`tcpserver` is a **single-process** TCP server that supports **concurrency**: one process handles many connections at the same time (one goroutine per connection).

It:

1. Listens on a configurable TCP address.
2. Accepts and serves multiple connections concurrently.
3. Reads a blank-line-framed message from each connection.
4. Runs a user-supplied command once per successfully framed message, feeding the accumulated input to the command's stdin and writing the command's stdout and stderr back to the connection.
5. Shuts down on `SIGINT` / `SIGTERM` by stopping new accepts and waiting for in-flight handlers to finish.

Module: `github.com/ohoushyar/tcpserver`  
Language: Go 1.26+  
Entry point: `main` package (`main.go`)

## CLI

Flags are parsed with the Go `flag` package. Unknown flags and parse errors exit with status `1` after printing the error to stderr. `-h` / `--help` prints usage and exits `0`.

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--cmd` | string | _(empty)_ | **yes** | Command and arguments, space-separated |
| `--addr` | string | `127.0.0.1:3131` | no | TCP listen address (`host:port`) |
| `--io-timeout` | duration | `0` (no timeout) | no | Read deadline duration string (e.g. `2s`, `500ms`) |
| `-v` / `--verbose` | bool | `false` | no | Enable debug logging |

### `--cmd` parsing

The `--cmd` value is trimmed of leading/trailing Unicode whitespace, then split on ASCII spaces (` `):

- The first non-empty token is the executable path/name.
- Remaining tokens are arguments; empty tokens from consecutive spaces are skipped.
- There is **no** shell quoting or escape handling. `"hello world"` is not one argument.
- An empty or whitespace-only value is an error: `cmd is required` (exit status `1`, usage printed).

Examples:

| `--cmd` value | Executable | Args |
|---------------|------------|------|
| `tr a-z A-Z` | `tr` | `a-z`, `A-Z` |
| `tee` | `tee` | _(none)_ |
| `echo  hello` | `echo` | `hello` |

## Logging

- Logs go to **stderr** via `log/slog` text handler.
- Default level: `INFO`.
- With `-v` / `--verbose`: `DEBUG`.
- Connection handlers add a `remote` attribute (peer address).

Notable log events:

| Level | When |
|-------|------|
| INFO | Listening started (`addr`) |
| DEBUG | Config dump, accept, per-line data, command start/finish |
| ERROR | Accept failure (non-shutdown), read errors, command failures, fatal server errors |

## Networking

### Listen

- Network: TCP (`net.Listen("tcp", addr)`).
- Listen failure ends the process with a non-zero exit after logging.

### Accept loop

- Connections are accepted in a loop until shutdown closes the listener.
- Non-shutdown accept errors are logged and the loop continues.
- Each accepted connection is handled in its own goroutine (`sync.WaitGroup`), so multiple clients can be served at the same time within the single process.

### I/O timeout

When `--io-timeout` is greater than zero, immediately after accept the server calls `SetReadDeadline(now + io-timeout)` once on the connection.

- Deadline is **not** refreshed per read or per line.
- Default `0` means no application read deadline (reads may block indefinitely).

## Wire protocol

### Framing

- Input is line-oriented. Lines are delimited by `\n` (`bufio.Scanner` default split).
- Trailing `\r` is not specially stripped beyond Scanner's normal behavior (lines are `Text()` of each token).
- Message end is signaled by **two consecutive blank lines**: an empty line followed immediately by another empty line.
- At least one prior line must have been read before the terminator can fire (`cnt > 0`).

Sequence that completes a message:

```text
<one or more content lines, each ending in \n>
\n          ← first blank line (empty line)
\n          ← second blank line (empty line) → run command
```

Example bytes that trigger processing of `Hi there!`:

```text
Hi there!\n\n\n
```

### Accumulation rules

For each scanned line **before** the terminator completes:

1. Append the line text to an internal buffer.
2. Append a single `\n`.

When the second consecutive blank line is seen:

- The command runs with the buffer accumulated so far.
- The second blank line is **not** appended to the buffer.
- The first blank line **is** included (as an empty line / trailing `\n` in the buffer).

So for `Hi there!\n\n\n`, command stdin is:

```text
Hi there!\n
\n
```

### Incomplete / abandoned messages

If the peer closes the connection (or a read error occurs) **before** two consecutive blank lines:

- The command is **not** run.
- Accumulated data is discarded.
- Read errors are logged at ERROR; clean EOF is logged at DEBUG as connection closed.

### Connection lifetime

- One framed message per connection: after a successful frame, the command runs and the handler returns (connection closed via `defer`).
- The server does not read a second message on the same connection.

## Command execution

When a message is framed:

1. Build `exec.Command(cfg.cmd, cfg.cmdArgs...)`.
2. Set stdin to the accumulated buffer string.
3. Set both stdout and stderr to the TCP connection.
4. `Run()` the command (wait until exit).

On command failure:

- Log at ERROR.
- Write `err: <error>\n` to the connection (best-effort).
- Then close the connection.

On success:

- Command output has already been written to the connection.
- Connection is closed when the handler returns.

The executable is looked up using the process environment (`PATH`, etc.). There is no sandboxing beyond what the OS provides.

## Shutdown

Signals: `SIGINT` (`os.Interrupt`) and `SIGTERM`.

On signal:

1. Context cancels.
2. Listener is closed (unblocks `Accept`).
3. Accept loop exits.
4. Server waits on the connection wait group until **all** in-flight handlers finish.
5. Process exits `0` if no prior fatal error.

Handlers are **not** force-interrupted. Idle clients blocked in `Scan()`, or long-running commands, delay process exit until they complete or disconnect. This drain-until-done behavior is intentional.

## Process exit codes

| Code | Meaning |
|------|---------|
| `0` | Help requested, or clean run/shutdown |
| `1` | Flag/config error, or `run` returned an error (e.g. listen failure) |

## Out of scope (current behavior)

The following are **not** implemented:

- TLS / encryption
- Authentication or authorization
- Multiple messages per connection
- Shell-style quoting in `--cmd`
- Write deadlines (only optional read deadline)
- Hard shutdown deadline / force-close of active connections
- Metrics or health endpoints
- Config files or environment-variable configuration (flags only)

## Reference client

```sh
tcpserver --cmd 'tr a-z A-Z'
printf 'Hi there!\n\n\n' | nc 127.0.0.1 3131
# expected response: HI THERE!
```
