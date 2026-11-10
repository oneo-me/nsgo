---
title: "Example: Popular GitHub repositories"
description: List the most-starred Go repositories from GitHub's API, loading on a goroutine with a spinner, errors and a retry.
group: Get started
order: 13
---

This tutorial builds an app that lists the most-starred Go repositories on GitHub. It covers what every app that talks to a server needs: a request that does not block the screen, a loading indicator, an error with a retry, and a test that runs without the network. It assumes the [setup](../getting-started.md).

## Create the app

```sh
mkdir repos && cd repos
go mod init example.com/repos
go get github.com/oneo-me/nsgo
go get -tool github.com/oneo-me/nsgo/cmd/nsgo
go tool nsgo init
go tool nsgo dev
```

## Write the app

Replace the whole of `app.go`:

```go program height=480
package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/ui"
)

// api lists the most-starred Go repositories on GitHub.
var api = "https://api.github.com/search/repositories?q=language:go&sort=stars&per_page=30"

type repo struct {
	Name        string `json:"full_name"`
	Description string `json:"description"`
	Stars       int    `json:"stargazers_count"`
}

// fetch runs on its own goroutine, never the engine's.
func fetch() ([]repo, error) {
	resp, err := http.Get(api)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var body struct {
		Items []repo `json:"items"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	return body.Items, err
}

func main() { nsgo.Run(Root) }

func Root(cx *core.Scope) core.Node {
	repos := core.NewSignalFunc(cx, []repo(nil), nil)
	loading := core.NewSignal(cx, false)
	failure := core.NewSignal(cx, "")
	colors := ui.ColorsOf(cx)

	load := func() {
		loading.Set(true)
		failure.Set("")
		go func() {
			list, err := fetch()
			cx.Post(func() {
				loading.Set(false)
				if err != nil {
					failure.Set(err.Error())
					return
				}
				repos.Set(list)
			})
		}()
	}
	load()

	return core.View{
		Style:     core.Style{FlexGrow: 1, FlexShrink: 1, PaddingEnv: core.EnvAll(core.EnvSafeArea)},
		StyleFunc: func() core.Style { return core.Style{Background: colors.Background()} },
		Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.All(16)}, Children: []core.Node{
				ui.Text("Popular Go repositories", 22, ui.Bold, colors.Text),
				core.Show{When: loading.Get, Then: func(*core.Scope) core.Node { return ui.Spinner{Label: "Loading"} }},
			}},
			core.Show{
				When: func() bool { return failure.Get() != "" },
				Then: func(*core.Scope) core.Node {
					return core.View{Style: core.Style{Padding: core.All(16), Gap: 12, AlignItems: core.AlignStart}, Children: []core.Node{
						ui.Textf(failure.Get, 14, ui.Regular, colors.Muted),
						ui.Button{Text: "Try again", OnPress: load},
					}}
				},
				Else: func(*core.Scope) core.Node {
					return core.List{
						Count:         func() int { return len(repos.Get()) },
						Key:           func(i int) any { return repos.Get()[i].Name },
						EstimatedSize: 64,
						Style:         core.Style{FlexGrow: 1},
						Row: func(cx *core.Scope, row core.ListRow) core.Node {
							r := repos.Get()[row.Index()]
							return core.View{Style: core.Style{Padding: core.Symmetric(10, 16), Gap: 2}, Children: []core.Node{
								core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween, Gap: 8}, Children: []core.Node{
									ui.Text(r.Name, 15, ui.Medium, colors.Text),
									ui.Text(fmt.Sprintf("★ %d", r.Stars), 13, ui.Regular, colors.Muted),
								}},
								ui.Text(r.Description, 13, ui.Regular, colors.Muted),
							}}
						},
					}
				},
			},
		},
	}
}
```

The spinner shows while the request runs, then the list fills in.

## How it works

**Go's own networking.** `net/http` works on every target, with the system's TLS and proxy settings, so `fetch` is plain Go.

**Off the engine's goroutine.** Components and signals belong to the engine's goroutine, which must never wait on the network. `load` starts the request with `go`, and `cx.Post` brings the result back to the engine, where it sets the signals.

**One signal per state.** `loading`, `failure` and `repos` each hold one part of the state. `core.Show` shows the spinner while loading is true, and either the error with **Try again** or the list, rebuilding only that part when its condition changes.

**A list of any length.** `core.List` builds only the rows near the screen, keyed by the repository's name, so a new result reuses the rows of the same repositories.

## Test it

The request's URL is a variable, so a test points it at a local server with `httptest` and controls what the "network" answers:

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

// serve answers the app's requests with body instead of GitHub.
func serve(t *testing.T, status int, body string) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	api = server.URL
}

func TestShowsRepositories(t *testing.T) {
	serve(t, http.StatusOK, `{"items":[{"full_name":"golang/go","description":"The Go programming language","stargazers_count":130000}]}`)
	app := nsgotest.Start(t, Root)
	app.WaitFor("text:golang/go", 5*time.Second)
	app.Find("text:★ 130000")
}

func TestRetriesAfterAFailure(t *testing.T) {
	serve(t, http.StatusServiceUnavailable, "")
	app := nsgotest.Start(t, Root)
	app.WaitFor("text:Try again", 5*time.Second)
}
```

```sh
go tool nsgo test
```

`WaitFor` waits until the answer has arrived and the screen shows it.

## Next

- [Platform services](../services.md#networking): more on networking, storage and files.
- [Lists and scrolling](../lists.md): pull to refresh, snapping and long lists.
- [Running and debugging](../debugging.md): watch each update with `-trace`.
