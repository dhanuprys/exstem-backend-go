package config

import (
	"fmt"
)

type CacheKeyStruct struct{}

func NewCacheKeyStruct() *CacheKeyStruct {
	return &CacheKeyStruct{}
}

// StudentSessionKey returns the cache key for a student's session
func (r *CacheKeyStruct) StudentSessionKey(studentID int) string {
	return fmt.Sprintf("login:%d", studentID)
}

// StudentExamSessionStartKey returns the cache key for a student's exam session start
func (r *CacheKeyStruct) StudentExamSessionStartKey(examID string, studentID int) string {
	return fmt.Sprintf("exam:%d:student:%d:session_start", examID, studentID)
}

// StudentAnswersKey returns the cache key for a student's answers
func (r *CacheKeyStruct) StudentAnswersKey(examID string, studentID int) string {
	return fmt.Sprintf("student:%d:exam:%s:answers", studentID, examID)
}

// ExamPayloadKey returns the cache key for an exam's payload
func (r *CacheKeyStruct) ExamPayloadKey(examID string) string {
	return fmt.Sprintf("exam:%s:payload", examID)
}

// ExamDurationKey returns the cache key for an exam's duration
func (r *CacheKeyStruct) ExamDurationKey(examID string) string {
	return fmt.Sprintf("exam:%s:duration", examID)
}

// ExamAnswerKey returns the cache key for an exam's answer
func (r *CacheKeyStruct) ExamAnswerKey(examID string) string {
	return fmt.Sprintf("exam:%s:key", examID)
}

var CacheKey = NewCacheKeyStruct()
