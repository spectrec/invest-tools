package bond

type BondType uint8

const (
	TypeUndef = iota
	TypeGov
	TypeMun
	TypeCorp
)

func Type(name string) BondType {
	switch name {
	case "corp":
		return TypeCorp
	case "mun":
		return TypeMun
	case "gov":
		return TypeGov
	default:
		return TypeUndef
	}
}

func (t BondType) String() string {
	switch t {
	case TypeCorp:
		return "corp"
	case TypeMun:
		return "mun"
	case TypeGov:
		return "gov"
	default:
		return "unknown"
	}
}
