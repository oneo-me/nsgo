package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The notary service checks a Developer ID build for malware and issues a
// ticket that Gatekeeper accepts, so a downloaded app opens without
// warnings. nsgo build submits the disk image, or the app zipped without
// one, with notarytool, waits for the verdict and staples the ticket to
// what it submitted and to the app, so they open offline too.

// notaryKeyEnv are the environment variables of an App Store Connect API
// key: the .p8 file, its ID and its issuer.
var notaryKeyEnv = [3]string{"NSGO_NOTARY_KEY", "NSGO_NOTARY_KEY_ID", "NSGO_NOTARY_ISSUER"}

// notarizing reports whether nsgo build notarizes the macOS package: it
// has a keychain profile or an API key to do it with.
func (p *project) notarizing() bool {
	return p.Release && (p.Sign.Notarize != "" || os.Getenv(notaryKeyEnv[0]) != "")
}

// notaryCredentials are notarytool's arguments naming the credentials: the
// API key the environment gives, else sign.notarize's keychain profile.
func (p *project) notaryCredentials() ([]string, error) {
	if key := os.Getenv(notaryKeyEnv[0]); key != "" {
		id, issuer := os.Getenv(notaryKeyEnv[1]), os.Getenv(notaryKeyEnv[2])
		if id == "" || issuer == "" {
			return nil, fmt.Errorf("%s needs %s and %s", notaryKeyEnv[0], notaryKeyEnv[1], notaryKeyEnv[2])
		}
		return []string{"--key", key, "--key-id", id, "--issuer", issuer}, nil
	}
	return []string{"--keychain-profile", p.Sign.Notarize}, nil
}

// checkNotarizing refuses to notarize what the service would refuse, an
// app without a Developer ID signature, before anything is built.
func (p *project) checkNotarizing() error {
	if !p.notarizing() {
		return nil
	}
	if p.Sign.Identity == "" || p.Sign.Identity == "-" {
		return errors.New("notarizing needs sign.identity, a Developer ID Application identity")
	}
	return nil
}

// notarizeMac notarizes the macOS package in dir: dmg when there is one,
// else app zipped, stapling the ticket to both.
func notarizeMac(ctx context.Context, p *project, app, dmg string) error {
	if !p.notarizing() {
		return nil
	}
	file := dmg
	if file == "" {
		zip, err := os.CreateTemp("", "nsgo-notarize-*.zip")
		if err != nil {
			return err
		}
		zip.Close()
		defer os.Remove(zip.Name())
		if err := run(ctx, "ditto", "-c", "-k", "--keepParent", app, zip.Name()); err != nil {
			return err
		}
		file = zip.Name()
	}
	if err := notarize(ctx, p, file); err != nil {
		return err
	}
	stage(ctx, "stapling the ticket")
	if err := run(ctx, "xcrun", "stapler", "staple", app); err != nil {
		return err
	}
	if dmg != "" {
		return run(ctx, "xcrun", "stapler", "staple", dmg)
	}
	return nil
}

// notarize submits file to the notary service and waits for its verdict,
// which takes minutes; a rejection comes with the service's log, which
// says what it found.
func notarize(ctx context.Context, p *project, file string) error {
	stage(ctx, "notarizing %s (usually a few minutes)", filepath.Base(file))
	creds, err := p.notaryCredentials()
	if err != nil {
		return err
	}
	args := append([]string{"notarytool", "submit", file, "--wait", "--output-format", "json"}, creds...)
	out, err := exec.CommandContext(ctx, "xcrun", args...).Output()
	result, perr := parseNotaryResult(out)
	if err == nil && perr == nil && result.Status == "Accepted" {
		return nil
	}
	msg := fmt.Sprintf("notarization of %s failed", filepath.Base(file))
	if result.Status != "" {
		msg += ": " + strings.TrimSpace(result.Status+" "+result.Message)
	} else if err != nil {
		msg += ": " + exitMessage(err)
	}
	if result.ID != "" {
		logArgs := append([]string{"notarytool", "log", result.ID}, creds...)
		if log, err := exec.CommandContext(ctx, "xcrun", logArgs...).Output(); err == nil {
			msg += "\n" + strings.TrimSpace(string(log))
		}
	}
	return errors.New(msg)
}

// notaryResult is what notarytool submit prints as JSON.
type notaryResult struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// parseNotaryResult reads notarytool's JSON, the last object it prints.
func parseNotaryResult(out []byte) (notaryResult, error) {
	var r notaryResult
	i := strings.LastIndexByte(string(out), '{')
	if i < 0 {
		return r, errors.New("notarytool printed no result")
	}
	return r, json.Unmarshal(out[i:], &r)
}

// exitMessage is a failed command's error with what it wrote to stderr.
func exitMessage(err error) string {
	var exit *exec.ExitError
	if errors.As(err, &exit) && len(exit.Stderr) > 0 {
		return strings.TrimSpace(string(exit.Stderr))
	}
	return err.Error()
}
