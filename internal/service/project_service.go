package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"rag_project/internal/auth"
	"rag_project/internal/chat"
	"rag_project/internal/docling"
	"rag_project/internal/embedding"
	"rag_project/internal/projects"
	"rag_project/internal/settings"
	"rag_project/internal/training"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	trainingEventProgress = "training:progress"
	trainingEventFinished = "training:finished"
	chatEventToken        = "chat:token"
	chatEventDone         = "chat:done"
)

type ProjectService struct {
	settingsRepo settings.Repository
	trainer      *training.Manager
	auth         *auth.Service
	chatMu       sync.Mutex
	chatCancels  map[string]context.CancelFunc
}

func NewProjectService(settingsRepo settings.Repository, trainer *training.Manager, authService *auth.Service) *ProjectService {
	if trainer == nil {
		trainer = training.NewManager(2)
	}
	return &ProjectService{
		settingsRepo: settingsRepo,
		trainer:      trainer,
		auth:         authService,
		chatCancels:  map[string]context.CancelFunc{},
	}
}

// authorizeView allows admins and users assigned to the project to read it.
func (s *ProjectService) authorizeView(projectID string) error {
	if s.auth == nil {
		return nil
	}
	return s.auth.RequireProjectView(projectID)
}

// authorizeWrite reserves project mutations for the administrator.
func (s *ProjectService) authorizeWrite(projectID string) error {
	if s.auth == nil {
		return nil
	}
	return s.auth.RequireProjectWrite(projectID)
}

func (s *ProjectService) getStore() (*projects.Store, error) {
	settingsData, err := s.settingsRepo.Get()
	if err != nil {
		return nil, fmt.Errorf("load database settings: %w", err)
	}
	password, err := s.settingsRepo.DatabasePassword()
	if err != nil {
		return nil, fmt.Errorf("load database password: %w", err)
	}
	if settingsData.Database.Host == "" || settingsData.Database.DatabaseName == "" {
		return nil, errors.New("database is not configured yet. Please configure and save database settings first.")
	}
	return projects.NewStoreFromSettings(settingsData.Database, password)
}

func (s *ProjectService) ListProjects() ([]projects.Project, error) {
	allowed := []string(nil)
	if s.auth != nil {
		var err error
		allowed, err = s.auth.VisibleProjectIDs()
		if err != nil {
			return nil, err
		}
	}
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	list, err := store.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	if allowed == nil {
		return list, nil
	}
	permitted := map[string]bool{}
	for _, id := range allowed {
		permitted[id] = true
	}
	filtered := []projects.Project{}
	for _, project := range list {
		if permitted[project.ID] {
			filtered = append(filtered, project)
		}
	}
	return filtered, nil
}

func (s *ProjectService) GetProject(id string) (*projects.Project, error) {
	if err := s.authorizeView(id); err != nil {
		return nil, err
	}
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return store.GetProject(ctx, id)
}

func (s *ProjectService) CreateProject(name, description string) (*projects.Project, error) {
	if s.auth != nil {
		if err := s.auth.RequireAdmin(); err != nil {
			return nil, err
		}
	}
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return store.CreateProject(ctx, name, description)
}

func (s *ProjectService) UpdateProject(id, name, description string) (*projects.Project, error) {
	if err := s.authorizeWrite(id); err != nil {
		return nil, err
	}
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return store.UpdateProject(ctx, id, name, description)
}

func (s *ProjectService) DeleteProject(id string) error {
	if err := s.authorizeWrite(id); err != nil {
		return err
	}
	store, err := s.getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return store.DeleteProject(ctx, id)
}

func (s *ProjectService) UpdatePersona(projectId string, persona projects.PersonaConfig) error {
	if err := s.authorizeWrite(projectId); err != nil {
		return err
	}
	store, err := s.getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return store.UpdatePersona(ctx, projectId, persona)
}

// ImportDocuments stores uploaded files on the local disk and records their
// metadata in PostgreSQL.
func (s *ProjectService) ImportDocuments(projectId string, files []projects.DocumentInput) ([]projects.Document, error) {
	if err := s.authorizeWrite(projectId); err != nil {
		return nil, err
	}
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	return store.AddDocuments(ctx, projectId, files)
}

// DeleteDocument removes a document from PostgreSQL and deletes its local file.
func (s *ProjectService) DeleteDocument(projectId, documentId string) error {
	if err := s.authorizeWrite(projectId); err != nil {
		return err
	}
	store, err := s.getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return store.DeleteDocument(ctx, projectId, documentId)
}

