package bond

type BondType uint8

const (
	TypeUndef = iota
	TypeGov
	TypeMun
	TypeCorp
	TypeEuro
)

func Type(name string) BondType {
	switch name {
	case "euro":
		return TypeEuro
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
	case TypeEuro:
		return "euro"
	default:
		return "unknown"
	}
}
