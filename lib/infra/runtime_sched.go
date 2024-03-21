package infra

import (
	_ "unsafe"
)

//go:linkname osYield runtime.osyield
func osYield()

func OsYield() {
	osYield()
}

//go:linkname procYield runtime.procyield
func procYield(cycles uint32)

func ProcYield(cycles uint32) {
	procYield(cycles)
}