func (s *ProjectService) MigrateDatabase() error {
	if s.auth != nil {
		if err := s.auth.RequireAdmin(); err != nil {
			return err
		}
	}
	settingsData, err := s.settingsRepo.Get()
	if err != nil {
		return fmt.Errorf("load database settings: %w", err)
	}
	if err := requireDatabase(settingsData.Database); err != nil {
		return err
	}
	if err := requireEmbedding(settingsData.EmbeddingModel); err != nil {
		return err
	}
	password, err := s.settingsRepo.DatabasePassword()
	if err != nil {
		return fmt.Errorf("load database password: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return projects.MigratePostgres(ctx, settingsData.Database, password, settingsData.EmbeddingModel.VectorDimension)
}

// TrainProject starts chunking the project's pending documents through Docling.
// It returns immediately and reports progress through Wails events.
func (s *ProjectService) TrainProject(projectID string) (*training.Progress, error) {
	if err := s.authorizeWrite(projectID); err != nil {
		return nil, err
	}
	if status, ok := s.trainer.Status(projectID); ok && status.Running() {
		return status, nil
	}

	settingsData, err := s.settingsRepo.Get()
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	if err := requireDatabase(settingsData.Database); err != nil {
		return nil, err
	}
	if err := requireEmbedding(settingsData.EmbeddingModel); err != nil {
		return nil, err
	}
	doclingConfig := settingsData.Docling
	if !doclingConfig.Configured {
		return nil, fmt.Errorf("document processing is not configured. Open Settings → Document processing and save it first")
	}
	if err := settings.ValidateDocling(doclingConfig); err != nil {
		return nil, fmt.Errorf("document processing is not configured correctly: %w", err)
	}

	apiKey, err := s.settingsRepo.DoclingAPIKey()
	if err != nil {
		return nil, fmt.Errorf("load Docling API key: %w", err)
	}

	embedder, err := s.newEmbedder(settingsData.EmbeddingModel)
	if err != nil {
		return nil, err
	}

	chunkOptions := docling.ChunkOptions{
		MaxTokens: settingsData.EmbeddingModel.ChunkMaxTokens,
		Tokenizer: settingsData.EmbeddingModel.Tokenizer,
	}
	chunker, err := docling.NewChunker(doclingConfig, apiKey, chunkOptions)
	if err != nil {
		return nil, err
	}

	store, err := s.getStore()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := store.EnsureDocumentStatusConstraint(ctx); err != nil {
		store.Close()
		return nil, err
	}
	if err := store.EnsureChunksVectorDimension(ctx, settingsData.EmbeddingModel.VectorDimension); err != nil {
		store.Close()
		return nil, err
	}

	if _, err := store.ResetStaleProcessing(ctx, projectID); err != nil {
		store.Close()
		return nil, err
	}

	documents, err := store.ListTrainableDocuments(ctx, projectID)
	if err != nil {
		store.Close()
		return nil, err
	}

	// If all documents were already processed (e.g. user clicked "Re-index knowledge base"),
	// automatically reset them so training/reindexing can run.
	if len(documents) == 0 {
		if _, err := store.ResetDocumentsForReindex(ctx, projectID); err != nil {
			store.Close()
			return nil, err
		}
		documents, err = store.ListTrainableDocuments(ctx, projectID)
		if err != nil {
			store.Close()
			return nil, err
		}
	}

	timeout := time.Duration(doclingConfig.TimeoutSeconds) * time.Second
	pipeline, err := training.NewPipeline(store, chunker, embedder, timeout, nil)
	if err != nil {
		store.Close()
		return nil, err
	}

	progress, started := s.trainer.Start(projectID, documents, pipeline, emitTrainingProgress)
	if !started {
		// Another request won the race for this project.
		store.Close()
	}
	return progress, nil
}

// ReindexDocument resets a single document so only it is re-chunked and embedded.
func (s *ProjectService) ReindexDocument(projectID, documentID string) error {
	if err := s.authorizeWrite(projectID); err != nil {
		return err
	}
	store, err := s.getStore()
	if err != nil {
		return err
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return store.ResetDocumentForReindex(ctx, projectID, documentID)
}

// CancelTraining stops an in-flight training job, if any.
func (s *ProjectService) CancelTraining(projectID string) error {
	if err := s.authorizeWrite(projectID); err != nil {
		return err
	}
	s.trainer.Cancel(projectID)
	return nil
}

// GetTrainingStatus returns the current job for a project, or nil when no job
// has been started in this app session.
func (s *ProjectService) GetTrainingStatus(projectID string) (*training.Progress, error) {
	status, ok := s.trainer.Status(projectID)
	if !ok {
		return nil, nil
	}
	return status, nil
}

func requireDatabase(cfg settings.Database) error {
	if !cfg.Configured {
		return errors.New("database is not configured. Open Settings → Database and save it first")
	}
	if err := settings.ValidateDatabase(cfg); err != nil {
		return fmt.Errorf("database settings are invalid: %w", err)
	}
	return nil
}

func requireEmbedding(cfg settings.EmbeddingModel) error {
	if !cfg.Configured {
		return errors.New("embedding model is not configured. Open Settings → Embedding Model and save it first")
	}
	if err := settings.ValidateEmbeddingModel(cfg); err != nil {
		return fmt.Errorf("embedding model settings are invalid: %w", err)
	}
	return nil
}

// AskProjectStream answers a question token by token. Tokens are emitted as
// Wails events (chat:token); the final ChatAnswer is also returned.
func (s *ProjectService) AskProjectStream(projectID, question string) (*projects.ChatAnswer, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return nil, errors.New("question is required")
	}
	if err := s.authorizeView(projectID); err != nil {
		return nil, err
	}
	all, err := s.settingsRepo.Get()
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	if err := requireDatabase(all.Database); err != nil {
		return nil, err
	}
	embedder, err := s.newEmbedder(all.EmbeddingModel)
	if err != nil {
		return nil, err
	}
	chatClient, err := s.newChatClient(all.ChatModel)
	if err != nil {
		return nil, err
	}
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()

	ctx, cancel := context.WithCancel(context.Background())
	s.chatMu.Lock()
	s.chatCancels[projectID] = cancel
	s.chatMu.Unlock()
	defer func() {
		cancel()
		s.chatMu.Lock()
		delete(s.chatCancels, projectID)
		s.chatMu.Unlock()
	}()

	project, err := store.GetProject(ctx, projectID)
	if err != nil || project == nil {
		return nil, fmt.Errorf("load project: %w", err)
	}
	vectors, err := embedder.Embed(ctx, []string{question})
	if err != nil {
		return nil, fmt.Errorf("embed question: %w", err)
	}
	retrieval, err := store.GetRetrievalConfig(ctx, projectID)
	if err != nil {
		return nil, err
	}
	hits, err := store.SearchChunksForQuery(ctx, projectID, question, vectors[0], retrieval)
	if err != nil {
		return nil, err
	}
	systemPrompt := chat.SystemPrompt(*project)
	if retrieval.Guardrails {
		systemPrompt += chat.GuardrailNotes(question)
	}

	started := time.Now()
	var answer strings.Builder
	_, err = chatClient.Stream(ctx, systemPrompt, chat.UserPrompt(question, hits), func(token string) {
		answer.WriteString(token)
		emitChatEvent(chatEventToken, map[string]string{"projectId": projectID, "token": token})
	})
	if err != nil {
		return nil, fmt.Errorf("chat model: %w", err)
	}
	result := &projects.ChatAnswer{
		ProjectID: projectID,
		Answer:    strings.TrimSpace(answer.String()),
		Sources:   chat.Sources(hits),
		Chunks:    hits,
		Debug: projects.ChatDebug{
			SystemPrompt: systemPrompt,
			LatencyMs:    time.Since(started).Milliseconds(),
			TopK:         retrieval.TopK,
		},
	}
	emitChatEvent(chatEventDone, result)
	return result, nil
}

// CancelChat stops an in-flight streaming answer for a project.
func (s *ProjectService) CancelChat(projectID string) error {
	if err := s.authorizeView(projectID); err != nil {
		return err
	}
	s.chatMu.Lock()
	defer s.chatMu.Unlock()
	if cancel, ok := s.chatCancels[projectID]; ok {
		cancel()
	}
	return nil
}

func emitChatEvent(name string, data any) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(name, data)
}

func emitTrainingProgress(progress training.Progress) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(trainingEventProgress, progress)
	if progress.Phase == training.PhaseFinished {
		app.Event.Emit(trainingEventFinished, progress)
	}
}

