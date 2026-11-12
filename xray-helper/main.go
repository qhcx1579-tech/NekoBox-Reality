package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/xtls/xray-core/core"
	_ "github.com/xtls/xray-core/main/distro/all"
)

func run() error {
	configPath := flag.String("config", "", "Xray JSON configuration")
	check := flag.Bool("test", false, "Validate configuration without starting")
	version := flag.Bool("version", false, "Print core version")
	flag.Parse()
	if *version {
		fmt.Println("NekoBox XHTTP helper / Xray", core.Version())
		return nil
	}
	if *configPath == "" {
		return fmt.Errorf("a configuration file is required")
	}
	data, err := os.ReadFile(*configPath)
	if err != nil {
		return fmt.Errorf("cannot read configuration: %w", err)
	}
	config, err := core.LoadConfig("json", bytes.NewReader(data))
	if err != nil {
		// Do not include parser errors that could quote node credentials.
		return fmt.Errorf("invalid Xray configuration")
	}
	if *check {
		return nil
	}
	instance, err := core.New(config)
	if err != nil {
		return fmt.Errorf("cannot initialize Xray")
	}
	defer instance.Close()
	if err = instance.Start(); err != nil {
		return fmt.Errorf("cannot start Xray")
	}
	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stopped)
	<-stopped
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
