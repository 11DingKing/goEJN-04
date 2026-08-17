# Bug Reproduction

## 包的性质

当前 test_model_fix 保存的是被测模型修复后的结果源码，不是初始含 Bug 源码。要复现原始缺陷，必须检出下面固定的 parent SHA；不要在当前修复结果源码上期待重新出现修复前失败。生成系统使用的可信验证补丁和完整验证日志仅在本地留存，不提交到结果分支。

## 问题现象

帮我排查一个服务卡死的问题，先不要修改代码。

现场：值班同事用一个写错的电池舱编号调 PUT /api/v1/battery-cabins/bc-missing/telemetry，接口正常返回 404 {"error":"entity not found: bc-missing"}。从这一刻开始，所有写接口全部没有响应，客户端一直挂到自己超时：

  PUT  /api/v1/battery-cabins/bc-1/telemetry  -> 无响应（curl 6s 超时，exit 28）
  POST /api/v1/anomalies                      -> 无响应（curl 6s 超时）
  POST /api/v1/blackstarts                    -> 无响应（curl 6s 超时）

但只读接口完全正常，毫秒级返回 200：GET /api/v1/health、GET /api/v1/loads、GET /api/v1/workorders、GET /api/v1/battery-cabins/bc-1。服务端日志除了启动那一行什么都没有，没有 panic 也没有报错。后台的定时检查也从这时开始不再产出任何结果。重启进程后一切恢复，只要再发一次那个不存在编号的遥测请求就立刻复现；用存在的编号做遥测更新一直正常。

请定位是哪个 Go 文件、哪个符号的什么行为造成上面的现象，说明它如何一步步导致"一次 404 之后所有写接口永久无响应，而只读接口不受影响"，并给出实际的代码阅读或定向复现证据。这一轮只要诊断结论，请先不要改动仓库里的代码。

## 含 Bug 版本

- 仓库：11DingKing/goEJN-04
- 仓库地址：https://github.com/11DingKing/goEJN-04.git
- parent SHA：9ef3a1e23a11311cc13bc816947c0ec00da84fc6

## 复现步骤

```bash
git clone -- https://github.com/11DingKing/goEJN-04.git bug-repro
cd bug-repro
git checkout --detach 9ef3a1e23a11311cc13bc816947c0ec00da84fc6
go test ./... -run "^TestWritesStillWorkAfterTelemetryUpdateForUnknownCabin$" -count=1 -v
```

## 双架构完整错误信息

### linux/amd64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./... -run "^TestWritesStillWorkAfterTelemetryUpdateForUnknownCabin$" -count=1 -v
?   	ejina-microgrid/cmd/server	[no test files]
=== RUN   TestWritesStillWorkAfterTelemetryUpdateForUnknownCabin
    telemetry_unknown_cabin_test.go:51: write operations issued after the unknown-cabin telemetry request did not return within 2s
--- FAIL: TestWritesStillWorkAfterTelemetryUpdateForUnknownCabin (2.02s)
FAIL
FAIL	ejina-microgrid/internal/app	2.064s
testing: warning: no tests to run
PASS
ok  	ejina-microgrid/internal/config	0.040s [no tests to run]
testing: warning: no tests to run
PASS
ok  	ejina-microgrid/internal/domain	0.037s [no tests to run]
testing: warning: no tests to run
PASS
ok  	ejina-microgrid/internal/scheduler	0.045s [no tests to run]
testing: warning: no tests to run
PASS
ok  	ejina-microgrid/internal/store	0.045s [no tests to run]
testing: warning: no tests to run
PASS
ok  	ejina-microgrid/internal/transport/http	0.067s [no tests to run]
FAIL

```

stderr：

```text
(empty)
```

### linux/arm64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./... -run "^TestWritesStillWorkAfterTelemetryUpdateForUnknownCabin$" -count=1 -v
?   	ejina-microgrid/cmd/server	[no test files]
=== RUN   TestWritesStillWorkAfterTelemetryUpdateForUnknownCabin
    telemetry_unknown_cabin_test.go:51: write operations issued after the unknown-cabin telemetry request did not return within 2s
--- FAIL: TestWritesStillWorkAfterTelemetryUpdateForUnknownCabin (2.01s)
FAIL
FAIL	ejina-microgrid/internal/app	2.009s
testing: warning: no tests to run
PASS
ok  	ejina-microgrid/internal/config	0.002s [no tests to run]
testing: warning: no tests to run
PASS
ok  	ejina-microgrid/internal/domain	0.002s [no tests to run]
testing: warning: no tests to run
PASS
ok  	ejina-microgrid/internal/scheduler	0.002s [no tests to run]
testing: warning: no tests to run
PASS
ok  	ejina-microgrid/internal/store	0.002s [no tests to run]
testing: warning: no tests to run
PASS
ok  	ejina-microgrid/internal/transport/http	0.002s [no tests to run]
FAIL

```

stderr：

```text
(empty)
```

## 通过条件

准确定位 internal/app/service.go 的 (*Service).UpdateBatteryCabinTelemetry
说明该方法取得 Service.mu 后未以 defer 注册解锁、在 GetBatteryCabin 返回 domain.ErrEntityNotFound 的分支直接 return 导致锁被永久持有，进而说明所有先 s.mu.Lock() 的编排方法全部阻塞、而只经 store.Store 自身锁的只读方法不受影响、以及 scheduler 轮次停摆的完整因果链
结论有代码阅读或定向复现证据（如 go test ./... -run '^TestWritesStillWorkAfterTelemetryUpdateForUnknownCabin$' -count=1 -v）；目标仓库保持零改动