func (s *ProjectService) newEmbedder(cfg settings.EmbeddingModel) (embedding.Embedder, error) {
	if err := requireEmbedding(cfg); err != nil {
		return nil, err
	}
	apiKey, err := s.settingsRepo.EmbeddingAPIKey()
	if err != nil {
		return nil, fmt.Errorf("load embedding API key: %w", err)
	}
	return embedding.New(cfg, apiKey)
}

func (s *ProjectService) newChatClient(cfg settings.ChatModel) (chat.Client, error) {
	if !cfg.Configured {
		return nil, errors.New("chat model is not configured. Open Settings → Chat model and save it first")
	}
	if err := settings.ValidateChatModel(cfg); err != nil {
		return nil, fmt.Errorf("chat model settings are invalid: %w", err)
	}
	apiKey, err := s.settingsRepo.ChatModelAPIKey()
	if err != nil {
		return nil, fmt.Errorf("load chat API key: %w", err)
	}
	return chat.New(cfg, apiKey)
}

// RetrieveChunks embeds a query and returns the closest chunks of one project.
func (s *ProjectService) RetrieveChunks(projectID, query string, limit int) ([]projects.ChunkHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("query is required")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.New("project id is required")
	}
	if limit < 1 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	if err := s.authorizeView(projectID); err != nil {
		return nil, err
	}

	all, err := s.settingsRepo.Get()
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	embedder, err := s.newEmbedder(all.EmbeddingModel)
	if err != nil {
		return nil, err
	}
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	vectors, err := embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	retrieval, err := store.GetRetrievalConfig(ctx, projectID)
	if err != nil {
		return nil, err
	}
	retrieval.TopK = limit
	return store.SearchChunksForQuery(ctx, projectID, query, vectors[0], retrieval)
}

