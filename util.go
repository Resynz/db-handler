package db_handler

import (
	"reflect"
	"strings"
)

// camel string to snake string  XxYy = > xx_yy
func ToSnakeString(s string) string {
	data := make([]byte, 0, len(s)*2)
	j := false
	num := len(s)
	for i := 0; i < num; i++ {
		d := s[i]
		if i > 0 && d >= 'A' && d <= 'Z' && j {
			data = append(data, '_')
		}
		if d != '_' {
			j = true
		}
		data = append(data, d)
	}
	return strings.ToLower(string(data[:]))
}

// snake string to camel string xx_yy => XxYy
func ToCamelString(s string) string {
	data := make([]byte, 0, len(s))
	j := false
	k := false
	num := len(s) - 1
	for i := 0; i <= num; i++ {
		d := s[i]
		if k == false && d >= 'A' && d <= 'Z' {
			k = true
		}
		if d >= 'a' && d <= 'z' && (j || k == false) {
			d = d - 32
			j = false
			k = true
		}
		if k && d == '_' && num > i && s[i+1] >= 'a' && s[i+1] <= 'z' {
			j = true
			continue
		}
		data = append(data, d)
	}
	return string(data[:])
}

func ArrayJoin(arr []string, sep string) string {
	switch len(arr) {
	case 0:
		return ""
	case 1:
		return arr[0]
	case 2:
		// Special case for common small values.
		// Remove if golang.org/issue/6714 is fixed
		return arr[0] + sep + arr[1]
	case 3:
		// Special case for common small values.
		// Remove if golang.org/issue/6714 is fixed
		return arr[0] + sep + arr[1] + sep + arr[2]
	}
	n := len(sep) * (len(arr) - 1)
	for i := 0; i < len(arr); i++ {
		n += len(arr[i])
	}

	b := make([]byte, n)
	bp := copy(b, arr[0])
	for _, s := range arr[1:] {
		bp += copy(b[bp:], sep)
		bp += copy(b[bp:], s)
	}
	return string(b)
}

func StringArrayContains(arr []string, item string) bool {
	for _, v := range arr {
		if v == item {
			return true
		}
	}
	return false
}

func beanToBeans(bean interface{}) interface{} {
	t := reflect.TypeOf(bean)
	st := t.Elem()
	v := reflect.MakeSlice(reflect.SliceOf(st), 0, 0)
	return v.Addr().Interface()
}
