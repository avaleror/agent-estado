package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/avaleror/overto/internal/pass"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "intro" {
		os.Exit(cmdIntro(os.Args[2:]))
	}
	fs := flag.NewFlagSet("overto-pass", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	direct := fs.String("direct", "", "address for a path that already routes")
	intro := fs.String("intro", "", "introduction point URL")
	_ = fs.Bool("replace", false, "ignored; the shell decides")
	_ = fs.Bool("send-secrets", false, "ignored; the shell decides")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	err := pass.Run(ctx, pass.Config{
		Role:   os.Getenv("OVERTO_ROLE"),
		RunDir: os.Getenv("OVERTO_RUN"),
		Direct: *direct,
		Intro:  *intro,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "overto-pass: %s\n", err.Error())
		os.Exit(1)
	}
}

func cmdIntro(args []string) int {
	fs := flag.NewFlagSet("intro", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	httpAddr := fs.String("http", "127.0.0.1:0", "HTTP listen address")
	udpAddr := fs.String("udp", "", "UDP map address, default is the HTTP port")
	relayAddr := fs.String("relay", "127.0.0.1:0", "relay listen address")
	cert := fs.String("cert", "", "TLS certificate")
	key := fs.String("key", "", "TLS private key")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	_, httpShow, udpShow, relayShow, err := pass.ListenIntro(*httpAddr, *udpAddr, *relayAddr, *cert, *key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "overto-pass: %s\n", err.Error())
		return 1
	}
	fmt.Printf("overto-pass intro http=%s udp=%s relay=%s\n", httpShow, udpShow, relayShow)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	return 0
}
