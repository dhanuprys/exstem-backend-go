package config

type WorkerKeyStruct struct {
	PersistCheatsQueue  string
	PersistAnswersQueue string
	PersistScoresQueue  string
}

var WorkerKey = &WorkerKeyStruct{
	PersistCheatsQueue:  "persist_cheats_queue",
	PersistAnswersQueue: "persist_answers_queue",
	PersistScoresQueue:  "persist_scores_queue",
}
