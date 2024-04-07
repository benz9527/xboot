package main

import (
	. "github.com/mmcloughlin/avo/build"
	. "github.com/mmcloughlin/avo/operand"
)

/*
	SIMD
	1. Load group (A), uint8 * 16
	---------------------------------------------     ------------
	| 01010111 | 11111111 | 00110110 | 11111111 | ... | 11111111 |
	---------------------------------------------     ------------

	2. Set comparable 0b110110 (B), uint8 * 16
	---------------------------------------------     ------------
	| 00110110 | 00110110 | 00110110 | 00110110 | ... | 00110110 |
	---------------------------------------------     ------------

	3. Compare A and B, uint8 * 16
	---------------------------------------------     ------------
	| 00000000 | 00000000 | 11111111 | 00000000 | ... | 00000000 |
	---------------------------------------------     ------------
	                       (success!)

	4. Mask values
	---------------------------------------------     ------------
	|    0     |    0     |    1     |    0     | ... |    0     |
	---------------------------------------------     ------------
	                         (true)
*/

func main() {
	ConstraintExpr("amd64")
	ConstraintExpr("!nosimd")

	TEXT("Fast16WayHashMatch", NOSPLIT, "func(md *[16]int8, hash int8) uint16")
	Doc("Fast16WayHashMatch performs a 16-way linear probing of short hash (h2, metadata) list by SSE instructions",
		"short hash list must be an aligned pointer")

	// The AX store the md pointer address.
	Comment("Move the pointer of md to register AX")
	mem := Mem{Base: Load(Param("md"), GP64())}

	// Assume that hash is 0b01100110.
	// After extended into 32 bits, it becomes 0x00 00 00 66
	Comment("Move the hash value (int8) from mem to register CX then extend the size to int32")
	h := Load(Param("hash"), GP32())
	mask := GP32()

	// XMM 128 bits register
	x0, x1 := XMM(), XMM()

	// After movd instruction, X0/128bits becomes 0x00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 66.
	// windows storage order is little endian order.
	// 0x1234 => low address: 0x32; high address: 0x12.
	// So the asm read index 0 byte from low address.
	// In X0, we load the index 0 byte will be 0x66.
	Comment("Copy hash value from register CX to XMM register X0")
	Comment("XMM registers are used by SSE or AVX instructions")
	MOVD(h, x0)

	// Parallel XOR
	Comment("Clear the XMM register X1")
	PXOR(x1, x1)

	// https://www.felixcloutier.com/x86/pshufb
	// PSHUFB xmm1, xmm2/m128
	// Shuffle bytes in xmm1 according to contents of xmm2/m128.
	// xmm2 is the shuffle control mask.
	// If the most significant bit (bit[7]) of each byte of the shuffle
	// control mask (xmm2) is set, then constant zero is written in the
	// result byte.
	//
	// for i = 0 to 15 {
	//   if (SRC2[(i * 8)+7] = 1) then
	//     DEST[(i*8)+7..(i*8)+0] := 0;
	//     else
	//     index[3..0] := SRC2[(i*8)+3 .. (i*8)+0];
	//     DEST[(i*8)+7..(i*8)+0] := SRC1[(index*8+7)..(index*8+0)];
	//   endif
	// }
	// DEST[MAXVL-1:128] := 0
	//
	// But in go plan9 asm, the xmm2 is the destination register.
	// x1 all bits are zero, like 0x00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00.
	// After the pshufb, the x0 will be 0x66 66 66 66 66 66 66 66 66 66 66 66 66 66 66 66.
	Comment("Packed Shuffle Bytes instruction, let hash value in register X0 xor with X1 by byte to generate mask to X0")
	PSHUFB(x1, x0)

	// Plan9 MOVOU (move vector of oct-words unaligned)
	// An SSE instruction to load 128-bit data from memory to register.
	// The mem is AX actually.
	Comment("Load the metadata from memory to register X1")
	Comment("(AX) means de-reference of address value in AX")
	MOVOU(mem, x1)

	Comment("Packed Compare for Equal Byte instruction, compare X1 and X0 by byte then store into X0")
	Comment("The same byte are 0xFF. Otherwise, they are 0x00")
	PCMPEQB(x1, x0)

	Comment("Packed Move with Mask Signed Byte, Extract X0 hi part and convert into int16 then store into AX")
	Comment("The X0 lo part is unused usually")
	Comment("Now the each bit of AX mapping to the each hash of metadata array whether equals to target")
	PMOVMSKB(x0, mask)

	Comment("Copy the AX value to mem then return")
	Store(mask.As16(), ReturnIndex(0))
	RET()

	Generate()
}
