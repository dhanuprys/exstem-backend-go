package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/stemsi/exstem-backend/internal/config"
	"github.com/stemsi/exstem-backend/internal/database"
	"github.com/stemsi/exstem-backend/internal/handler"
	"github.com/stemsi/exstem-backend/internal/logger"
	"github.com/stemsi/exstem-backend/internal/repository"
	"github.com/stemsi/exstem-backend/internal/router"
	"github.com/stemsi/exstem-backend/internal/service"
	"github.com/stemsi/exstem-backend/internal/validator"
	"github.com/stemsi/exstem-backend/internal/worker"
)

func main() {
	// ─── Load Configuration ────────────────────────────────────────────
	cfg := config.Load()

	// ─── Initialize Logger ─────────────────────────────────────────────
	log := logger.Setup(cfg.LogLevel, cfg.LogFormat)
	log.Info().
		Str("port", cfg.ServerPort).
		Str("mode", cfg.GinMode).
		Str("log_level", cfg.LogLevel).
		Msg("Starting ExStem Backend")

	// ─── Initialize Validator ──────────────────────────────────────────
	validator.Setup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ─── Connect to PostgreSQL ─────────────────────────────────────────
	pool, err := database.NewPostgresPool(ctx, cfg, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}
	defer pool.Close()

	// ─── Connect to Redis ──────────────────────────────────────────────
	rdb, err := database.NewRedisClient(ctx, cfg, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer rdb.Close()

	// ─── Initialize Repositories ───────────────────────────────────────
	classRepo := repository.NewClassRepository(pool)
	_ = classRepo // Available for future use
	studentRepo := repository.NewStudentRepository(pool)
	adminRepo := repository.NewAdminRepository(pool)
	roleRepo := repository.NewRoleRepository(pool)
	examRepo := repository.NewExamRepository(pool)
	questionRepo := repository.NewQuestionRepository(pool)
	sessionRepo := repository.NewExamSessionRepository(pool)
	targetRepo := repository.NewExamTargetRuleRepository(pool)
	settingRepo := repository.NewSettingRepository(pool)
	subjectRepo := repository.NewSubjectRepository(pool)

	// ─── Initialize Services ──────────────────────────────────────────
	authService := service.NewAuthService(cfg, rdb)
	studentService := service.NewStudentService(studentRepo)
	adminService := service.NewAdminService(adminRepo, roleRepo)
	examService := service.NewExamService(examRepo, questionRepo, targetRepo, rdb, log)
	questionService := service.NewQuestionService(questionRepo)
	sessionService := service.NewExamSessionService(sessionRepo, examRepo, targetRepo)
	mediaService := service.NewMediaService(cfg)
	adminUserService := service.NewAdminUserService(pool)
	adminRoleService := service.NewAdminRoleService(roleRepo)
	classService := service.NewClassService(classRepo)
	settingService := service.NewSettingService(settingRepo, log)
	subjectService := service.NewSubjectService(subjectRepo, log)

	// ─── Initialize Handlers ──────────────────────────────────────────
	handlers := &router.Handlers{
		Auth:          handler.NewAuthHandler(authService, studentService, adminService),
		StudentPortal: handler.NewStudentPortalHandler(sessionService, examService),
		StudentMgmt:   handler.NewStudentManagementHandler(studentService, authService),
		Admin:         handler.NewAdminHandler(authService),
		Exam:          handler.NewExamHandler(examService, sessionService),
		Question:      handler.NewQuestionHandler(questionService),
		Media:         handler.NewMediaHandler(mediaService),
		WS:            handler.NewWSHandler(rdb, examService, sessionService, log, cfg.AllowedOrigins),
		AdminUser:     handler.NewAdminUserHandler(adminUserService),
		AdminRole:     handler.NewAdminRoleHandler(adminRoleService),
		Class:         handler.NewClassHandler(classService),
		Setting:       handler.NewSettingHandler(settingService),
		Subject:       handler.NewSubjectHandler(subjectService),
	}

	// ─── Start Background Workers ─────────────────────────────────────
	workerCtx, workerCancel := context.WithCancel(context.Background())

	autosaveWorker := worker.NewAutosaveWorker(pool, rdb, log)
	scoringWorker := worker.NewScoringWorker(pool, rdb, log)

	go autosaveWorker.Start(workerCtx)
	go scoringWorker.Start(workerCtx)

	// ─── Prewarm Redis Caches ─────────────────────────────────────────
	// Load all published exams into Redis BEFORE accepting traffic.
	// This avoids race conditions from lazy loading under thundering herd.
	if err := examService.PrewarmAllCaches(ctx); err != nil {
		log.Warn().Err(err).Msg("Cache prewarm failed")
	}

	// ─── Setup Router ──────────────────────────────────────────────────
	r := router.SetupRouter(authService, handlers, cfg)

	// ─── Create HTTP Server ────────────────────────────────────────────
	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: r,
	}

	// ─── Start Server in Goroutine ─────────────────────────────────────
	go func() {
		log.Info().Str("addr", ":"+cfg.ServerPort).Msg("Server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server error")
		}
	}()

	// ─── Graceful Shutdown ─────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Info().Str("signal", sig.String()).Msg("Shutting down gracefully...")

	// 1. Stop accepting new HTTP requests (5s timeout).
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP server shutdown error")
	}

	// 2. Stop background workers and wait for queues to drain.
	workerCancel()
	time.Sleep(2 * time.Second) // Allow workers to drain.

	log.Info().Msg("Shutdown complete")
}

// init sets zerolog global defaults before main runs.
func init() {
	zerolog.TimeFieldFormat = time.RFC3339
}
