package utils

import (
	"bytes"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	ccyaml "github.com/goccy/go-yaml"
	"github.com/pkg/errors"
)

func ToYaml(it any) ([]byte, error) {
	var (
		bts []byte
		err error
	)

	buffer := bytes.NewBuffer(bts)
	enc := jsontext.NewEncoder(buffer, 
		json.FormatNilMapAsNull(true), 
		json.FormatNilSliceAsNull(true),
		json.StringifyNumbers(true),
	)

	if err = json.MarshalEncode(enc, it); err != nil {
		return bts, err
	}

	bts = buffer.Bytes()

	if bts, err = ccyaml.JSONToYAML(bts); err != nil {
		return bts, err
	}

	return bts, nil
}

func FromYaml(bts []byte) (map[string]any, error) {
	var (
		out map[string]any
		err error
	)

	if err = ccyaml.Unmarshal(bts, &out); err != nil {
		return out, errors.Wrapf(err, "failed to unmarshal template %s", string(bts))
	}

	return out, nil
}
