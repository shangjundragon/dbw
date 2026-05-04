package dbw

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

var structMetaCache sync.Map

// fieldInfo holds metadata for a single struct field.
type fieldInfo struct {
	dbIgnore bool
	name     string
	dbColumn string
	index    int
	tag      reflect.StructTag
	dbwTag   map[string]string
}

// structMeta holds parsed struct metadata for ORM operations.
type structMeta struct {
	tableName               string
	idGenerator             string
	fieldsInfoMap           map[string]fieldInfo
	fieldMap                map[string]int    // lowercase db column name → struct field index
	dbColumnFieldMap        map[string]string // struct field name → db column name
	dbColumnFieldNameMap    map[string]string // db column name → struct field name
	dbColumnSlice           []string
	tableIdFieldName        string
	tableIdDbColumn         string
	logicDelFieldName       string
	logicDelDbColumn        string
	autoCreateTimeFieldName string
	autoCreateTimeDbColumn  string
	autoCreateTimeTagValue  string
	autoUpdateTimeFieldName string
	autoUpdateTimeDbColumn  string
	autoUpdateTimeTagValue  string
}

// getStructMeta returns cached struct metadata for type T.
func getStructMeta[T any]() *structMeta {
	var t T
	typeOf := reflect.TypeOf(t)

	if meta, ok := structMetaCache.Load(typeOf); ok {
		return meta.(*structMeta)
	}

	meta := &structMeta{
		fieldsInfoMap:        make(map[string]fieldInfo),
		fieldMap:             make(map[string]int),
		dbColumnFieldMap:     make(map[string]string),
		dbColumnFieldNameMap: make(map[string]string),
		dbColumnSlice:        make([]string, 0),
	}

	if tabler, ok := any(t).(Tabler); ok {
		meta.tableName = tabler.TableName()
	} else {
		meta.tableName = getTableName[T]()
	}

	for i := 0; i < typeOf.NumField(); i++ {
		field := typeOf.Field(i)
		dbwTag := resolveDbwTag(field.Tag.Get("dbw"))
		if dbwTag["dbIgnore"] == "true" {
			continue
		}
		colName := dbwTag["column"]
		if colName == "" {
			colName = camelToSnake(field.Name)
		}

		if dbwTag["primaryKey"] == "true" {
			setIdMeta(meta, field, colName)
		}
		if dbwTag["tableLogic"] == "true" {
			meta.logicDelFieldName = field.Name
			meta.logicDelDbColumn = colName
		}
		if dbwTag["autoCreateTime"] != "" {
			meta.autoCreateTimeFieldName = field.Name
			meta.autoCreateTimeDbColumn = colName
			meta.autoCreateTimeTagValue = dbwTag["autoCreateTime"]
		}
		if dbwTag["autoUpdateTime"] != "" {
			meta.autoUpdateTimeFieldName = field.Name
			meta.autoUpdateTimeDbColumn = colName
			meta.autoUpdateTimeTagValue = dbwTag["autoUpdateTime"]
		}

		fi := fieldInfo{
			dbIgnore: dbwTag["dbIgnore"] == "true",
			name:     field.Name,
			dbColumn: colName,
			index:    i,
			tag:      field.Tag,
			dbwTag:   dbwTag,
		}
		meta.fieldsInfoMap[field.Name] = fi
		meta.dbColumnFieldMap[field.Name] = colName
		meta.dbColumnFieldNameMap[colName] = field.Name
		meta.dbColumnSlice = append(meta.dbColumnSlice, colName)
		meta.fieldMap[strings.ToLower(colName)] = i
	}

	if meta.tableIdFieldName == "" {
		if field, ok := typeOf.FieldByName("Id"); ok {
			dbwTag := resolveDbwTag(field.Tag.Get("dbw"))
			colName := dbwTag["column"]
			if colName == "" {
				colName = camelToSnake(field.Name)
			}
			if field, ok := typeOf.FieldByName("Id"); ok && !field.Anonymous {
				setIdMeta(meta, field, colName)
			} else {
				setIdMeta(meta, field, colName)
			}
		}
	}

	structMetaCache.Store(typeOf, meta)
	if meta.tableIdFieldName == "" {
		fmt.Printf("dbw: %v table id property not found\n", typeOf)
	}
	return meta
}

// setIdMeta configures the primary key metadata for a struct field.
func setIdMeta(meta *structMeta, field reflect.StructField, colName string) {
	fieldType := field.Type
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}
	dbwTag := resolveDbwTag(field.Tag.Get("dbw"))
	switch fieldType.Kind() {
	case reflect.Int, reflect.Int64, reflect.Uint64:
		if dbwTag["idGenerator"] == "" {
			meta.idGenerator = "snowflake"
		} else {
			meta.idGenerator = dbwTag["idGenerator"]
		}
	case reflect.String:
		if dbwTag["idGenerator"] == "" {
			meta.idGenerator = "snowflakeStr"
		} else {
			meta.idGenerator = dbwTag["idGenerator"]
		}
	default:
		panic(fmt.Sprintf("dbw: unsupported id type: %s, only int, int64, uint64, string", fieldType))
	}
	meta.tableIdFieldName = field.Name
	meta.tableIdDbColumn = colName
}

// resolveDbwTag parses a dbw struct tag string into a key-value map.
func resolveDbwTag(tag string) map[string]string {
	result := make(map[string]string)
	if tag == "" {
		return result
	}
	parts := strings.Split(tag, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 1 {
			result[kv[0]] = "true"
		} else {
			result[kv[0]] = strings.TrimSpace(kv[1])
		}
	}
	return result
}
