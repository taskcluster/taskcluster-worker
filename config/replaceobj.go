package config

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
			v, err := inject(key, replacement, v)
			if err != nil {
				return nil, err
			}
			val[i] = v
		}
	case map[string]interface{}:
		if _, ok := val["$"+key].(string); ok {
			return replacement(val)
		}
		for k, v := range val {
			v, err := inject(key, replacement, v)
			if err != nil {
				return nil, err
			}
			val[k] = v
		}
	}
	return val, nil
}
