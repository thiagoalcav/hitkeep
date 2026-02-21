package hitkeepcmd

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"hitkeep/internal/config"
	"hitkeep/internal/database"
)

// Recover handles the "hitkeep recover <subcommand>" family of commands.
// These are offline recovery operations that require HitKeep to be stopped
// (DuckDB allows only one writer at a time).
func Recover() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, recoverUsage)
		os.Exit(1)
	}

	switch os.Args[2] {
	case "disable-2fa":
		recoverDisable2FA(os.Args[3:])
	default:
		//nolint:gosec // G705: writes to stderr, not an HTTP response; %q safely quotes the argument.
		fmt.Fprintf(os.Stderr, "Unknown recover subcommand: %q\n\n%s\n", os.Args[2], recoverUsage)
		os.Exit(1)
	}
}

const recoverUsage = `Usage: hitkeep recover <subcommand> [flags]

Subcommands:
  disable-2fa   Remove all 2FA methods (TOTP + passkeys) for a user.
                Allows the user to log in with email/password again.

Flags for disable-2fa:
  -email string   User email address (required)
  -db    string   Path to hitkeep.db (default: same as server config)
  -yes            Skip interactive confirmation prompt

NOTE: HitKeep must be stopped before running recovery commands.
      DuckDB does not allow concurrent write access.`

func recoverDisable2FA(args []string) {
	fs := flag.NewFlagSet("disable-2fa", flag.ExitOnError)
	email := fs.String("email", "", "User email address (required)")
	dbPath := fs.String("db", "", "Path to hitkeep.db (defaults to server config value)")
	yes := fs.Bool("yes", false, "Skip confirmation prompt")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *email == "" {
		fmt.Fprintln(os.Stderr, "Error: -email is required")
		fmt.Fprintln(os.Stderr)
		fs.Usage()
		os.Exit(1)
	}

	// Resolve DB path: flag overrides config default
	if *dbPath == "" {
		conf := config.Load()
		*dbPath = conf.DBPath
	}

	ctx := context.Background()

	// ---- Connect --------------------------------------------------------
	fmt.Printf("HitKeep Recovery — Disable 2FA\n")
	fmt.Printf("================================\n")
	fmt.Printf("DB:    %s\n", *dbPath)
	fmt.Printf("User:  %s\n\n", *email)

	store := database.NewStore(*dbPath)
	if err := store.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not open database: %v\n", err)
		fmt.Fprintln(os.Stderr, "Make sure HitKeep is stopped before running recovery commands.")
		os.Exit(1)
	}
	defer store.Close()

	if err := store.Migrate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: migration failed: %v\n", err)
		os.Exit(1)
	}

	// ---- Look up user ---------------------------------------------------
	user, err := store.GetUserByEmail(ctx, *email)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: database lookup failed: %v\n", err)
		os.Exit(1)
	}
	if user == nil {
		fmt.Fprintf(os.Stderr, "Error: no user found with email %q\n", *email)
		os.Exit(1)
	}

	name := strings.TrimSpace(user.GivenName + " " + user.LastName)
	if name == "" {
		name = "(no name set)"
	}
	fmt.Printf("Found user: %s (%s)\n\n", name, user.Email)

	// ---- Inventory active 2FA ------------------------------------------
	hasTOTP, err := store.HasEnabledTOTP(ctx, user.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not check TOTP status: %v\n", err)
		os.Exit(1)
	}

	passkeys, err := store.ListUserPasskeys(ctx, user.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not list passkeys: %v\n", err)
		os.Exit(1)
	}

	if !hasTOTP && len(passkeys) == 0 {
		fmt.Println("No 2FA methods are active for this user. Nothing to do.")
		os.Exit(0)
	}

	fmt.Println("Active 2FA methods:")
	if hasTOTP {
		fmt.Println("  TOTP:     enabled")
	} else {
		fmt.Println("  TOTP:     not enabled")
	}
	if len(passkeys) > 0 {
		fmt.Printf("  Passkeys: %d\n", len(passkeys))
		for _, pk := range passkeys {
			fmt.Printf("    - %s\n", pk.Name)
		}
	} else {
		fmt.Println("  Passkeys: none")
	}

	fmt.Println()
	fmt.Println("This will:")
	if hasTOTP {
		fmt.Println("  • Disable TOTP authenticator")
	}
	if len(passkeys) > 0 {
		fmt.Printf("  • Delete %d passkey(s)\n", len(passkeys))
	}
	fmt.Println("  • Invalidate all remember-me sessions")
	fmt.Println()

	// ---- Confirm -------------------------------------------------------
	if !*yes {
		fmt.Print(`Type "yes" to confirm: `)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		answer := strings.TrimSpace(scanner.Text())
		if answer != "yes" {
			fmt.Println("Aborted.")
			os.Exit(0)
		}
		fmt.Println()
	}

	// ---- Execute -------------------------------------------------------
	exitCode := 0

	if hasTOTP {
		if err := store.DisableUserTOTP(ctx, user.ID); err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not disable TOTP: %v\n", err)
			exitCode = 1
		} else {
			fmt.Println("✓ TOTP disabled")
		}
	}

	for _, pk := range passkeys {
		if err := store.DeleteUserPasskey(ctx, user.ID, pk.ID); err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not delete passkey %q: %v\n", pk.Name, err)
			exitCode = 1
		} else {
			fmt.Printf("✓ Passkey %q deleted\n", pk.Name)
		}
	}

	if err := store.DeleteAllRememberMeTokensForUser(ctx, user.ID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not invalidate sessions: %v\n", err)
		exitCode = 1
	} else {
		fmt.Println("✓ Remember-me sessions invalidated")
	}

	if exitCode == 0 {
		fmt.Printf("\nDone. %s can now log in with email and password.\n", user.Email)
	} else {
		fmt.Fprintln(os.Stderr, "\nRecovery completed with errors (see above).")
	}
	os.Exit(exitCode)
}
