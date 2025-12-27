package configs

// SourceDataType represents the type of data source
type SourceDataType int

const (
	Local SourceDataType = iota
	GDrive
)

// String returns the string representation of SourceDataType
func (s SourceDataType) String() string {
	switch s {
	case Local:
		return "Local"
	case GDrive:
		return "GDrive"
	default:
		return "Unknown"
	}
}

func ParseSourceDataType(s string) SourceDataType {
	switch s {
	case "Local":
		return Local
	case "GDrive":
		return GDrive
	default:
		return Local
	}
}

func GetHashId(input string) uint64 {
	var hash uint64 = 5381
	for i := 0; i < len(input); i++ {
		hash = ((hash << 5) + hash) + uint64(input[i])
	}
	return hash
}
