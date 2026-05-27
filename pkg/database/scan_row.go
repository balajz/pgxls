package database

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func Columns(rows pgx.Rows) ([]string, error) {
	fields := rows.FieldDescriptions()
	cols := make([]string, len(fields))

	for i, f := range fields {
		c := f.Name
		if strings.TrimSpace(c) == "" {
			c = fmt.Sprintf("col%d", i)
		}
		cols[i] = c
	}

	return cols, nil
}

func ScanRows(rows pgx.Rows, columnLength int) ([][]string, error) {
	stringRows := [][]string{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}

		stringRow := make([]string, columnLength)
		for i, val := range values {
			if i >= columnLength {
				break
			}
			s, err := sqlValToString(val)
			if err != nil {
				return nil, err
			}
			stringRow[i] = s
		}
		stringRows = append(stringRows, stringRow)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stringRows, nil
}

func sqlValToString(val interface{}) (string, error) {
	res := ""
	if val == nil {
		return res, nil
	}

	reflectVal := reflect.ValueOf(val)

	// extract pointer value
	if reflectVal.Kind() == reflect.Pointer {
		if reflectVal.IsNil() {
			return "", nil
		}
		val = reflectVal.Elem().Interface()
	}

	switch v := (val).(type) {
	case []byte:
		res = string(v)
	case string:
		res = v
	case time.Time:
		res = v.Format(time.RFC3339Nano)
	case fmt.Stringer:
		res = v.String()
	case map[string]interface{}:
		buf, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		res = string(buf)
	case []interface{}:
		buf, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		res = string(buf)
	default:
		res = fmt.Sprintf("%v", v)
	}

	return res, nil
}
