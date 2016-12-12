package main

/*
    TODO:
    o oop
    o log
    o chunk i/o
    o keep alive
*/

import (
    "bytes"
    "flag"
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

type Option struct {
    Addr string
    Cmd string
}

var DEBUG bool
var opt Option

func init() {
    const (
        verbose_usage = "Enable verbose"
        verbose_default_val = false
    )

    flag.BoolVar(&DEBUG, "v", verbose_default_val, verbose_usage)
    flag.BoolVar(&DEBUG, "verbose", verbose_default_val, verbose_usage)

    flag.StringVar(&opt.Addr, "addr", "127.0.0.1:3131", "Ip address to bind")
    flag.StringVar(&opt.Cmd, "cmd", "", "Command to run")
}


func main() {
    flag.Parse()
    debug(opt)

    conf := get_conf()
    listener := get_listener(conf)
    run(conf, listener)
}

func get_conf() Config {
    conf := Config{
        Bind: "127.0.0.1",
        Port: "3000",
        Cmd: "",
        CmdArgs: []string{},
    }

    if len(opt.Addr)>0 {
        conf.Bind, conf.Port = parse_opt_addr(opt.Addr)
    }

    conf.Cmd, conf.CmdArgs = parse_opt_cmd(opt.Cmd)

    debug(fmt.Sprintf("Config: %s", conf))
    return conf
}

func parse_opt_addr(addr string) (string, string) {
    addrs := strings.Split(addr, ":")
    if len(addrs) != 2 {
        print_usage()
        log.Fatal("Unable to parse addr")
    }
    return addrs[0], addrs[1]
}

func parse_opt_cmd(cmd string) (string, []string) {
    cmds := strings.Split(cmd, " ")
    if len(cmd) == 0 || len(cmds) < 1 {
        print_usage()
        log.Fatal("Unable to parse cmd. cmd is required.")
    }

    args := make([]string, 0)
    for _, arg := range cmds[1:] {
        if arg == "" || arg == " " { continue; }
        args = append(args, arg)
    }

    return cmds[0], args
}

func print_usage() {
    fmt.Println("Usage: tcpserver [--addr 0.0.0.0:3000] --cmd 'tr a-z A-Z'")
    flag.PrintDefaults()
    fmt.Println()
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
    debug(fmt.Sprintf("echo '%s' | %s", str, opt.Cmd))

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
    if !DEBUG { return; }
    fmt.Fprintf(os.Stderr, "[debug] %s\n", log)
}

func errr(log interface{}) {
    fmt.Fprintf(os.Stderr, "[error] %s\n", log)
}
