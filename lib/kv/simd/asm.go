package main

import (
	. "github.com/mmcloughlin/avo/build"
	. "github.com/mmcloughlin/avo/operand"
)

func main() {
	ConstraintExpr("amd64")

	TEXT("MatchMetadata", NOSPLIT, "func(md *[16]int8, hash int8) uint16")
	Doc("MatchMetadata performs a 16-way linear probing of metadata by SSE instructions",
		"metadata must be an aligned pointer")

	Comment("Move the pointer of md to register AX")
	mem := Mem{Base: Load(Param("md"), GP64())}
	Comment("Move the hash value (int8) from mem to register CX then extend the size to int32")
	h := Load(Param("hash"), GP32())
	mask := GP32()

	x0, x1, x2 := XMM(), XMM(), XMM()

	Comment("Copy hash value from register CX to XMM register X0")
	Comment("XMM registers are used by SSE or AVX instructions")
	MOVD(h, x0)

	Comment("Clear the XMM register X1")
	PXOR(x1, x1)

	Comment("Packed Shuffle Bytes instruction, let hash value in register X0 xor with X1 by byte to generate mask to X0")
	PSHUFB(x1, x0)

	Comment("Load the metadata from memory to register X2")
	MOVOU(mem, x2)

	Comment("Packed Compare for Equal Byte instruction, compare X1 and X0 by byte then store into X0")
	Comment("The same byte are 0xFF. Otherwise, they are 0x00")
	PCMPEQB(x2, x0)

	Comment("Packed Move with Mask Signed Byte, Extract X0 hi part and convert into int16 then store into AX")
	Comment("The X0 lo part is unused usually")
	Comment("Now the each bit of AX mapping to the each hash of metadata array whether equals to target")
	PMOVMSKB(x0, mask)

	Comment("Copy the AX value to mem then return")
	Store(mask.As16(), ReturnIndex(0))
	RET()

	Generate()
}
