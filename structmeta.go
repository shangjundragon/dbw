package dbw

import (
	"fmt"
	"reflect"
	"strings"
)

// 修改structMeta定义，添加更多字段信息
type fieldInfo struct {
	name    string // 字段名
	colName string // 数据库列名
	index   int    // 字段索引
	tag     reflect.StructTag
	dbwTag  map[string]string
}

// 结构体元数据结构
type structMeta struct {
	tableName     string               // 表名
	idGenerator   string               // 主键生成器名称
	fieldsInfoMap map[string]fieldInfo // 字段信息

	fieldMap         map[string]int    // 数据库列名到结构体字段索引的映射
	fieldDbColumnMap map[string]string // 结构体字段名到数据库列名的映射
	dbColumnFieldMap map[string]string // 数据库列名到结构体字段名的映射

	dbColumnSlice []string // 数据库列名（小写）切片

	tableIdProp     string // 表id属性名
	tableIdDbColumn string // 表id数据库列名

	logicDelFiledName string // 逻辑删除属性名
	logicDelDbColumn  string // 逻辑删除属性数据库列名

	autoCreateTimeFiledName string // 自动创建时间属性名
	autoCreateTimeDbColumn  string // 自动创建时间属性数据库列名
	autoCreateTimeTagValue  string // 自动创建时间tag值

	autoUpdateTimeFiledName string // 自动更新时间属性名
	autoUpdateTimeDbColumn  string // 自动更新时间属性数据库列名
	autoUpdateTimeTagValue  string // 自动更新时间tag值

}

func resolveDbwTag(dbwTag string) map[string]string {
	result := make(map[string]string)
	if dbwTag == "" {
		return result
	}

	// 按分号分割标签
	parts := strings.Split(dbwTag, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 按冒号分割键值对
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 1 {
			// 只有键没有值的情况，如 "primaryKey"
			result[kv[0]] = "true"
		} else {
			// 键值对，如 "type:varchar(100)"
			result[kv[0]] = strings.TrimSpace(kv[1])
		}
	}

	return result
}

// getStructMeta 获取结构体元数据（带缓存）
func getStructMeta[T any]() *structMeta {
	var t T
	typeOf := reflect.TypeOf(t)

	if meta, ok := structMetaCache.Load(typeOf); ok {
		return meta.(*structMeta)
	}

	meta := &structMeta{
		fieldsInfoMap:    make(map[string]fieldInfo),
		fieldMap:         make(map[string]int),
		fieldDbColumnMap: make(map[string]string),
		dbColumnFieldMap: make(map[string]string),
		dbColumnSlice:    make([]string, 0),
	}
	// 获取表名
	if tabler, ok := any(t).(Tabler); ok {
		meta.tableName = tabler.TableName()
	} else {
		meta.tableName = getTableName[T]()
	}

	for i := 0; i < typeOf.NumField(); i++ {
		field := typeOf.Field(i)

		dbwTag := resolveDbwTag(field.Tag.Get("dbw"))
		if dbwTag["ignore"] == "true" {

			continue
		}
		colName := dbwTag["column"]

		if colName == "" {
			// 如果没有db标签，使用字段名的蛇形命名
			colName = camelToSnake(field.Name)
		}

		// 主键
		if dbwTag["primaryKey"] == "true" {
			setIdMeta(meta, field)
		}
		// 逻辑删除
		if dbwTag["tableLogic"] == "true" {
			meta.logicDelFiledName = field.Name
			meta.logicDelDbColumn = colName
		}
		// 自动创建时间
		if dbwTag["autoCreateTime"] != "" {
			meta.autoCreateTimeFiledName = field.Name
			meta.autoCreateTimeDbColumn = colName
			meta.autoCreateTimeTagValue = dbwTag["autoCreateTime"]
		}
		// 自动更新时间
		if dbwTag["autoUpdateTime"] != "" {
			meta.autoUpdateTimeFiledName = field.Name
			meta.autoUpdateTimeDbColumn = colName
			meta.autoUpdateTimeTagValue = dbwTag["autoUpdateTime"]
		}

		fieldInfo := fieldInfo{
			name:    field.Name,
			colName: colName,
			index:   i,
			tag:     field.Tag,
			dbwTag:  dbwTag,
		}
		meta.fieldsInfoMap[field.Name] = fieldInfo
		meta.fieldDbColumnMap[field.Name] = colName
		meta.dbColumnFieldMap[colName] = field.Name
		meta.dbColumnSlice = append(meta.dbColumnSlice, colName)
		meta.fieldMap[colName] = i
	}

	// 循环完毕，如果未找到主键，则检查是否有一个名为Id的字段
	if meta.tableIdProp == "" {
		field, b := typeOf.FieldByName("Id")
		if b {
			setIdMeta(meta, field)
		}
	}

	structMetaCache.Store(typeOf, meta)
	if meta.tableIdProp == "" {
		fmt.Printf("%v table id property not found\n", typeOf)
	}
	return meta
}

// 设置主键元数据
func setIdMeta(meta *structMeta, field reflect.StructField) {
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
		panic(fmt.Sprintf("unsupported id type: %s only int, int64, uint64, string", fieldType))
	}
	meta.tableIdProp = field.Name
	meta.tableIdDbColumn = camelToSnake(field.Name)
}
