//-----------------------------------------------------------------------------

package jprop

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

//-----------------------------------------------------------------------------

// Unmarshal loads data from a .properties format into a struct.
func Unmarshal(data []byte, v interface{}) error {
	_, vErr := unmarshal(data, v)
	if vErr != nil {
		return vErr
	}

	return nil
}

//--------------------------------------

// UnmarshalStrict requires all fields of data to exist in v.
func UnmarshalStrict(data []byte, v interface{}) error {
	vLineNotSet, vErr := unmarshal(data, v)
	if vErr != nil {
		return vErr
	}
	if vLineNotSet != "" {
		return fmt.Errorf("property failed to be assigned: %q", vLineNotSet)
	}

	return nil
}

//--------------------------------------

// unmarshal loads data from a .properties format into a struct.  If any of the
// property lines can't be set, that line is returned.
func unmarshal(data []byte, v interface{}) (string, error) {
	vLineNotSet := ""
	vLines := strings.Split(string(data), "\n")
	val := reflect.ValueOf(v).Elem()

	for _, vLine := range vLines {
		vLine = strings.TrimSpace(vLine)
		if vLine == "" || strings.HasPrefix(vLine, "#") {
			continue
		}

		vParts := strings.SplitN(vLine, "=", 2)
		if len(vParts) != 2 {
			continue
		}

		key, value := strings.TrimSpace(vParts[0]), strings.TrimSpace(vParts[1])
		vDidSet, vErr := setValueFromString(val, key, value)
		if vErr != nil {
			return "", vErr
		}

		if !vDidSet && (vLineNotSet == "") {
			vLineNotSet = vLine
		}
	}

	return vLineNotSet, nil
}

//--------------------------------------

// setValueFromString manages the deserialization process.
func setValueFromString(v reflect.Value, key, value string) (bool, error) {
	val := reflect.Indirect(v)

	switch val.Kind() {
	case reflect.Struct:
		return setStructValue(val, key, value)
	case reflect.Map:
		return setMapValue(val, key, value)
	default:
		return setBasicValue(val, value)
	}
}

//--------------------------------------

func setStructValue(val reflect.Value, key, value string) (bool, error) {
	vDidSet := false
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)
		tag := field.Tag.Get("jprop")
		fieldKey := parseTagOptions(tag).name
		if fieldKey == "" {
			fieldKey = field.Name
		}

		vDidSetOne := false
		if strings.HasPrefix(key, fieldKey) {
			subKey := strings.TrimPrefix(key, fieldKey)
			if subKey == "" || subKey[0] == '.' {
				subKey = strings.TrimPrefix(subKey, ".")
				var vErr error
				vDidSetOne, vErr = handleFieldType(fieldValue, subKey, value)
				if vErr != nil {
					return false, vErr
				}
			}
		}
		if vDidSetOne {
			vDidSet = true
		}
	}
	return vDidSet, nil
}

//--------------------------------------

func handleFieldType(fieldValue reflect.Value, subKey, value string) (bool, error) {
	switch fieldValue.Kind() {
	case reflect.Struct:
		return setValueFromString(fieldValue, subKey, value)
	case reflect.Slice:
		return setSliceValue(fieldValue, value)
	case reflect.Map:
		return setMapValue(fieldValue, subKey, value)
	default:
		return setValueFromString(fieldValue, subKey, value)
	}
}

//--------------------------------------

func setSliceValue(fieldValue reflect.Value, value string) (bool, error) {
	vDidSetAll := true
	items := strings.Split(value, ",")
	slice := reflect.MakeSlice(fieldValue.Type(), len(items), len(items))
	for idx, item := range items {
		vDidSet, vErr := setBasicValue(slice.Index(idx), strings.TrimSpace(item))
		if vErr != nil {
			return false, vErr
		}
		if !vDidSet {
			vDidSetAll = false
		}
	}
	fieldValue.Set(slice)
	return vDidSetAll, nil
}

//--------------------------------------

func setMapValue(val reflect.Value, key, value string) (bool, error) {
	vDidSetAll := true
	mapKey := extractMapKey(key)

	// Initialize the map if it is nil
	if val.IsNil() {
		val.Set(reflect.MakeMap(val.Type()))
	}

	// Handle empty keys correctly
	if mapKey == "" {
		// Set the empty key value
		mapValue := reflect.New(val.Type().Elem()).Elem()
		vDidSet, vErr := setBasicValue(mapValue, value)
		if vErr != nil {
			return false, vErr
		}
		if !vDidSet {
			vDidSetAll = false
		}
		val.SetMapIndex(reflect.ValueOf(mapKey), mapValue)
		return vDidSetAll, nil
	}

	// Retrieve the existing value for the given map key
	mapValue := val.MapIndex(reflect.ValueOf(mapKey))

	// If it doesn't exist, create a new value
	if !mapValue.IsValid() {
		mapValue = reflect.New(val.Type().Elem()).Elem()
	}

	// Set the value in the map
	vDidSet, vErr := setBasicValue(mapValue, value)
	if vErr != nil {
		return false, vErr
	}
	if !vDidSet {
		vDidSetAll = false
	}
	val.SetMapIndex(reflect.ValueOf(mapKey), mapValue)

	return vDidSetAll, nil
}

//--------------------------------------

// extractMapKey extracts the key of the map from the given string.
func extractMapKey(key string) string {
	parts := strings.SplitN(key, ".", 2)
	return parts[0] // Return only the key part
}

//--------------------------------------

// setBasicValue sets the value of a basic field type (string, bool, int, float, etc.).
func setBasicValue(v reflect.Value, value string) (bool, error) {
	vDidSet := false
	if !v.IsValid() || !v.CanSet() {
		return false, fmt.Errorf("invalid or unassignable value provided: %s", v)
	}

	switch v.Kind() {
	case reflect.String:
		v.SetString(value)
		vDidSet = true
	case reflect.Bool:
		boolVal, vErr := strconv.ParseBool(value)
		if vErr != nil {
			return false, vErr
		}
		v.SetBool(boolVal)
		vDidSet = true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, vErr := strconv.ParseInt(value, 10, 64)
		if vErr != nil {
			return false, vErr
		}
		v.SetInt(intVal)
		vDidSet = true
	case reflect.Float32, reflect.Float64:
		floatVal, vErr := strconv.ParseFloat(value, 64)
		if vErr != nil {
			return false, vErr
		}
		v.SetFloat(floatVal)
		vDidSet = true
	default:
		return false, fmt.Errorf("unsupported type: %s", v.Type())
	}
	return vDidSet, nil
}

//-----------------------------------------------------------------------------
