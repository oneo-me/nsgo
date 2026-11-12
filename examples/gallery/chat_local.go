package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
)

// localReply runs one turn of a local CLI, Claude or Codex, on a goroutine:
// text receives its chunks and done its end, an error when it failed,
// both on the engine. Credentials stay in the CLI's own login store. stop
// ends the child.
func localReply(cx *core.Scope, provider, prompt string, text func(chunk string), done func(err string)) (stop func()) {
	tr := locales.For(cx)
	command := "codex"
	args := []string{"exec", "--json", "--skip-git-repo-check", "--ephemeral", "--sandbox", "read-only", "-"}
	if provider == "Claude" {
		command = "claude"
		args = []string{"--print", "--verbose", "--output-format", "stream-json", "--include-partial-messages", "--tools", "", "--strict-mcp-config", "--no-session-persistence"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	stopped := false
	stop = func() { stopped = true; cancel() }
	fail := func(why string) {
		cx.Post(func() {
			if !stopped {
				done(tr.ChatInstallCli(provider, why, command))
			}
		})
	}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = strings.NewReader(prompt)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		cancel()
		go fail(err.Error())
		return stop
	}
	go func() {
		defer cancel()
		output, failure := false, ""
		lines := bufio.NewScanner(out)
		lines.Buffer(make([]byte, 64<<10), 8<<20)
		for lines.Scan() {
			var e struct {
				Type  string `json:"type"`
				Event struct {
					Delta struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"delta"`
				} `json:"event"`
				IsError bool     `json:"is_error"`
				Result  string   `json:"result"`
				Errors  []string `json:"errors"`
				Item    struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"item"`
				Message string `json:"message"`
				Error   struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal(lines.Bytes(), &e) != nil {
				continue
			}
			chunk := ""
			if provider == "Claude" {
				if e.Type == "stream_event" && e.Event.Delta.Type == "text_delta" {
					chunk = e.Event.Delta.Text
				}
				if e.Type == "result" {
					switch {
					case e.IsError:
						failure = e.Result
						if failure == "" {
							failure = strings.Join(e.Errors, "\n")
						}
						if failure == "" {
							failure = tr.ChatClaudeCouldNotCompleteThis()
						}
					case !output:
						chunk = e.Result
					}
				}
			} else {
				if e.Type == "item.completed" && e.Item.Type == "agent_message" {
					chunk = e.Item.Text
				}
				if e.Type == "error" || e.Type == "turn.failed" {
					failure = e.Message
					if failure == "" {
						failure = e.Error.Message
					}
					if failure == "" {
						failure = tr.ChatCodexCouldNotCompleteThis()
					}
				}
			}
			if chunk != "" {
				output = true
				cx.Post(func() {
					if !stopped {
						text(chunk)
					}
				})
			}
		}
		err := cmd.Wait()
		switch {
		case failure != "":
			fail(failure)
		case err != nil:
			why := strings.TrimSpace(stderr.String())
			if why == "" {
				why = err.Error()
			}
			fail(why)
		case !output:
			fail(tr.ChatNoReply())
		default:
			cx.Post(func() {
				if !stopped {
					done("")
				}
			})
		}
	}()
	return stop
}