// AskProject answers a question with the project's persona and knowledge.
// It returns the answer together with the sources and chunks used, so the UI
// can show real evidence instead of mock data.
func (s *ProjectService) AskProject(projectID, question string) (*projects.ChatAnswer, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return nil, errors.New("question is required")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.New("project id is required")
	}
	if err := s.authorizeView(projectID); err != nil {
		return nil, err
	}

	all, err := s.settingsRepo.Get()
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	if err := requireDatabase(all.Database); err != nil {
		return nil, err
	}
	embedder, err := s.newEmbedder(all.EmbeddingModel)
	if err != nil {
		return nil, err
	}
	chatClient, err := s.newChatClient(all.ChatModel)
	if err != nil {
		return nil, err
	}
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	project, err := store.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("load project: %w", err)
	}
	if project == nil {
		return nil, fmt.Errorf("project %s was not found", projectID)
	}

	vectors, err := embedder.Embed(ctx, []string{question})
	if err != nil {
		return nil, fmt.Errorf("embed question: %w", err)
	}
	retrieval, err := store.GetRetrievalConfig(ctx, projectID)
	if err != nil {
		return nil, err
	}
	hits, err := store.SearchChunksForQuery(ctx, projectID, question, vectors[0], retrieval)
	if err != nil {
		return nil, err
	}

	systemPrompt := chat.SystemPrompt(*project)
	if retrieval.Guardrails {
		systemPrompt += chat.GuardrailNotes(question)
	}
	started := time.Now()
	answer, err := chatClient.Complete(ctx, systemPrompt, chat.UserPrompt(question, hits))
	latency := time.Since(started).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("chat model: %w", err)
	}

	return &projects.ChatAnswer{
		ProjectID: projectID,
		Answer:    strings.TrimSpace(answer),
		Sources:   chat.Sources(hits),
		Chunks:    hits,
		Debug: projects.ChatDebug{
			SystemPrompt: systemPrompt,
			LatencyMs:    latency,
			TopK:         retrieval.TopK,
		},
	}, nil
}

