---
title: 示例：GitHub 热门仓库
description: 从 GitHub API 列出 star 最多的 Go 仓库，在 goroutine 中加载，并显示加载指示、错误和重试。
group: 入门
order: 13
---

本教程构建一个列出 GitHub 上 star 最多的 Go 仓库的应用。它涵盖了每个与服务器通信的应用都需要的东西：不阻塞界面的请求、加载指示、可以重试的错误，以及不依赖网络的测试。开始前先完成[环境准备](../getting-started.md)。

## 创建应用

```sh
mkdir repos && cd repos
go mod init example.com/repos
go get github.com/oneo-me/nsgo
go get -tool github.com/oneo-me/nsgo/cmd/nsgo
go tool nsgo init
go tool nsgo dev
```

## 编写应用

把 `app.go` 整个替换为：

```go
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

请求进行时显示加载指示，随后列表填充出来。

## 工作原理

**Go 自己的网络库。** `net/http` 在每个目标上都能用，并使用系统的 TLS 和代理设置，所以 `fetch` 就是普通的 Go 代码。

**不在引擎的 goroutine 上请求。** 组件和 signal 属于引擎的 goroutine，它绝不能等待网络。`load` 用 `go` 发起请求，`cx.Post` 把结果送回引擎，在那里设置 signal。

**每种状态一个 signal。** `loading`、`failure` 和 `repos` 各保存状态的一部分。`core.Show` 在加载时显示加载指示，然后显示带**重试**按钮的错误或列表；条件变化时只重建那一部分。

**任意长度的列表。** `core.List` 只构建靠近屏幕的行，并以仓库名作为 key，所以新的结果会复用同一仓库的行。

## 测试

请求的 URL 是一个变量，测试用 `httptest` 把它指向本地服务器，从而控制“网络”的回答：

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

`WaitFor` 会一直等到回答到达、界面显示出来。

## 下一步

- [平台服务](../services.md#网络)：更多关于网络、存储和文件的内容。
- [列表与滚动](../lists.md)：下拉刷新、吸附和长列表。
- [运行与调试](../debugging.md)：用 `-trace` 观察每次更新。
