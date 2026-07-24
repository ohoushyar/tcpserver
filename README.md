# tcpserver

A simple TCP server that reads blank-line-framed input and pipes it to a command.

```sh
tcpserver --cmd 'tr a-z A-Z'
tcpserver -v --addr 127.0.0.1:3131 --io-timeout 10s --cmd 'tee'
```

Default address is `127.0.0.1:3131`. `--cmd` is required.
