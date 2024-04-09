package id

// Gen generates the number uuid.
type Gen func() uint64

type UUIDGen interface {
	Number() uint64
	Str() string
}

var (
	_ UUIDGen = (*uuidDelegator)(nil)
)

type uuidDelegator struct {
	number Gen
	str    func() string
}

func (id *uuidDelegator) Number() uint64 { return id.number() }
func (id *uuidDelegator) Str() string    { return id.str() }

type NanoIDGen func() string
