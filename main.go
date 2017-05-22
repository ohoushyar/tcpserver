package main

/*
   TODO:
   o oop
   o keep alive
*/

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
	"strings"
)

type config struct {
	Bind    string
	Port    string
	Cmd     string
	CmdArgs []string
}

type option struct {
	Addr string
	Cmd  string
}

var deb bool
var opt option

func init() {
	const (
		verbUsage  = "Enable verbose"
		verbDefVal = false
	)

	flag.BoolVar(&deb, "v", verbDefVal, verbUsage)
	flag.BoolVar(&deb, "verbose", verbDefVal, verbUsage)

	flag.StringVar(&opt.Addr, "addr", "127.0.0.1:3131", "Ip address to bind")
	flag.StringVar(&opt.Cmd, "cmd", "", "Command to run")
}

func main() {
	flag.Parse()
	debug("%s", opt)

	conf := getConf()
	listener := getListener(conf)
	run(conf, listener)
}

func getConf() config {
	conf := config{
		Bind:    "127.0.0.1",
		Port:    "3000",
		Cmd:     "tee",
		CmdArgs: []string{},
	}

	if len(opt.Addr) > 0 {
		conf.Bind, conf.Port = parseOptAddr(opt.Addr)
	}

	if len(opt.Cmd) > 0 {
		conf.Cmd, conf.CmdArgs = parseOptCmd(opt.Cmd)
	}

	debug("Config: %s", conf)
	return conf
}

func parseOptAddr(addr string) (string, string) {
	addrs := strings.Split(addr, ":")
	if len(addrs) != 2 {
		printUsage()
		log.Fatal("Unable to parse addr")
	}
	return addrs[0], addrs[1]
}

func parseOptCmd(cmd string) (string, []string) {
	cmds := strings.Split(cmd, " ")
	if len(cmds[0]) == 0 || len(cmds) < 1 {
		printUsage()
		log.Fatal("Unable to parse cmd. cmd is required!")
	}

	args := make([]string, 0)
	for _, arg := range cmds[1:] {
		if arg == "" || arg == " " {
			continue
		}
		args = append(args, arg)
	}

	return cmds[0], args
}

func printUsage() {
	fmt.Println("Usage: tcpserver [--addr 0.0.0.0:3000] --cmd 'tr a-z A-Z'")
	flag.PrintDefaults()
	fmt.Println()
}

func getListener(conf config) net.Listener {
	addr := conf.Bind + ":" + conf.Port
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(fmt.Sprintf("Unable to listen to address: [%v] ERROR: %v", addr, err))
	}
	debug("Listenning to: %v", ln.Addr())
	return ln
}

func run(conf config, listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			errr("Something went wrong while connecting! ERROR: %v", err)
		} else {
			go handleConn(conn, conf)
		}
	}
}

func getCmd(conf config) *exec.Cmd {
	cmd := exec.Command(conf.Cmd)
	for _, arg := range conf.CmdArgs {
		cmd.Args = append(cmd.Args, arg)
	}
	debug("cmd: %s", cmd.Args)
	return cmd
}

func handleConn(conn net.Conn, conf config) {
	defer conn.Close()

	cmd := getCmd(conf)
	remote := conn.RemoteAddr()
	from := fmt.Sprintf("%s ", remote)
	debug("Accepted connection from: %v", remote)

	cnt := 0
	var prev string
	var str string
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		curr := sc.Text()
		debug("%s-> data: [%s]", from, curr)
		if cnt > 0 &&
			strings.Compare(prev, "") == 0 &&
			strings.Compare(curr, prev) == 0 {
			runCmd(cmd, conn, str)
			return
		}
		str += fmt.Sprintf("%s\n", curr)
		prev = curr
		cnt++
	}
	if err := sc.Err(); err != nil {
		errr("reading err [%v]", err)
	}

	debug(from + "XX ... connection closed")
}

func runCmd(cmd *exec.Cmd, conn net.Conn, str string) {
	from := fmt.Sprintf("%s", conn.RemoteAddr())
	debug("echo '%s' | %s", str, opt.Cmd)

	cmd.Stdin = strings.NewReader(str)
	cmd.Stdout = conn
	cmd.Stderr = conn

	err := cmd.Run()
	if err != nil {
		errr("%s !! Failed to exec! ERROR: %s\n", from, err)
		errstr := strings.NewReader(fmt.Sprintf("err: %s\n", err))
		io.Copy(conn, errstr)
		return
	}

	debug("%s <- ... Ran cmd", from)
	debug("!! ... %s", cmd.ProcessState)
}

func debug(pattern string, args ...interface{}) {
	if !deb {
		return
	}
	pattern = "[debug] " + pattern
	log.Printf(pattern, args...)
}

func errr(pattern string, args ...interface{}) {
	pattern = "[error] " + pattern
	log.Printf(pattern, args...)
}
