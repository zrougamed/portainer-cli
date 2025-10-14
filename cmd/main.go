package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/zrougamed/portainer-cli/internal/api"
	"github.com/zrougamed/portainer-cli/internal/config"
	"github.com/zrougamed/portainer-cli/internal/tui"
)

var (
	cfgURL   string
	cfgToken string
	cfgKey   string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "portainer-tui",
	Short: "⚓ A terminal UI for Portainer",
	Long: `portainer-tui — Interactive TUI for managing Portainer Open Source.

Supports Environments, Containers, Stacks, Images, and Volumes.
Authenticates via JWT token, username/password, or X-API-Key.

Config file: ~/.config/portainer-tui/config.yaml
Env vars:    PORTAINER_URL, PORTAINER_TOKEN, PORTAINER_API_KEY`,
	RunE: runTUI,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgURL, "url", "", "Portainer URL (e.g. http://localhost:9000)")
	rootCmd.PersistentFlags().StringVar(&cfgToken, "token", "", "Portainer JWT token or API key")
	rootCmd.PersistentFlags().StringVar(&cfgKey, "api-key", "", "Portainer community X-API-Key")

	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
}

func buildClient() (*api.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	// Flag overrides
	if cfgURL != "" {
		cfg.URL = cfgURL
	}
	if cfgToken != "" {
		cfg.Token = cfgToken
	}
	if cfgKey != "" {
		cfg.APIKey = cfgKey
	}

	// Use API key if token not set
	token := cfg.Token
	if token == "" {
		token = cfg.APIKey
	}
	if token == "" && cfg.Username != "" {
		// Authenticate with username/password
		client := api.NewClient(cfg.URL, "")
		if err := client.Authenticate(cfg.Username, cfg.Password); err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
		return client, nil
	}
	if token == "" {
		return nil, fmt.Errorf("no token or credentials found. Run 'portainer-tui login' first")
	}

	return api.NewClient(cfg.URL, token), nil
}

func runTUI(cmd *cobra.Command, args []string) error {
	client, err := buildClient()
	if err != nil {
		return err
	}

	app := tui.NewApp(client)
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}

// ─── login command ─────────────────────────────────────────────────────────────

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Portainer and save token",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		url := cfg.URL
		if cfgURL != "" {
			url = cfgURL
		}

		var username, password string
		fmt.Printf("Portainer URL [%s]: ", url)
		fmt.Scanln(&url)
		if url == "" {
			url = cfg.URL
		}

		fmt.Print("Username: ")
		fmt.Scanln(&username)
		fmt.Print("Password: ")
		fmt.Scanln(&password)

		client := api.NewClient(url, "")
		if err := client.Authenticate(username, password); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		if err := config.WriteDefault(url, client.Token); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Println("✓ Logged in successfully. Token saved to ~/.config/portainer-tui/config.yaml")
		return nil
	},
}

// ─── open command (open Portainer in browser) ──────────────────────────────────

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open Portainer in the default browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := buildClient()
		if err != nil {
			return err
		}
		url := client.OpenURL()
		fmt.Printf("Opening %s ...\n", url)
		return openBrowser(url)
	},
}

// ─── config command ────────────────────────────────────────────────────────────

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show current config",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		token := cfg.Token
		if len(token) > 10 {
			token = token[:10] + "..."
		}
		fmt.Printf("URL:     %s\n", cfg.URL)
		fmt.Printf("Token:   %s\n", token)
		fmt.Printf("API Key: %s\n", cfg.APIKey)
		return nil
	},
}

// ─── version command ───────────────────────────────────────────────────────────

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("portainer-tui v0.1.0")
	},
}
