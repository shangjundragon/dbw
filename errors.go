package dbw

import "errors"

// ErrRecordNotFound is returned when FindOne finds no matching record.
var ErrRecordNotFound = errors.New("dbw: record not found")

// ErrMultipleRecords is returned when SelectOne or SelectById finds more than one record.
var ErrMultipleRecords = errors.New("dbw: expected 1 record, got multiple")

// ErrNoWhereClause is returned when Update or Delete is called without WHERE conditions.
var ErrNoWhereClause = errors.New("dbw: dangerous operation without WHERE clause")

// ErrNoFieldsToUpdate is returned when Insert or UpdateById has no fields to set.
var ErrNoFieldsToUpdate = errors.New("dbw: no fields to update")

// ErrNoPrimaryKey is returned when a struct has no primary key configured.
var ErrNoPrimaryKey = errors.New("dbw: primary key not configured on struct")

// ErrBatchTooLarge is returned when InsertBatch exceeds 1000 records.
var ErrBatchTooLarge = errors.New("dbw: batch size exceeds maximum limit of 1000, use InsertBatchSplit")

// ErrEmptyData is returned when an empty slice is passed to batch operations.
var ErrEmptyData = errors.New("dbw: data slice is empty")

// ErrNilEntity is returned when a nil pointer is passed to Insert or InsertBatch.
var ErrNilEntity = errors.New("dbw: entity cannot be nil")
