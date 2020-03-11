// Code generated by go generate; DO NOT EDIT.
// This file was generated by robots.

package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
)

var dereferencableKindsConfig = map[reflect.Kind]struct{}{
	reflect.Array: {}, reflect.Chan: {}, reflect.Map: {}, reflect.Ptr: {}, reflect.Slice: {},
}

// Checks if t is a kind that can be dereferenced to get its underlying type.
func canGetElementConfig(t reflect.Kind) bool {
	_, exists := dereferencableKindsConfig[t]
	return exists
}

// This decoder hook tests types for json unmarshaling capability. If implemented, it uses json unmarshal to build the
// object. Otherwise, it'll just pass on the original data.
func jsonUnmarshalerHookConfig(_, to reflect.Type, data interface{}) (interface{}, error) {
	unmarshalerType := reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
	if to.Implements(unmarshalerType) || reflect.PtrTo(to).Implements(unmarshalerType) ||
		(canGetElementConfig(to.Kind()) && to.Elem().Implements(unmarshalerType)) {

		raw, err := json.Marshal(data)
		if err != nil {
			fmt.Printf("Failed to marshal Data: %v. Error: %v. Skipping jsonUnmarshalHook", data, err)
			return data, nil
		}

		res := reflect.New(to).Interface()
		err = json.Unmarshal(raw, &res)
		if err != nil {
			fmt.Printf("Failed to umarshal Data: %v. Error: %v. Skipping jsonUnmarshalHook", data, err)
			return data, nil
		}

		return res, nil
	}

	return data, nil
}

func decode_Config(input, result interface{}) error {
	config := &mapstructure.DecoderConfig{
		TagName:          "json",
		WeaklyTypedInput: true,
		Result:           result,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
			jsonUnmarshalerHookConfig,
		),
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}

	return decoder.Decode(input)
}

func join_Config(arr interface{}, sep string) string {
	listValue := reflect.ValueOf(arr)
	strs := make([]string, 0, listValue.Len())
	for i := 0; i < listValue.Len(); i++ {
		strs = append(strs, fmt.Sprintf("%v", listValue.Index(i)))
	}

	return strings.Join(strs, sep)
}

func testDecodeJson_Config(t *testing.T, val, result interface{}) {
	assert.NoError(t, decode_Config(val, result))
}

func testDecodeSlice_Config(t *testing.T, vStringSlice, result interface{}) {
	assert.NoError(t, decode_Config(vStringSlice, result))
}

func TestConfig_GetPFlagSet(t *testing.T) {
	val := Config{}
	cmdFlags := val.GetPFlagSet("")
	assert.True(t, cmdFlags.HasFlags())
}

