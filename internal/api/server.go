package api

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"time"

	"be-project-monitoring/internal/domain/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oklog/run"
	"go.uber.org/zap"
)

type (
	Server struct {
		*http.Server
		logger *zap.SugaredLogger
		svc    Service

		shutdownTimeout int
	}

	Service interface {
		userService
		projectService
		participantService
		taskService
		tokenService
	}
	userService interface {
		CreateUser(ctx context.Context, user *CreateUserReq) (*model.User, string, error)
		AuthUser(ctx context.Context, username, password string) (string, error)
		GetFullUsers(ctx context.Context, userReq *GetUserReq) ([]model.User, int, error)
		GetPartialUsers(ctx context.Context, userReq *GetUserReq) ([]model.ShortUser, int, error)
		FindGithubUser(ctx context.Context, userReq string) bool
		UpdateUser(ctx context.Context, userReq *UpdateUserReq) (*model.User, error)
		DeleteUser(ctx context.Context, id uuid.UUID) error
		GetUserProfile(ctx context.Context, id uuid.UUID) (*model.Profile, error)
	}

	tokenService interface {
		GetUserIDFromToken(ctx context.Context, token string) (uuid.UUID, error)
		VerifyToken(ctx context.Context, token string, toAllow ...model.UserRole) error
		VerifySelf(ctx context.Context, token string, id uuid.UUID) error
		VerifyParticipant(ctx context.Context, userID uuid.UUID, projectID int) error
		VerifyParticipantRole(ctx context.Context, userID uuid.UUID, projectID int, toAllow ...model.ParticipantRole) error
	}

	projectService interface {
		CreateProject(ctx context.Context, projectReq *CreateProjectReq) (*model.Project, error)
		UpdateProject(ctx context.Context, projectReq *UpdateProjectReq) (*model.Project, error)
		DeleteProject(ctx context.Context, id int) error
		GetProjects(ctx context.Context, projectReq *GetProjectsReq) ([]model.Project, int, error)
		GetProjectInfo(ctx context.Context, id int) (*model.ProjectInfo, error)
	}

	participantService interface {
		AddParticipant(ctx context.Context, participant *AddParticipantReq) (*model.Participant, error)
		GetParticipantByID(ctx context.Context, id int) (*model.Participant, error)
		GetParticipants(ctx context.Context, projectID int) ([]model.Participant, error)
		DeleteParticipant(ctx context.Context, userID uuid.UUID, projectID int) error
	}

	taskService interface {
		CreateTask(ctx context.Context, task *CreateTaskReq) (*model.Task, error)
		UpdateTask(ctx context.Context, taskReq *UpdateTaskReq) (*model.Task, error)
		DeleteTask(ctx context.Context, id int) error
		GetTasks(ctx context.Context, taskReq *GetTasksReq) ([]model.Task, int, error)
		GetTaskInfo(ctx context.Context, id int) (*model.TaskInfo, error)
	}

	OptionFunc func(s *Server)
)

func New(opts ...OptionFunc) *Server {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	s := &Server{
		Server: &http.Server{
			Addr:         ":" + port,
			ReadTimeout:  time.Duration(10) * time.Second,
			WriteTimeout: time.Duration(10) * time.Second},
	}
	for _, opt := range opts {
		opt(s)
	}

	rtr := gin.Default()

	rtr.GET("/index", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl.html", nil)
	})

	// /api/*
	apiRtr := rtr.Group("/api")
	// /api/auth
	apiRtr.POST("/auth", s.auth)
	// /api/register
	apiRtr.POST("/register", s.register)

	// /api/user
	usersRtr := apiRtr.Group("/user")
	usersRtr.GET("/", s.getPartialUsers)
	usersRtr.GET("/:id", s.getUserProfile)
	usersRtr.POST("/:id", s.selfUpdateMiddleware(), s.updateUser)
	//usersRtr.DELETE("/:id", s.deleteUser)

	// /api/pm
	pmRtr := apiRtr.Group("/pm", s.authMiddleware(model.ProjectManager))
	pmRtr.POST("/", s.createProject)

	// /api/project
	projectRtr := apiRtr.Group("/project", s.authMiddleware(model.Admin, model.ProjectManager, model.Student))
	//projectRtr.GET("/projects", s.getProjects) ПОИСК ПРОЕКТОВ
	//projectRtr.POST("/", s.verifyParticipantMiddleware(), s.verifyParticipantRoleMiddleware(model.RoleOwner), s.updateProject)
	projectRtr.POST("/", s.updateProject)
	projectRtr.GET("/:project_id", s.getProjectInfo)
	projectRtr.DELETE("/:project_id", s.verifyParticipantRoleMiddleware(model.RoleOwner), s.deleteProject)
	projectRtr.POST("/:project_id/", s.verifyParticipantRoleMiddleware(model.RoleOwner, model.RoleTeamlead), s.addParticipant)
	projectRtr.DELETE("/:project_id/:user_id", s.verifyParticipantRoleMiddleware(model.RoleOwner, model.RoleTeamlead), s.deleteParticipant)

	// /api/project/task
	taskRtr := projectRtr.Group("/:project_id/task", s.verifyParticipantMiddleware())
	taskRtr.POST("/", s.createTask)
	taskRtr.PUT("/", s.updateTask)
	taskRtr.GET("/:task_id", s.getTaskInfo)
	taskRtr.DELETE("/:task_id", s.deleteTask)

	// /api/admin
	adminRtr := apiRtr.Group("/admin", s.authMiddleware(model.Admin))
	// /api/admin/users
	adminRtr.GET("/users", s.getFullUsers)
	adminRtr.POST("/users", s.updateUser)
	// /api/admin/projects
	adminRtr.GET("/projects", s.getProjects)

	s.Handler = rtr
	return s
}

func (s *Server) Run(g *run.Group) {
	g.Add(func() error {
		s.logger.Info("[http-server] started")
		s.logger.Info(fmt.Sprintf("listening on %v", s.Addr))
		return s.ListenAndServe()
	}, func(err error) {
		s.logger.Error("[http-server] terminated", zap.Error(err))

		ctx, cancel := context.WithTimeout(context.Background(), 30)
		defer cancel()

		s.logger.Error("[http-server] stopped", zap.Error(s.Shutdown(ctx)))
	})
}

func WithLogger(logger *zap.SugaredLogger) OptionFunc {
	return func(s *Server) {
		s.logger = logger
	}
}

/*func WithServer(srv *http.Server) OptionFunc {
	return func(s *Server) {
		s.Server = srv
	}
}*/

func WithService(svc Service) OptionFunc {
	return func(s *Server) {
		s.svc = svc
	}
}

func WithShutdownTimeout(timeout int) OptionFunc {
	return func(s *Server) {
		s.shutdownTimeout = timeout
	}
}
