package csgo

import (
	"strings"
	"unicode"
)

// SubStringLast 截取掉字符串之后，取得后面的字符串
func SubStringLast(str string, substr string) string {

	index := strings.Index(str, substr)

	if index < 0 {
		return ""
	}
	return str[index+len(substr):]
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}
