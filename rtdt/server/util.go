package rtdtserver

import "strconv"

// hbytes == "human bytes"
func hbytes(i uint64) string {
	switch {
	case i < 1e3:
		return strconv.FormatUint(i, 10) + "B"
	case i < 1e6:
		f := float64(i)
		return strconv.FormatFloat(f/1e3, 'f', 2, 64) + "KB"
	case i < 1e9:
		f := float64(i)
		return strconv.FormatFloat(f/1e6, 'f', 2, 64) + "MB"
	case i < 1e12:
		f := float64(i)
		return strconv.FormatFloat(f/1e9, 'f', 2, 64) + "GB"
	case i < 1e15:
		f := float64(i)
		return strconv.FormatFloat(f/1e12, 'f', 2, 64) + "TB"
	default:
		return strconv.FormatUint(i, 10)
	}
}

// hcount == "human count"
func hcount(i uint64) string {
	switch {
	case i < 1e3:
		return strconv.FormatUint(i, 10)
	case i < 1e6:
		f := float64(i)
		return strconv.FormatFloat(f/1e3, 'f', 2, 64) + "K"
	case i < 1e9:
		f := float64(i)
		return strconv.FormatFloat(f/1e6, 'f', 2, 64) + "M"
	case i < 1e12:
		f := float64(i)
		return strconv.FormatFloat(f/1e9, 'f', 2, 64) + "G"
	case i < 1e15:
		f := float64(i)
		return strconv.FormatFloat(f/1e12, 'f', 2, 64) + "T"
	default:
		return strconv.FormatUint(i, 10)
	}
}

// hrate == "human rate"
func hrate(f float64) string {
	switch {
	case f < 1e3:
		return strconv.FormatFloat(f, 'f', 2, 64)
	case f < 1e6:
		return strconv.FormatFloat(f/1e3, 'f', 2, 64) + "K"
	case f < 1e9:
		return strconv.FormatFloat(f/1e6, 'f', 2, 64) + "M"
	case f < 1e12:
		return strconv.FormatFloat(f/1e9, 'f', 2, 64) + "G"
	case f < 1e15:
		return strconv.FormatFloat(f/1e12, 'f', 2, 64) + "T"
	default:
		return strconv.FormatFloat(f, 'f', 2, 64)
	}
}
