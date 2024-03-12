package id

// Gen generates the number uuid.
type Gen func() uint64

type Generator interface {
	NumberUUID() uint64
	StrUUID() string
}

type defaultID struct {
	number func() uint64
	str    func() string
}

func (id *defaultID) NumberUUID() uint64 { return id.number() }
func (id *defaultID) StrUUID() string    { return id.str() }
