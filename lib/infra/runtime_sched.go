package infra

import (
	_ "unsafe"
)

//go:linkname OsYield runtime.osyield
func OsYield()

//go:linkname ProcYield runtime.procyield
func ProcYield(cycles uint32)
