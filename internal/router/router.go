package router

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/stemsi/exstem-backend/internal/config"
	"github.com/stemsi/exstem-backend/internal/handler"
	"github.com/stemsi/exstem-backend/internal/middleware"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
)

// Handlers groups all handler instances for route setup.
type Handlers struct {
	Auth          *handler.AuthHandler
	StudentPortal *handler.StudentPortalHandler
	StudentMgmt   *handler.StudentManagementHandler
	Admin         *handler.AdminHandler
	Exam          *handler.ExamHandler
	Question      *handler.QuestionHandler
	Media         *handler.MediaHandler
	WS            *handler.WSHandler
	AdminUser     *handler.AdminUserHandler
	AdminRole     *handler.AdminRoleHandler
	Class         *handler.ClassHandler
}

// SetupRouter configures all Gin route groups with appropriate middlewares.
func SetupRouter(
	authService *service.AuthService,
	handlers *Handlers,
	cfg *config.Config,
) *gin.Engine {
	gin.SetMode(cfg.GinMode)
	router := gin.Default()

	// ─── CORS ──────────────────────────────────────────────────────────
	// If AllowedOrigins is set in config, restrict to that list;
	// otherwise allow all (*) so dev works without extra config.
	corsConfig := cors.DefaultConfig()
	if len(cfg.AllowedOrigins) > 0 {
		corsConfig.AllowOrigins = cfg.AllowedOrigins
	} else {
		corsConfig.AllowAllOrigins = true
	}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"}
	corsConfig.ExposeHeaders = []string{"X-Request-ID"}
	corsConfig.MaxAge = 12 * time.Hour
	router.Use(cors.New(corsConfig))

	// Apply request ID middleware globally so every response includes metadata.
	router.Use(response.RequestIDMiddleware())

	// Serve uploaded media files statically.
	router.Static("/uploads", "./uploads")

	// Health check.
	router.GET("/health", func(c *gin.Context) {
		response.Success(c, http.StatusOK, gin.H{"status": "ok"})
	})

	// Rate limiter for auth routes (30 requests per minute per IP).
	authLimiter := middleware.NewRateLimiter(30, time.Minute)

	// ─── 1. Auth Group (Public, Rate Limited) ──────────────────────────
	auth := router.Group("/api/v1/auth")
	auth.Use(authLimiter.Middleware())
	{
		auth.POST("/student/login", handlers.Auth.StudentLogin)
		auth.POST("/admin/login", handlers.Auth.AdminLogin)

		// Authenticated profile routes
		auth.GET("/student/me", middleware.RequireStudentJWT(authService), handlers.Auth.GetStudentProfile)
		auth.GET("/admin/me", middleware.RequireAdminJWT(authService), handlers.Auth.GetAdminProfile)
	}

	// ─── 2. Student Group (JWT + Single Device) ────────────────────────
	studentAPI := router.Group("/api/v1/student")
	studentAPI.Use(
		middleware.RequireStudentJWT(authService),
		middleware.CheckSingleDeviceSession(authService),
	)
	{
		studentAPI.GET("/lobby", handlers.StudentPortal.GetLobby)
		studentAPI.POST("/exams/:exam_id/join", handlers.StudentPortal.JoinExam)
		studentAPI.GET("/exams/:exam_id/paper", handlers.StudentPortal.GetExamPaper)
	}

	// ─── 3. WebSocket Group (Student WS Auth) ──────────────────────────
	ws := router.Group("/ws/v1")
	ws.Use(middleware.RequireStudentWSAuth(authService))
	{
		ws.GET("/student/exams/:exam_id/stream", handlers.WS.ExamWebSocketStream)
	}

	// ─── 4. Admin Group (JWT + RBAC) ───────────────────────────────────
	adminAPI := router.Group("/api/v1/admin")
	adminAPI.Use(middleware.RequireAdminJWT(authService))
	{
		// Media upload
		adminAPI.POST("/media/upload",
			middleware.RequirePermission(string(model.PermissionMediaUpload)),
			handlers.Media.UploadMedia,
		)

		// Class management
		adminAPI.GET("/classes",
			middleware.RequirePermission(string(model.PermissionStudentsRead)),
			handlers.Class.ListClasses,
		)
		adminAPI.POST("/classes",
			middleware.RequirePermission(string(model.PermissionStudentsWrite)),
			handlers.Class.CreateClass,
		)
		adminAPI.PUT("/classes/:id",
			middleware.RequirePermission(string(model.PermissionStudentsWrite)),
			handlers.Class.UpdateClass,
		)
		adminAPI.DELETE("/classes/:id",
			middleware.RequirePermission(string(model.PermissionStudentsWrite)),
			handlers.Class.DeleteClass,
		)

		// Student management
		adminAPI.GET("/students",
			middleware.RequirePermission(string(model.PermissionStudentsRead)),
			handlers.StudentMgmt.ListStudents,
		)
		adminAPI.POST("/students",
			middleware.RequirePermission(string(model.PermissionStudentsWrite)),
			handlers.StudentMgmt.CreateStudent,
		)
		adminAPI.PUT("/students/:id",
			middleware.RequirePermission(string(model.PermissionStudentsWrite)),
			handlers.StudentMgmt.UpdateStudent,
		)
		adminAPI.DELETE("/students/:id",
			middleware.RequirePermission(string(model.PermissionStudentsWrite)),
			handlers.StudentMgmt.DeleteStudent,
		)
		adminAPI.POST("/students/:id/reset-session",
			middleware.RequirePermission(string(model.PermissionStudentsResetSession)),
			handlers.StudentMgmt.ResetStudentSession,
		)

		// Admin User Management
		adminAPI.GET("/users",
			middleware.RequirePermission(string(model.PermissionAdminsRead)),
			handlers.AdminUser.ListAdmins,
		)
		adminAPI.POST("/users",
			middleware.RequirePermission(string(model.PermissionAdminsWrite)),
			handlers.AdminUser.CreateAdmin,
		)
		adminAPI.PUT("/users/:id",
			middleware.RequirePermission(string(model.PermissionAdminsWrite)),
			handlers.AdminUser.UpdateAdmin,
		)
		adminAPI.DELETE("/users/:id",
			middleware.RequirePermission(string(model.PermissionAdminsWrite)),
			handlers.AdminUser.DeleteAdmin,
		)
		// Roles for selection (using read permission as it's needed for viewing user form)
		adminAPI.GET("/roles",
			middleware.RequirePermission(string(model.PermissionAdminsRead)),
			handlers.AdminUser.GetRoles,
		)

		// Admin Role Management
		adminAPI.GET("/roles/all",
			middleware.RequirePermission(string(model.PermissionRolesRead)),
			handlers.AdminRole.ListRoles,
		)
		adminAPI.GET("/roles/permissions",
			middleware.RequirePermission(string(model.PermissionRolesRead)),
			handlers.AdminRole.GetPermissions,
		)
		adminAPI.GET("/roles/:id",
			middleware.RequirePermission(string(model.PermissionRolesRead)),
			handlers.AdminRole.GetRole,
		)
		adminAPI.POST("/roles",
			middleware.RequirePermission(string(model.PermissionRolesWrite)),
			handlers.AdminRole.CreateRole,
		)
		adminAPI.PUT("/roles/:id",
			middleware.RequirePermission(string(model.PermissionRolesWrite)),
			handlers.AdminRole.UpdateRole,
		)
		adminAPI.DELETE("/roles/:id",
			middleware.RequirePermission(string(model.PermissionRolesWrite)),
			handlers.AdminRole.DeleteRole,
		)

		// Exam management
		adminAPI.GET("/exams",
			middleware.RequirePermission(string(model.PermissionExamsRead)),
			handlers.Exam.ListExams,
		)
		adminAPI.GET("/exams/:exam_id/results",
			middleware.RequirePermission(string(model.PermissionExamsRead)),
			handlers.Exam.GetExamResults,
		)
		adminAPI.POST("/exams",
			middleware.RequirePermission(string(model.PermissionExamsWriteOwn)),
			handlers.Exam.CreateExam,
		)
		adminAPI.POST("/exams/:exam_id/publish",
			middleware.RequirePermission(string(model.PermissionExamsPublish)),
			handlers.Exam.PublishExam,
		)
		adminAPI.POST("/exams/:exam_id/target-rules",
			middleware.RequirePermission(string(model.PermissionExamsWriteOwn)),
			handlers.Exam.AddTargetRule,
		)
		adminAPI.POST("/exams/:exam_id/refresh-cache",
			middleware.RequirePermission(string(model.PermissionExamsPublish)),
			handlers.Exam.RefreshExamCache,
		)

		// Question management
		adminAPI.GET("/exams/:exam_id/questions",
			middleware.RequirePermission(string(model.PermissionExamsRead)),
			handlers.Question.ListQuestions,
		)
		adminAPI.POST("/exams/:exam_id/questions",
			middleware.RequirePermission(string(model.PermissionExamsWriteOwn)),
			handlers.Question.AddQuestion,
		)
	}

	return router
}
