// Command ota-sign generates the OTA signing key pair and signs gateway-agent
// firmware. The private key never leaves the machine this runs on (offline
// signing): the API/staging server only ever sees the signature, never the key.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/mistypass/cloud/api/internal/otasig"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "gen-key":
		if err := runGenKey(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "gen-key:", err)
			os.Exit(1)
		}
	case "sign":
		if err := runSign(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "sign:", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  ota-sign gen-key --out-priv priv.pem --out-pub pub.hex")
	fmt.Fprintln(os.Stderr, "  ota-sign sign --key priv.pem --version <ver> --in <binary> [--gateway <id>] [--tenant <id>] [--url <url>]")
}

func runGenKey(args []string) error {
	fs := flag.NewFlagSet("gen-key", flag.ExitOnError)
	outPriv := fs.String("out-priv", "ota-priv.pem", "output path for PKCS#8 private key PEM")
	outPub := fs.String("out-pub", "ota-pub.hex", "output path for hex public key")
	_ = fs.Parse(args)

	pub, priv, err := otasig.GenerateKey()
	if err != nil {
		return err
	}
	pemBytes, err := otasig.MarshalPrivateKeyPEM(priv)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*outPriv, pemBytes, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(*outPub, []byte(otasig.MarshalPublicKeyHex(pub)+"\n"), 0o600); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "private key: %s  (keep OFFLINE — never copy to the API/staging server)\n", *outPriv)
	fmt.Fprintf(os.Stderr, "public key:  %s\n", *outPub)
	fmt.Fprintf(os.Stderr, "pin on agent: --ota-pubkey %s\n", otasig.MarshalPublicKeyHex(pub))
	return nil
}

// signFile is the testable core: hash + sign with the PEM private key.
func signFile(privPEM []byte, version string, data []byte) (sha, sig string, err error) {
	if version == "" {
		return "", "", errors.New("version must not be empty")
	}
	priv, err := otasig.ParsePrivateKeyPEM(privPEM)
	if err != nil {
		return "", "", err
	}
	sha = otasig.SHA256Hex(data)
	sig = otasig.Sign(priv, version, sha)
	return sha, sig, nil
}

func runSign(args []string) error {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	keyPath := fs.String("key", "", "path to PKCS#8 Ed25519 private key PEM")
	version := fs.String("version", "", "firmware version (e.g. 1.4.0)")
	in := fs.String("in", "", "path to firmware binary to sign")
	gateway := fs.String("gateway", "", "target gateway id (for printed task JSON)")
	tenant := fs.String("tenant", "", "tenant id (for printed task JSON)")
	url := fs.String("url", "", "firmware download URL (for printed task JSON)")
	_ = fs.Parse(args)

	if *keyPath == "" || *version == "" || *in == "" {
		return fmt.Errorf("--key, --version and --in are required")
	}
	privPEM, err := os.ReadFile(*keyPath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	sha, sig, err := signFile(privPEM, *version, data)
	if err != nil {
		return err
	}
	task := map[string]string{
		"tenant_id":          *tenant,
		"firmware_version":   *version,
		"firmware_url":       *url,
		"firmware_sha256":    sha,
		"firmware_signature": sig,
	}
	body, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal task JSON: %w", err)
	}
	fmt.Fprintf(os.Stderr, "firmware_sha256:    %s\n", sha)
	fmt.Fprintf(os.Stderr, "firmware_signature: %s\n", sig)
	fmt.Fprintf(os.Stderr, "\nPOST /api/v1/gateways/%s/ota/tasks\n", *gateway)
	fmt.Printf("%s\n", body)
	return nil
}
