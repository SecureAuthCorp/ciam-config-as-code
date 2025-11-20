package utils

import (
	"fmt"
	"github.com/mitchellh/mapstructure"
	"net/url"
	"reflect"
	"strings"
	"time"
)

func Decoder(result any) (*mapstructure.Decoder, error) {
	config := &mapstructure.DecoderConfig{
		Result: result,
	}

	ConfigureDecoder(config)

	return mapstructure.NewDecoder(config)
}

func ConfigureDecoder(config *mapstructure.DecoderConfig) {
	config.TagName = "json"
	config.WeaklyTypedInput = false
	config.DecodeHook = mapstructure.ComposeDecodeHookFunc(urlDecoder(), timeDecoder(), stringToSlice())
}

func urlDecoder() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		if t != reflect.TypeOf(url.URL{}) {
			return data, nil
		}

		var (
			str string
			err error
			ok  bool
			u   *url.URL
		)

		if str, ok = data.(string); !ok {
			return nil, fmt.Errorf("cannot map %v", reflect.TypeOf(data))
		}

		if u, err = url.Parse(str); err != nil {
			return nil, err
		}

		return u, nil
	}
}

func timeDecoder() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{},
	) (interface{}, error) {
		if t != reflect.TypeOf(time.Time{}) {
			return data, nil
		}

		switch f.Kind() {
		case reflect.String:
			return time.Parse(time.RFC3339, data.(string))
		case reflect.Float64:
			return time.Unix(0, int64(data.(float64))*int64(time.Millisecond)), nil
		case reflect.Int64:
			return time.Unix(0, data.(int64)*int64(time.Millisecond)), nil
		default:
			return data, nil
		}
	}
}

func stringToSlice() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Kind,
		t reflect.Kind,
		data interface{}) (interface{}, error) {
		if f != reflect.String || t != reflect.Slice {
			return data, nil
		}

		raw := data.(string)
		if raw == "" {
			return []string{}, nil
		}

		return strings.Split(raw, ","), nil
	}
}
