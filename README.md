# tcpserver

A simple TCP server that reads blank-line-framed input and pipes it to a command.

```sh
tcpserver --cmd 'tr a-z A-Z'
tcpserver -v --addr 127.0.0.1:3131 --io-timeout 10s --cmd 'tee'
```

Default address is `127.0.0.1:3131`. `--cmd` is required.

## Protocol

Each connection is line-oriented (`\n`-terminated lines). The client sends one or more content lines, then ends the message with **two consecutive blank lines** (an empty line followed by another empty line). That terminator is what tells the server to run `--cmd` with the accumulated input on stdin and write the command's stdout/stderr back on the connection.

Closing the connection without that terminator does **not** process the input.

Client sample:

```sh
printf 'Hi there!\n\n\n' | nc 127.0.0.1 3131
```