func TestConfig_SetFlags(t *testing.T) {
	actual := Config{}
	cmdFlags := actual.GetPFlagSet("")
	assert.True(t, cmdFlags.HasFlags())

	t.Run("Test_endpoint", func(t *testing.T) {
		t.Run("DefaultValue", func(t *testing.T) {
			// Test that default value is set properly
			if vString, err := cmdFlags.GetString("endpoint"); err == nil {
				assert.Equal(t, string(defaultConfig.Environment.String()), vString)
			} else {
				assert.FailNow(t, err.Error())
			}
		})

		t.Run("Override", func(t *testing.T) {
			testValue := defaultConfig.Environment.String()

			cmdFlags.Set("endpoint", testValue)
			if vString, err := cmdFlags.GetString("endpoint"); err == nil {
				testDecodeJson_Config(t, fmt.Sprintf("%v", vString), &actual.Environment)

			} else {
				assert.FailNow(t, err.Error())
			}
		})
	})
	t.Run("Test_defaultRoutingGroup", func(t *testing.T) {
		t.Run("DefaultValue", func(t *testing.T) {
			// Test that default value is set properly
			if vString, err := cmdFlags.GetString("defaultRoutingGroup"); err == nil {
				assert.Equal(t, string(defaultConfig.DefaultRoutingGroup), vString)
			} else {
				assert.FailNow(t, err.Error())
			}
		})

		t.Run("Override", func(t *testing.T) {
			testValue := "1"

			cmdFlags.Set("defaultRoutingGroup", testValue)
			if vString, err := cmdFlags.GetString("defaultRoutingGroup"); err == nil {
				testDecodeJson_Config(t, fmt.Sprintf("%v", vString), &actual.DefaultRoutingGroup)

			} else {
				assert.FailNow(t, err.Error())
			}
		})
	})
	t.Run("Test_workers", func(t *testing.T) {
		t.Run("DefaultValue", func(t *testing.T) {
			// Test that default value is set properly
			if vInt, err := cmdFlags.GetInt("workers"); err == nil {
				assert.Equal(t, int(defaultConfig.Workers), vInt)
			} else {
				assert.FailNow(t, err.Error())
			}
		})

		t.Run("Override", func(t *testing.T) {
			testValue := "1"

			cmdFlags.Set("workers", testValue)
			if vInt, err := cmdFlags.GetInt("workers"); err == nil {
				testDecodeJson_Config(t, fmt.Sprintf("%v", vInt), &actual.Workers)

			} else {
				assert.FailNow(t, err.Error())
			}
		})
	})
	t.Run("Test_lruCacheSize", func(t *testing.T) {
		t.Run("DefaultValue", func(t *testing.T) {
			// Test that default value is set properly
			if vInt, err := cmdFlags.GetInt("lruCacheSize"); err == nil {
				assert.Equal(t, int(defaultConfig.LruCacheSize), vInt)
			} else {
				assert.FailNow(t, err.Error())
			}
		})

		t.Run("Override", func(t *testing.T) {
			testValue := "1"

			cmdFlags.Set("lruCacheSize", testValue)
			if vInt, err := cmdFlags.GetInt("lruCacheSize"); err == nil {
				testDecodeJson_Config(t, fmt.Sprintf("%v", vInt), &actual.LruCacheSize)

			} else {
				assert.FailNow(t, err.Error())
			}
		})
	})
	t.Run("Test_awsS3ShardFormatter", func(t *testing.T) {
		t.Run("DefaultValue", func(t *testing.T) {
			// Test that default value is set properly
			if vString, err := cmdFlags.GetString("awsS3ShardFormatter"); err == nil {
				assert.Equal(t, string(defaultConfig.AwsS3ShardFormatter), vString)
			} else {
				assert.FailNow(t, err.Error())
			}
		})

		t.Run("Override", func(t *testing.T) {
			testValue := "1"

			cmdFlags.Set("awsS3ShardFormatter", testValue)
			if vString, err := cmdFlags.GetString("awsS3ShardFormatter"); err == nil {
				testDecodeJson_Config(t, fmt.Sprintf("%v", vString), &actual.AwsS3ShardFormatter)

			} else {
				assert.FailNow(t, err.Error())
			}
		})
	})
	t.Run("Test_awsS3ShardStringLength", func(t *testing.T) {
		t.Run("DefaultValue", func(t *testing.T) {
			// Test that default value is set properly
			if vInt, err := cmdFlags.GetInt("awsS3ShardStringLength"); err == nil {
				assert.Equal(t, int(defaultConfig.AwsS3ShardCount), vInt)
			} else {
				assert.FailNow(t, err.Error())
			}
		})

		t.Run("Override", func(t *testing.T) {
			testValue := "1"

			cmdFlags.Set("awsS3ShardStringLength", testValue)
			if vInt, err := cmdFlags.GetInt("awsS3ShardStringLength"); err == nil {
				testDecodeJson_Config(t, fmt.Sprintf("%v", vInt), &actual.AwsS3ShardCount)

			} else {
				assert.FailNow(t, err.Error())
			}
		})
	})
}
