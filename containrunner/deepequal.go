package containrunner

// Copy from the reflect.DeepEqual with a bit more naive support
// Main difference is that this supports a `DeepEqual:"skip"` tag
// in structs

import (
	"fmt"
	"reflect"
)

// During myDeepValueEqual, must keep track of checks that are
// in progress.  The comparison algorithm assumes that all
// checks in progress are true when it reencounters them.
// Visited comparisons are stored in a map indexed by visit.
type visit struct {
	a1  uintptr
	a2  uintptr
	typ reflect.Type
}

// Tests for deep equality using reflected types. The map argument tracks
// comparisons that have already been seen, which allows short circuiting on
// recursive types.
func deepValueEqual(v1, v2 reflect.Value, visited map[visit]bool, depth int) bool {
	if !v1.IsValid() || !v2.IsValid() {
		//fmt.Println("deepValueEqual: IsValid failure")
		return v1.IsValid() == v2.IsValid()
	}
	if v1.Type() != v2.Type() {
		//fmt.Println("deepValueEqual: Type failure")
		return false
	}

	// if depth > 10 { panic("deepValueEqual") }	// for debugging
	hard := func(k reflect.Kind) bool {
		switch k {
		case reflect.Array, reflect.Map, reflect.Slice, reflect.Struct:
			return true
		}
		//fmt.Printf("deepValueEqual: hard failure: %+v\n", k)
		return false
	}

	if v1.CanAddr() && v2.CanAddr() && hard(v1.Kind()) {
		addr1 := v1.UnsafeAddr()
		addr2 := v2.UnsafeAddr()
		if addr1 > addr2 {
			// Canonicalize order to reduce number of entries in visited.
			addr1, addr2 = addr2, addr1
		}

		// Short circuit if references are identical ...
		if addr1 == addr2 {
			return true
		}

		// ... or already seen
		typ := v1.Type()
		v := visit{addr1, addr2, typ}
		if visited[v] {
			return true
		}

		// Remember for later.
		visited[v] = true
	}

	switch v1.Kind() {
	case reflect.Array:
		for i := 0; i < v1.Len(); i++ {
			if !deepValueEqual(v1.Index(i), v2.Index(i), visited, depth+1) {
				//	fmt.Println("deepValueEqual: Array failure")
				return false
			}
		}
		return true
	case reflect.Slice:
		if v1.IsNil() != v2.IsNil() {
			//fmt.Println("deepValueEqual: Slice IsNil failure")
			return false
		}
		if v1.Len() != v2.Len() {
			//fmt.Println("deepValueEqual: Slice Len failure")
			return false
		}
		if v1.Pointer() == v2.Pointer() {
			return true
		}
		for i := 0; i < v1.Len(); i++ {
			if !deepValueEqual(v1.Index(i), v2.Index(i), visited, depth+1) {
				//fmt.Println("deepValueEqual: Slice deep failure")

				return false
			}
		}
		return true
	case reflect.Interface:
		if v1.IsNil() || v2.IsNil() {
			if v1.IsNil() != v2.IsNil() {
				//	fmt.Println("deepValueEqual: Interface IsNil failure")
			}

			return v1.IsNil() == v2.IsNil()
		}
		return deepValueEqual(v1.Elem(), v2.Elem(), visited, depth+1)
	case reflect.Ptr:
		return deepValueEqual(v1.Elem(), v2.Elem(), visited, depth+1)

	case reflect.Struct:
		for i, n := 0, v1.NumField(); i < n; i++ {
			if v1.Type().Field(i).Tag.Get("DeepEqual") == "skip" {
				//fmt.Printf("Skipping DeepEqual field %s\n", v1.Type().Name())
				continue
			}
			if !deepValueEqual(v1.Field(i), v2.Field(i), visited, depth+1) {
				//fmt.Printf("deepValueEqual: Struct deepValueEqual failure: %+v vs %+v at depth %d\n", v1.Field(i), v2.Field(i), depth+1)

				return false
			}
		}
		return true
	case reflect.Map:
		if v1.IsNil() != v2.IsNil() {
			//fmt.Println("deepValueEqual: Map IsNil failure")

			return false
		}
		if v1.Len() != v2.Len() {
			//fmt.Printf("deepValueEqual: Map %s Len failure: %d vs %d. Keys1: %+v, Keys2: %+v\n", v1.Type().Name(), v1.Len(), v2.Len(), v1.MapKeys(), v2.MapKeys())

			return false
		}
		if v1.Pointer() == v2.Pointer() {
			return true
		}
		for _, k := range v1.MapKeys() {
			if !deepValueEqual(v1.MapIndex(k), v2.MapIndex(k), visited, depth+1) {
				//fmt.Println("deepValueEqual: Map keys myDeepValueEqual failure")

				return false
			}
		}
		return true
	case reflect.Func:
		if v1.IsNil() && v2.IsNil() {
			return true
		}
		// Can't do better than this:
		//fmt.Println("deepValueEqual: Func failure")

		return false
	case reflect.String:
		if v1.String() != v2.String() {
			//fmt.Println("deepValueEqual: String failure")
			return false
		}

		return true

	case reflect.Float32, reflect.Float64:
		if v1.Float() != v2.Float() {
			//fmt.Println("deepValueEqual: Float failure")
			return false
		}

		return true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if v1.Uint() != v2.Uint() {
			//fmt.Println("deepValueEqual: Uint failure")
			return false
		}

		return true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v1.Int() != v2.Int() {
			//fmt.Println("deepValueEqual: Uint failure")
			return false
		}

		return true
	case reflect.Bool:
		if v1.Bool() != v2.Bool() {
			//fmt.Println("deepValueEqual: Uint failure")
			return false
		}

		return true
	default:
		// Normal equality suffices

		panic(fmt.Sprintf("Unsupported type: %+v", v1.Kind()))
		/*
			if ret == false {
				fmt.Printf("deepValueEqual: valueInterface failure: '%+v' vs '%+v' Kind: %+v vs %+v\n", v1, v2, v1.Kind(), v2.Kind())
				return false
			} else {
				return true
			}*/
		//return valueInterface(v1, false) == valueInterface(v2, false)
	}
	return true

}

// DeepEqual tests for deep equality. It uses normal == equality where
// possible but will scan elements of arrays, slices, maps, and fields of
// structs. In maps, keys are compared with == but elements use deep
// equality. DeepEqual correctly handles recursive types. Functions are equal
// only if they are both nil.
// An empty slice is not equal to a nil slice.
func DeepEqual(a1, a2 interface{}) bool {
	if a1 == nil || a2 == nil {
		return a1 == a2
	}
	v1 := reflect.ValueOf(a1)
	v2 := reflect.ValueOf(a2)
	if v1.Type() != v2.Type() {
		//fmt.Println("DeepEqual: Type failure")

		return false
	}
	return deepValueEqual(v1, v2, make(map[visit]bool), 0)
}
