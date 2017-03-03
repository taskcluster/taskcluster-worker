package config

import "fmt"

// ReplaceObjects will traverse through the config object and replace all
// objects that has a the given '$' + key property with the value returned
// from replacement(obj).
//
// This is useful when implementing TransformationProviders.
func ReplaceObjects(
	config map[string]interface{},
	key string,
	replacement func(obj map[string]interface{}) (interface{}, error),
) error {
	_, err := inject(key, replacement, config)
	return err
}

func inject(
	key string,
	replacement func(obj map[string]interface{}) (interface{}, error),
	val interface{},
) (interface{}, error) {
	switch val := val.(type) {
	case []interface{}:
		for i, v := range val {
			v2, err := inject(key, replacement, v)
			if err != nil {
				return nil, err
			}
			val[i] = v2
		}
	case map[string]interface{}:
		if _, ok := val["$"+key].(interface{}); ok {
			return replacement(val)
		}
		for k, v := range val {
			v2, err := inject(key, replacement, v)
			if err != nil {
				return nil, err
			}
			val[k] = v2
		}
	}
	return val, nil
}

func jsonCompatTypes(val interface{}) error {
	switch val := val.(type) {
	case map[string]interface{}:
		for _, v := range val {
			if err := jsonCompatTypes(v); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, v := range val {
			if err := jsonCompatTypes(v); err != nil {
				return err
			}
		}
	case string, float64, bool, nil:
	default:
		return fmt.Errorf(
			"Illegal type: %T, config structure can only contain simple JSON "+
				"compatible types", val)
	}
	return nil
}
