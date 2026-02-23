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
	return fmt.Sprintf("student:%d:exam:%s:session_start", studentID, examID)
}

// StudentShuffledQuestionKey returns the cache key for a student's shuffled questions
func (r *CacheKeyStruct) StudentShuffledQuestionKey(examID string, studentID int) string {
	return fmt.Sprintf("student:%d:exam:%s:shuffled_questions", studentID, examID)
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

// ExamCheatRulesKey returns the cache key for an exam's cheat rules
func (r *CacheKeyStruct) ExamCheatRulesKey(examID string) string {
	return fmt.Sprintf("exam:%s:cheat_rules", examID)
}

// ExamRandomOrderKey returns the cache key for an exam's random order
func (r *CacheKeyStruct) ExamRandomOrderKey(examID string) string {
	return fmt.Sprintf("exam:%s:random_order", examID)
}

// StudentActiveExamKey returns the cache key for a student's currently active exam
func (r *CacheKeyStruct) StudentActiveExamKey(studentID int) string {
	return fmt.Sprintf("student:%d:active_exam", studentID)
}

// ExamMonitorChannel returns the Redis PubSub channel name for an exam monitor
func (r *CacheKeyStruct) ExamMonitorChannel(examID string) string {
	return fmt.Sprintf("exam:%s:monitor", examID)
}

var CacheKey = NewCacheKeyStruct()
