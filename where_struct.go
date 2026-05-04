package dbw

import "reflect"

// WhereStruct adds WHERE conditions from non-zero struct fields.
func (q *DbWrapper[T]) WhereStruct(data *T) *DbWrapper[T] {
	if data == nil {
		return q
	}
	val := reflect.ValueOf(data).Elem()
	typ := val.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldInfo, ok := q.meta.fieldsInfoMap[field.Name]
		if !ok || fieldInfo.dbIgnore {
			continue
		}
		fieldValue := val.Field(i)
		if fieldValue.IsZero() {
			continue
		}
		if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
			continue
		}
		q.Eq(fieldInfo.dbColumn, fieldValue.Interface())
	}
	return q
}
