package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// startStdinInput reads credential input from stdin for testing.
// Format: <credential_type> <credential_data> <lock_id>
// Example: nfc_uid UID-1001 door_jkt_001
// BLE:    ble <user_id> <nonce_hex> <sig_hex> [lock_id]
func startStdinInput(agent *Agent, logger *slog.Logger) {
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Println()
		fmt.Println("=== Credential Input (stdin) ===")
		fmt.Println("Format: <type> <data> <lock_id>")
		fmt.Println("Example: nfc_uid UID-1001 door_jkt_001")
		fmt.Println("BLE:    ble <user_id> <nonce_hex> <sig_hex> [lock_id]")
		fmt.Println("Type 'rules' to show cached access rules")
		fmt.Println("Type 'quit' to exit")
		fmt.Println()

		for {
			fmt.Print("card> ")
			if !scanner.Scan() {
				return
			}
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if line == "quit" || line == "exit" {
				fmt.Println("Exiting...")
				os.Exit(0)
			}
			if line == "rules" {
				agent.mu.RLock()
				fmt.Printf("Cached rules (version=%s, count=%d):\n", agent.ruleVersion, len(agent.accessRules))
				for i, rule := range agent.accessRules {
					fmt.Printf("  [%d] %s:%s → %s (%s) → locks: %v\n",
						i, rule.CredentialType, rule.CredentialData,
						rule.UserID, rule.UserEmail, rule.LockIDs)
				}
				rulesEmpty := len(agent.accessRules) == 0
				agent.mu.RUnlock()
				if rulesEmpty {
					fmt.Println("  (no rules cached, waiting for config pull)")
				}
				fmt.Println()
				continue
			}

			parts := strings.Fields(line)

			// Handle BLE simulation command
			if len(parts) >= 1 && parts[0] == "ble" {
				handleBLESimCommand(agent, logger, parts)
				fmt.Println()
				continue
			}

			if len(parts) < 3 {
				fmt.Println("Usage: <credential_type> <credential_data> <lock_id>")
				fmt.Println("Example: nfc_uid UID-1001 door_jkt_001")
				fmt.Println("BLE:    ble <user_id> <nonce_hex> <sig_hex> [lock_id]")
				continue
			}

			credType := parts[0]
			credData := parts[1]
			lockID := parts[2]

			logger.Info("credential presented",
				"type", credType,
				"data", credData,
				"lock_id", lockID,
			)
			agent.HandleCredentialPresented(credType, credData, lockID)
			fmt.Println()
		}
	}()
}
