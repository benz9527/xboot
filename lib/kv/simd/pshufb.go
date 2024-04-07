//go:build amd64 && !nosimd

package main

func pshufb(md *[16]int8, hash int8)
