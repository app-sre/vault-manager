package utils

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Yaml unmarshal limitation causes nested options objects to be decode as strings with json format
// ex: `{"foo": "bar"}`
// UnmarshalJsonObj performs unmarshal of jsons strings
func UnmarshalJsonObj(key string, obj interface{}) (map[string]interface{}, error) {
	if obj == nil {
		return nil, nil
	}
	strObj, ok := obj.(string)
	if !ok {
		return nil, errors.New(fmt.Sprintf("Type conversion failed for %s", key))
	}
	var unmarshalled map[string]interface{}
	err := json.Unmarshal([]byte(strObj), &unmarshalled)
	if err != nil {
		return nil, err
	}
	return unmarshalled, nil
}
