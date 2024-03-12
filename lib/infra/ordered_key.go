package infra

type Signed interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

// Unsigned is a constraint that permits any unsigned integer type.
// If future releases of Go add new predeclared unsigned integer types,
// this constraint will be modified to include them.
type Unsigned interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Integer is a constraint that permits any integer type.
// If future releases of Go add new predeclared integer types,
// this constraint will be modified to include them.
type Integer interface {
	Signed | Unsigned
}

// Float is a constraint that permits any floating-point type.
// If future releases of Go add new predeclared floating-point types,
// this constraint will be modified to include them.
type Float interface {
	~float32 | ~float64
}

// Complex is a constraint that permits any complex numeric type.
// If future releases of Go add new predeclared complex numeric types,
// this constraint will be modified to include them.
// We have to calc the complex square root.
// i.e. Amplitude (modulus) comparison in the complex plane.
type Complex interface {
	~complex64 | ~complex128
}

// OrderedKey
// byte => ~uint8
type OrderedKey interface {
	Integer | Float | ~string
}

// OrderedKeyComparator
// Assume i is the new key.
//  1. i == j (i-j == 0, return 0)
//  2. i > j (i-j > 0, return 1), turn to right part.
//  3. i < j (i-j < 0, return -1), turn to left part.
type OrderedKeyComparator[K OrderedKey] func(i, j K) int64
