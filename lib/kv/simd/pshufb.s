//go:build amd64 && !nosimd

#include "textflag.h"

// func pshufb(md *[16]int8, hash int8)
// Requires: SSE2, SSSE3
TEXT ·pshufb(SB), NOSPLIT, $0-18
	MOVQ md+0(FP), AX
	MOVBLSX hash+8(FP), CX
	MOVD CX, X0
	PXOR X1, X1
	// x1 all bits are zero, like 0x00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00.
	// Assume that hash is 0x66
	// After the pshufb, the x0 will be 0x66 66 66 66 66 66 66 66 66 66 66 66 66 66 66 66.
    //
    // for i = 0 to 15 {
    //     if (SRC2[(i * 8)+7] = 1) then
    //         DEST[(i*8)+7..(i*8)+0] := 0;
    //         else
    //         index[3..0] := SRC2[(i*8)+3 .. (i*8)+0]; // index is 4 bits value.
    //         DEST[(i*8)+7..(i*8)+0] := SRC1[(index*8+7)..(index*8+0)];
    //     endif
    // }
    // DEST[MAXVL-1:128] := 0
    //
	PSHUFB X1, X0
    MOVUPS X0, (AX)
    RET 
