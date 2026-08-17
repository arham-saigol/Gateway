package service

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"arham-gateway/internal/app"
	"arham-gateway/internal/auth"
	"arham-gateway/internal/config"
	"arham-gateway/internal/crypto"
	"arham-gateway/internal/database"
	"golang.org/x/term"
)

const SystemdUnitContent = `[Unit]
Description=Arham Gateway OpenAI-Compatible Model Gateway
After=network.target

[Service]
Type=simple
User=arham-gateway
Group=arham-gateway
ExecStart=/usr/local/bin/gateway serve
Restart=always
RestartSec=5s
LimitNOFILE=65536
NoNewPrivileges=true
ProtectSystem=full
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
`

func PromptPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		bytePassword, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(bytePassword)), nil
	}

	// Non-terminal fallback
	reader := bufio.NewReader(os.Stdin)
	pass, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(pass), nil
}

func ensureServiceAccountAndOwnership(cfg *config.Config) error {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return nil
	}

	userName := "arham-gateway"
	u, err := user.Lookup(userName)
	if err != nil {
		fmt.Printf("Creating system user '%s'...\n", userName)
		cmd := exec.Command("useradd", "-r", "-U", "-s", "/usr/sbin/nologin", "-M", userName)
		if _, err := cmd.CombinedOutput(); err != nil {
			// Fallback if /usr/sbin/nologin doesn't exist
			cmd = exec.Command("useradd", "-r", "-s", "/bin/false", "-M", userName)
			_ = cmd.Run()
		}
		u, err = user.Lookup(userName)
		if err != nil {
			return fmt.Errorf("looking up user %s after creation: %w", userName, err)
		}
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("invalid uid %s: %w", u.Uid, err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("invalid gid %s: %w", u.Gid, err)
	}

	pathsToChown := []string{
		filepath.Dir(cfg.Security.MasterKeyPath),
		cfg.Security.MasterKeyPath,
		filepath.Dir(cfg.Database.Path),
		cfg.Database.Path,
		cfg.Database.Path + "-wal",
		cfg.Database.Path + "-shm",
	}

	for _, p := range pathsToChown {
		if _, err := os.Stat(p); err == nil {
			_ = os.Chown(p, uid, gid)
		}
	}

	return nil
}

func RunSetup(configPath string) error {
	fmt.Println("=== Arham Gateway Setup ===")

	cfg := config.DefaultConfig()
	if configPath != "" {
		loaded, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("loading config %s: %w", configPath, err)
		}
		cfg = *loaded
	}

	// 1. Create directories
	masterKeyDir := filepath.Dir(cfg.Security.MasterKeyPath)
	dbDir := filepath.Dir(cfg.Database.Path)

	_ = os.MkdirAll(masterKeyDir, 0700)
	_ = os.MkdirAll(dbDir, 0700)

	// 2. Master Key
	if _, err := crypto.LoadMasterKey(cfg.Security.MasterKeyPath); err != nil {
		fmt.Printf("Generating random master encryption key at %s...\n", cfg.Security.MasterKeyPath)
		if _, err := crypto.GenerateAndSaveMasterKey(cfg.Security.MasterKeyPath); err != nil {
			return fmt.Errorf("creating master key: %w", err)
		}
	} else {
		fmt.Printf("Master key exists at %s.\n", cfg.Security.MasterKeyPath)
	}

	// 3. Database & Migrations
	fmt.Printf("Initializing database at %s...\n", cfg.Database.Path)
	db, err := database.Open(cfg.Database.Path, cfg.Database.BusyTimeoutMs)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	if err := db.SeedDefaults(); err != nil {
		return fmt.Errorf("seeding defaults: %w", err)
	}

	// 4. Admin Password
	for {
		p1, err := PromptPassword("Enter new dashboard admin password: ")
		if err != nil {
			return fmt.Errorf("reading password: %w", err)
		}
		if len(p1) < 8 {
			fmt.Println("Password must be at least 8 characters long.")
			continue
		}

		p2, err := PromptPassword("Confirm dashboard admin password: ")
		if err != nil {
			return fmt.Errorf("reading password confirmation: %w", err)
		}

		if p1 != p2 {
			fmt.Println("Passwords do not match. Please try again.")
			continue
		}

		hashed, err := auth.HashPassword(p1)
		if err != nil {
			return fmt.Errorf("hashing password: %w", err)
		}

		if err := db.SetAdminPasswordHash(hashed); err != nil {
			return fmt.Errorf("saving admin password: %w", err)
		}
		fmt.Println("Dashboard password saved successfully.")
		break
	}

	// 5. Set service account ownership on Linux
	if err := ensureServiceAccountAndOwnership(&cfg); err != nil {
		fmt.Printf("Warning: setting service account ownership: %v\n", err)
	}

	// 6. Systemd unit installation on Linux
	if runtime.GOOS == "linux" && os.Geteuid() == 0 {
		unitPath := "/etc/systemd/system/gateway.service"
		fmt.Printf("Installing systemd service unit to %s...\n", unitPath)
		_ = os.WriteFile(unitPath, []byte(SystemdUnitContent), 0644)
		_ = exec.Command("systemctl", "daemon-reload").Run()
		_ = exec.Command("systemctl", "enable", "gateway").Run()
		fmt.Println("Systemd service 'gateway' installed and enabled.")

		// Check for cloudflared
		if _, err := exec.LookPath("cloudflared"); err != nil {
			fmt.Println("\nCloudflared not detected. To install:")
			fmt.Println("  curl -L --output cloudflared.deb https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb")
			fmt.Println("  sudo dpkg -i cloudflared.deb")
		}
	}

	fmt.Println("\nSetup complete! Next steps:")
	fmt.Println("1. Start service: gateway start (or 'gateway serve' for foreground)")
	fmt.Println("2. Access local dashboard: http://127.0.0.1:8080")
	fmt.Println("3. Configure Cloudflare Tunnel to forward public domain traffic to http://127.0.0.1:8080")
	return nil
}

func RunPasswordReset(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	db, err := database.Open(cfg.Database.Path, cfg.Database.BusyTimeoutMs)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	for {
		p1, err := PromptPassword("Enter new admin password: ")
		if err != nil {
			return err
		}
		if len(p1) < 8 {
			fmt.Println("Password must be at least 8 characters long.")
			continue
		}

		p2, err := PromptPassword("Confirm new admin password: ")
		if err != nil {
			return err
		}

		if p1 != p2 {
			fmt.Println("Passwords do not match.")
			continue
		}

		hash, err := auth.HashPassword(p1)
		if err != nil {
			return err
		}

		if err := db.SetAdminPasswordHash(hash); err != nil {
			return err
		}

		_ = db.RevokeAdminSessions()
		fmt.Println("Password reset successfully. All active sessions have been revoked.")
		return nil
	}
}

func RunServiceCommand(action string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("systemd service control is supported on Linux (use 'gateway serve' for foreground mode)")
	}

	cmd := exec.Command("systemctl", action, "gateway")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunStatus(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	status, err := app.CheckLocalStatus(cfg.Server.ListenAddr)
	fmt.Printf("Arham Gateway Local Listener (%s): %s\n", cfg.Server.ListenAddr, status)

	if runtime.GOOS == "linux" {
		fmt.Println("\nSystemd Service Status:")
		cmd := exec.Command("systemctl", "status", "gateway", "--no-pager")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}

	return nil
}
