package util

import "k8s.io/apimachinery/pkg/util/intstr"

func IntstrPointer(i int) *intstr.IntOrString {
	is := intstr.FromInt(i)
	return &is
}

func Int64Pointer(i int64) *int64 {
	return &i
}

func Int32Pointer(i int32) *int32 {
	return &i
}

func BoolPointer(b bool) *bool {
	return &b
}

func MergeLabels(l map[string]string, l2 map[string]string) map[string]string {
	if l == nil {
		l = make(map[string]string)
	}
	for lKey, lValue := range l2 {
		l[lKey] = lValue
	}
	return l
}