// GetRetrievalConfig returns the retrieval settings of a project.
func (s *ProjectService) GetRetrievalConfig(projectID string) (*projects.RetrievalConfig, error) {
	if err := s.authorizeView(projectID); err != nil {
		return nil, err
	}
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cfg, err := store.GetRetrievalConfig(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// UpdateRetrievalConfig saves the retrieval settings of a project.
func (s *ProjectService) UpdateRetrievalConfig(projectID string, cfg projects.RetrievalConfig) error {
	if err := s.authorizeWrite(projectID); err != nil {
		return err
	}
	store, err := s.getStore()
	if err != nil {
		return err
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return store.UpdateRetrievalConfig(ctx, projectID, cfg)
}

// CreateEvalSet adds an evaluation set to a project.
func (s *ProjectService) CreateEvalSet(projectID, name string) (*projects.EvalSet, error) {
	if err := s.authorizeWrite(projectID); err != nil {
		return nil, err
	}
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return store.CreateEvalSet(ctx, projectID, name)
}

// ListEvalSets returns evaluation sets of a project.
func (s *ProjectService) ListEvalSets(projectID string) ([]projects.EvalSet, error) {
	if err := s.authorizeView(projectID); err != nil {
		return nil, err
	}
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return store.ListEvalSets(ctx, projectID)
}

// AddEvalCase appends a test case to an evaluation set.
func (s *ProjectService) AddEvalCase(setID, question, expectedAnswer, expectedSource string) (*projects.EvalCase, error) {
	set, err := s.getEvalSet(setID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeWrite(set.ProjectID); err != nil {
		return nil, err
	}
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return store.AddEvalCase(ctx, setID, question, expectedAnswer, expectedSource)
}

// ListEvalCases returns the test cases of an evaluation set.
func (s *ProjectService) ListEvalCases(setID string) ([]projects.EvalCase, error) {
	set, err := s.getEvalSet(setID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeView(set.ProjectID); err != nil {
		return nil, err
	}
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return store.ListEvalCases(ctx, setID)
}

// RunEvaluation executes every case and reports answer/source matches.
func (s *ProjectService) RunEvaluation(setID string) (*projects.EvalReport, error) {
	set, err := s.getEvalSet(setID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeWrite(set.ProjectID); err != nil {
		return nil, err
	}
	all, err := s.settingsRepo.Get()
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	embedder, err := s.newEmbedder(all.EmbeddingModel)
	if err != nil {
		return nil, err
	}
	chatClient, err := s.newChatClient(all.ChatModel)
	if err != nil {
		return nil, err
	}
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	project, err := store.GetProject(ctx, set.ProjectID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, errors.New("evaluation project no longer exists")
	}
	retrieval, err := store.GetRetrievalConfig(ctx, set.ProjectID)
	if err != nil {
		return nil, err
	}
	cases, err := store.ListEvalCases(ctx, setID)
	if err != nil {
		return nil, err
	}
	systemPrompt := chat.SystemPrompt(*project)
	report := &projects.EvalReport{SetID: setID, Total: len(cases), Results: []projects.EvalCaseResult{}}

	for _, evalCase := range cases {
		vectors, embedErr := embedder.Embed(ctx, []string{evalCase.Question})
		if embedErr != nil {
			return nil, fmt.Errorf("embed evaluation question: %w", embedErr)
		}
		hits, searchErr := store.SearchChunksForQuery(ctx, set.ProjectID, evalCase.Question, vectors[0], retrieval)
		if searchErr != nil {
			return nil, searchErr
		}
		answer, chatErr := chatClient.Complete(ctx, systemPrompt, chat.UserPrompt(evalCase.Question, hits))
		if chatErr != nil {
			return nil, fmt.Errorf("evaluation chat call: %w", chatErr)
		}
		sources := chat.Sources(hits)
		result := projects.EvalCaseResult{
			Question:   evalCase.Question,
			Answer:     strings.TrimSpace(answer),
			Sources:    sources,
			NoEvidence: len(hits) == 0,
		}
		if expected := strings.TrimSpace(evalCase.ExpectedAnswer); expected != "" {
			result.AnswerMatched = strings.Contains(strings.ToLower(result.Answer), strings.ToLower(expected))
		}
		if expected := strings.TrimSpace(evalCase.ExpectedSourceName); expected != "" {
			for _, source := range sources {
				if strings.EqualFold(source.FileName, expected) {
					result.SourceMatched = true
					break
				}
			}
		}
		if strings.TrimSpace(result.Answer) != "" {
			report.Answered++
		}
		if result.AnswerMatched {
			report.AnswerHit++
		}
		if result.SourceMatched {
			report.SourceHit++
		}
		report.Results = append(report.Results, result)
	}
	return report, nil
}

func (s *ProjectService) getEvalSet(setID string) (*projects.EvalSet, error) {
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return store.GetEvalSet(ctx, setID)
}

func (s *ProjectService) ListPromptTemplates() ([]projects.PromptTemplate, error) {
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return store.ListPromptTemplates(ctx)
}

// SavePromptTemplate creates or updates a persona instruction template.
func (s *ProjectService) SavePromptTemplate(name, instructions string) (*projects.PromptTemplate, error) {
	if s.auth != nil {
		if err := s.auth.RequireAdmin(); err != nil {
			return nil, err
		}
	}
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return store.SavePromptTemplate(ctx, name, instructions)
}
