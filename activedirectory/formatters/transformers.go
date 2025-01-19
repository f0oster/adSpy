package formatters

import (
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

type Transformer interface {
	Transform(values []string) (interface{}, error)
}

type StringTransformer struct{}

func (t StringTransformer) Transform(values []string) (interface{}, error) {
	sanitizedValues := make([]string, len(values))
	for i, value := range values {
		sanitizedValues[i] = strings.ToValidUTF8(value, "")
	}
	return sanitizedValues, nil
}

type ByteTransformer struct{}

func (t ByteTransformer) Transform(values []string) (interface{}, error) {
	if len(values) > 0 {
		binaryData := values[0]
		return base64.StdEncoding.EncodeToString([]byte(binaryData)), nil
	}
	return nil, nil
}

type IntTransformer struct{}

func (t IntTransformer) Transform(values []string) (interface{}, error) {
	intValues := make([]int, len(values))
	for i, value := range values {
		parsedValue, err := strconv.Atoi(value)
		if err != nil {
			return nil, err
		}
		intValues[i] = parsedValue
	}
	return intValues, nil
}

type Int64Transformer struct{}

func (t Int64Transformer) Transform(values []string) (interface{}, error) {
	int64Values := make([]int64, len(values))
	for i, value := range values {
		parsedValue, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, err
		}
		int64Values[i] = parsedValue
	}
	return int64Values, nil
}

type BoolTransformer struct{}

func (t BoolTransformer) Transform(values []string) (interface{}, error) {
	boolValues := make([]bool, len(values))
	for i, value := range values {
		parsedValue, err := strconv.ParseBool(value)
		if err != nil {
			return nil, err
		}
		boolValues[i] = parsedValue
	}
	return boolValues, nil
}

type TimeTransformer struct {
	Layout string
}

func (t TimeTransformer) Transform(values []string) (interface{}, error) {
	timeValues := make([]time.Time, len(values))
	for i, value := range values {
		parsedValue, err := time.Parse(t.Layout, value)
		if err != nil {
			return nil, err
		}
		timeValues[i] = parsedValue
	}
	return timeValues, nil
}
