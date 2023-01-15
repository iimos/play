
# Perf analysis notes

```sh
# Gather some cpu stat
go test -bench=. ./sort -benchmem -cpuprofile cpu.out
# This command creates two files:
# 1. sort.test - compilated binary
# 2. cpu.out - runtime statistics

# View function code anotated with time costs
go tool pprof -list InsertionSort sort.test cpu.out

# View assembly code of the function
go tool objdump -s sort.InsertionSort\\[go.shape.int32 -S sort.test

# Debug SSA optimisations
GOSSAFUNC=main go build && open ./ssa.html

# See what Go threads are doing
strace -o /tmp/trace -ff ./bin
```

