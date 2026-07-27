package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type config struct {
	addr      string
	cmd       string
	cmdArgs   []string
	ioTimeout time.Duration
	quiet     bool
	debug     bool
}

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	level := slog.LevelInfo
	if cfg.quiet {
		level = slog.LevelWarn
	}
	if cfg.debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	if err := run(context.Background(), cfg, logger); err != nil {
		logger.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func parseFlags(args []string) (config, error) {
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var cfg config
	var cmdStr string
	fs.BoolVar(&cfg.quiet, "q", false, "Set log level to warn")
	fs.BoolVar(&cfg.quiet, "quiet", false, "Set log level to warn")
	fs.BoolVar(&cfg.debug, "debug", false, "Set log level to debug")
	fs.StringVar(&cfg.addr, "addr", "127.0.0.1:3131", "IP address to bind")
	fs.StringVar(&cmdStr, "cmd", "", "Command to run")
	fs.DurationVar(&cfg.ioTimeout, "io-timeout", 0, "IO timeout as a duration string (e.g. 2s, 500ms; default: no timeout)")

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}

	cmd, cmdArgs, err := parseCmd(cmdStr)
	if err != nil {
		printUsage(fs)
		return config{}, err
	}
	cfg.cmd = cmd
	cfg.cmdArgs = cmdArgs
	return cfg, nil
}

func parseCmd(cmd string) (string, []string, error) {
	cmd = strings.TrimSpace(cmd)
	cmds := strings.Split(cmd, " ")
	if cmds[0] == "" {
		return "", nil, errors.New("cmd is required")
	}

	args := make([]string, 0, len(cmds)-1)
	for _, arg := range cmds[1:] {
		if arg == "" {
			continue
		}
		args = append(args, arg)
	}
	return cmds[0], args, nil
}

func printUsage(fs *flag.FlagSet) {
	fmt.Fprintf(fs.Output(), "Usage: %s [-q|--quiet] [--debug] [-io-timeout 10s] [--addr 127.0.0.1:3131] --cmd 'tr a-z A-Z'\n", fs.Name())
	fs.PrintDefaults()
	fmt.Fprintln(fs.Output())
}

func listen(addr string) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}
	return ln, nil
}

func run(ctx context.Context, cfg config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	ln, err := listen(cfg.addr)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	log.Info("listening", "addr", ln.Addr())
	log.Debug("config",
		"addr", cfg.addr,
		"cmd", cfg.cmd,
		"cmdArgs", cfg.cmdArgs,
		"ioTimeout", cfg.ioTimeout,
	)

	var wg sync.WaitGroup
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			log.Error("accept failed", "err", err)
			continue
		}

		if cfg.ioTimeout > 0 {
			deadline := time.Now().Add(cfg.ioTimeout)
			_ = conn.SetReadDeadline(deadline)
		}

		c := conn
		wg.Go(func() {
			handleConn(c, cfg, log)
		})
	}

	wg.Wait()
	return nil
}

func newCmd(cfg config) *exec.Cmd {
	return exec.Command(cfg.cmd, cfg.cmdArgs...)
}

func handleConn(conn net.Conn, cfg config, log *slog.Logger) {
	defer conn.Close()

	start := time.Now()
	remote := conn.RemoteAddr()
	log = log.With("remote", remote)
	log.Debug("accepted connection")

	var (
		inputLen int
		recvDur  time.Duration
		procDur  time.Duration
		state    string
		cmdErr   error
		readErr  error
		ran      bool
	)
	defer func() {
		args := []any{
			"state", state,
			"inputLen", inputLen,
			"process", procDur,
			"recv", recvDur,
			"duration", time.Since(start),
		}
		if !ran {
			if readErr == nil {
				log.Debug("incomplete connection", args...)
			}
			return
		}
		if cmdErr != nil {
			log.Warn("handled connection", append(args, "err", cmdErr)...)
			return
		}
		log.Info("handled connection", args...)
	}()

	cmd := newCmd(cfg)
	log.Debug("cmd", "args", cmd.Args)

	var (
		cnt  int
		prev string
		b    strings.Builder
	)
	recvStart := time.Now()
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		curr := sc.Text()
		log.Debug("data", "lineLen", len(curr))
		if cnt > 0 && prev == "" && curr == prev {
			recvDur = time.Since(recvStart)
			input := b.String()
			inputLen = len(input)

			ran = true
			procStart := time.Now()
			var ps *os.ProcessState
			ps, cmdErr = runCmd(cmd, conn, input, log)
			procDur = time.Since(procStart)
			if ps != nil {
				state = ps.String()
			}
			return
		}
		b.WriteString(curr)
		b.WriteByte('\n')
		prev = curr
		cnt++
	}
	recvDur = time.Since(recvStart)
	inputLen = b.Len()
	if err := sc.Err(); err != nil {
		readErr = err
		log.Error("reading", "err", err)
	}
}

func runCmd(cmd *exec.Cmd, conn net.Conn, input string, log *slog.Logger) (*os.ProcessState, error) {
	log.Debug("running command", "inputLen", len(input), "cmd", cmd.Args)

	cmd.Stdin = strings.NewReader(input)
	cmd.Stdout = conn
	cmd.Stderr = conn

	if err := cmd.Run(); err != nil {
		log.Error("command failed", "err", err)
		_, _ = io.Copy(conn, strings.NewReader(fmt.Sprintf("err: %s\n", err)))
		return cmd.ProcessState, err
	}

	log.Debug("command finished", "state", cmd.ProcessState)
	return cmd.ProcessState, nil
}
