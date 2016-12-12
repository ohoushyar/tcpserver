package main

/*
    TODO:
    o oop
    o log
    o cmd helper
*/

import (
    "bytes"
    "fmt"
    "io"
    "log"
    "net"
    "os"
    "os/exec"
    "strings"
)

type Config struct {
    Bind string
    Port string
    Cmd string
    CmdArgs []string
}

func main() {
    conf := Config{
        Bind: "127.0.0.1",
        Port: "3000",
        Cmd: "tr",
        CmdArgs: []string{"[::lower:]", "[:upper:]"},
    }
    debug(fmt.Sprintf("Config: %s", conf))

    // listen to a port for a tcp connection
    listener := get_listener(conf)
    run(conf, listener)
}

func get_listener(conf Config) net.Listener {
    addr := conf.Bind + ":" + conf.Port
    ln, err := net.Listen("tcp", addr)
    if err != nil {
        log.Fatal(fmt.Sprintf("Unable to listen to address: [%v] ERROR: %v", addr, err))
    }
    debug(fmt.Sprintf("Listenning to: %v", ln.Addr()))
    return ln
}

func run(conf Config, listener net.Listener) {
    for {
        conn, err := listener.Accept()
        if err != nil {
            errr(fmt.Sprintf("Something went wrong while connecting! ERROR: %v", err))
        } else {
            go handle_conn(conn, conf)
        }
    }
}

func get_cmd(conf Config) *exec.Cmd {
    cmd := exec.Command(conf.Cmd)
    for _, arg := range conf.CmdArgs {
        cmd.Args = append(cmd.Args, arg)
    }
    debug(fmt.Sprintf("cmd: %s", cmd.Args))
    return cmd
}

func handle_conn(conn net.Conn, conf Config) {
    defer conn.Close()

    cmd := get_cmd(conf)
    remote := conn.RemoteAddr()
    from := fmt.Sprintf("%s ", remote)
    debug(fmt.Sprintf("Accepted connection from: %v", remote))

    var buff bytes.Buffer;
    arr := make([]byte, 50)
    prev := make([]byte, 50)
    for {
        prev = arr
        n, err := conn.Read(arr)
        if n > 0 {
            debug(from + fmt.Sprintf("-> Read bytes: %v", n))
            debug(from + fmt.Sprintf("-> data: [%s]", arr[:n]))

            if prev[0] == '\r' && arr[0] == '\r' {
                str := string(buff.Bytes())
                run_cmd(cmd, conn, str)
                debug(from + "XX ... connection closed")
                return
            }

            buff.Write(arr[:n])
        }


        if err != nil {
            if err == io.EOF {
                debug(from + "XX ... connection closed")
                return
            }
            errr(from + fmt.Sprintf("!! Something happened while reading! ERROR: [%v]", err))
            return
        }
    }
}

func run_cmd(cmd *exec.Cmd, conn net.Conn, str string) {
    from := fmt.Sprintf("%s ", conn.RemoteAddr())
    debug(fmt.Sprintf("echo '%s' |", str))

    cmd.Stdin = strings.NewReader(str)
    cmd.Stdout = conn
    cmd.Stderr = conn

    err := cmd.Run()
    if err != nil {
        errr(from + fmt.Sprintf("!! Failed to exec! ERROR: %s\n",  err))
        errstr := strings.NewReader(fmt.Sprintf("err: %s\n", err))
        io.Copy(conn, errstr)
        return
    }

    debug(from + "!! ... Ran cmd")
    debug(fmt.Sprintf("%s", cmd.ProcessState))
}

func debug(log interface{}) {
    fmt.Fprintf(os.Stderr, "[debug] %s\n", log)
}

func errr(log interface{}) {
    fmt.Fprintf(os.Stderr, "[error] %s\n", log)
}
