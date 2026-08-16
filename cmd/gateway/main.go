package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"arham-gateway/internal/app"
	"arham-gateway/internal/service"
)

var (
	Version   = "1.0.0"
	BuildDate = "2026-08-16"
)

func printUsage() {
	fmt.Println("Arham Gateway - Fast, Self-Hosted OpenAI-Compatible Model Gateway")
	fmt.Printf("Version: %s (%s)\n\n", Version, BuildDate)
	fmt.Println("Usage:")
	fmt.Println("  gateway <command> [flags]")
	fmt.Println("\nCommands:")
	fmt.Println("  serve           Run the gateway HTTP server in the foreground")
	fmt.Println("  setup           Initialize directories, master key, database and admin password")
	fmt.Println("  start           Start the gateway service via systemd")
	fmt.Println("  stop            Stop the gateway service via systemd")
	fmt.Println("  restart         Restart the gateway service via systemd")
	fmt.Println("  status          Check local listener and service status")
	fmt.Println("  password-reset  Prompt and reset the admin dashboard password")
	fmt.Println("  version         Display gateway binary version")
	fmt.Println("\nFlags:")
	fmt.Println("  -config string  Path to config.toml (default: /etc/arham-gateway/config.toml or ./config.toml)")
}

func main() {
	cmd := "serve"
	subArgs := os.Args[1:]
	if len(os.Args) >= 2 {
		if !strings.HasPrefix(os.Args[1], "-") {
			cmd = os.Args[1]
			subArgs = os.Args[2:]
		} else if os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "-v" || os.Args[1] == "--version" {
			cmd = os.Args[1]
			subArgs = os.Args[2:]
		}
	}

	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	configPath := fs.String("config", "", "Path to configuration TOML file")
	_ = fs.Parse(subArgs)

	// Fallback config discovery
	resolvedConfig := *configPath
	if resolvedConfig == "" {
		if _, err := os.Stat("/etc/arham-gateway/config.toml"); err == nil {
			resolvedConfig = "/etc/arham-gateway/config.toml"
		} else if _, err := os.Stat("config.toml"); err == nil {
			resolvedConfig = "config.toml"
		}
	}

	switch cmd {
	case "serve":
		runServe(resolvedConfig)
	case "setup":
		if err := service.RunSetup(resolvedConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Setup error: %v\n", err)
			os.Exit(1)
		}
	case "start":
		if err := service.RunServiceCommand("start"); err != nil {
			fmt.Fprintf(os.Stderr, "Start error: %v\n", err)
			os.Exit(1)
		}
	case "stop":
		if err := service.RunServiceCommand("stop"); err != nil {
			fmt.Fprintf(os.Stderr, "Stop error: %v\n", err)
			os.Exit(1)
		}
	case "restart":
		if err := service.RunServiceCommand("restart"); err != nil {
			fmt.Fprintf(os.Stderr, "Restart error: %v\n", err)
			os.Exit(1)
		}
	case "status":
		if err := service.RunStatus(resolvedConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Status error: %v\n", err)
			os.Exit(1)
		}
	case "password-reset":
		if err := service.RunPasswordReset(resolvedConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Password reset error: %v\n", err)
			os.Exit(1)
		}
	case "version", "--version", "-v":
		fmt.Printf("Arham Gateway v%s (%s)\n", Version, BuildDate)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func runServe(configPath string) {
	application, err := app.New(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize application: %v\n", err)
		os.Exit(1)
	}

	if err := application.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Application runtime error: %v\n", err)
		os.Exit(1)
	}
}
