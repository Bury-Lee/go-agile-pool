// test/ is a separate module so the harness and its heavy deps
// (gopsutil) never pollute the root library module. The module path keeps
// the github.com/Yiming1997/agilePool/v2 prefix so internal packages
// (internal/hook) stay importable, and the replace below forces every
// agilePool import to resolve to the root directory on disk: building the
// harness never downloads or uses the network copy of the library.
module github.com/Yiming1997/agilePool/v2/test

go 1.26.4

require (
	github.com/Yiming1997/agilePool/v2 v2.1.0
	github.com/shirou/gopsutil/v3 v3.24.5
)

replace github.com/Yiming1997/agilePool/v2 => ../

require (
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/sys v0.20.0 // indirect
)
