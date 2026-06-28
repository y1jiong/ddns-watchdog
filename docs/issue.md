# `text/template` usage causes binary size inflation via Go linker conservative mode

**Repository:** kardianos/service  
**Version:** v1.2.4

## Summary

Using `text/template` for service file generation (plist on Darwin, unit files on Linux) causes
a significant binary size regression in any program that also links against libraries with many
concrete types implementing a common interface.

## Root Cause

`text/template/exec.go` calls `reflect.Value.MethodByName` to support method invocations in
templates (e.g. `{{.SomeMethod}}`). When this call is reachable in a binary, Go's linker switches
to a conservative mode: it must retain **all exported methods of all concrete types that implement
any used interface** in the entire binary, because any of those methods could potentially be
invoked via reflection at runtime.

This is a well-known Go linker behaviour — the problem is that `text/template` triggers it even
when no method invocations appear in the template string itself.

## Reproduction

Add `kardianos/service` to a project that also uses
`github.com/aliyun/alibaba-cloud-sdk-go` (or any library with a large number of concrete types
implementing a common interface):

```go
// Without service management: only 2 request types linked (173 symbols)
// With newService() reachable:  all 100+ request types linked (18 334 symbols)
svc, _ := service.New(&program{runFunc: run}, cfg)
svc.Install()  // triggers darwinLaunchdService.Install() → template.Execute()
```

| Scenario | alibaba SDK symbols | Binary size (stripped) |
|---|---|---|
| Without `kardianos/service` | 173 | 9 MB |
| With `kardianos/service` (v1.2.4) | 18 334 | 18 MB |
| After replacing SDK with custom client | 0 | 10 MB |

The size doubles purely because `darwinLaunchdService.Install()` calls
`template.Execute(f, to)`, making `reflect.Value.MethodByName` reachable.

**This happens even with an empty run function — the service file content is irrelevant.**

## Affected Files

- `service_darwin.go` — uses `text/template` for plist generation
- `service_systemd_linux.go`, `service_upstart_linux.go`, `service_openrc_linux.go`,
  `service_sysv_linux.go`, `service_procd_linux.go` — same pattern for unit files

## Suggested Fix

Replace `text/template` with `fmt.Fprintf` or `strings.NewReplacer`. The service file
templates are static strings with simple variable substitution — they do not require a
full template engine.

Example for Darwin:

```go
// before
func (s *darwinLaunchdService) template() *template.Template { ... }
s.template().Execute(f, to)

// after
fmt.Fprintf(f, launchdConfig,
    s.Config.Name,
    execPath,
    strings.Join(s.Config.Arguments, "</string>\n\t\t<string>"),
    // ...
)
```

This eliminates the `text/template` and `text/template/parse` dependencies entirely,
removing the `reflect.Value.MethodByName` call from the reachable symbol set and
restoring the linker's precise dead-code elimination.

## Environment

- Go 1.26 (darwin/arm64)
- kardianos/service v1.2.4
- Verified with `go tool nm` symbol counting and `go list -deps` package diffing
