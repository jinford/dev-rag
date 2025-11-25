package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// WikiService はWiki生成のユースケースを提供します
type WikiService struct {
	orchestrator *WikiOrchestrator
	log          *slog.Logger
}

// NewWikiService は新しいWikiServiceを作成します
func NewWikiService(orchestrator *WikiOrchestrator, log *slog.Logger) *WikiService {
	return &WikiService{
		orchestrator: orchestrator,
		log:          log,
	}
}

// GenerateWiki はWikiを生成します
func (s *WikiService) GenerateWiki(ctx context.Context, sourceID, snapshotID uuid.UUID) error {
	// バリデーション
	if sourceID == uuid.Nil {
		return fmt.Errorf("source ID is required")
	}
	if snapshotID == uuid.Nil {
		return fmt.Errorf("snapshot ID is required")
	}

	s.log.Info("Starting wiki generation",
		"sourceID", sourceID,
		"snapshotID", snapshotID,
	)

	// オーケストレーターを使ってWiki生成フローを実行
	if err := s.orchestrator.GenerateWiki(ctx, sourceID, snapshotID); err != nil {
		s.log.Error("Wiki generation failed",
			"sourceID", sourceID,
			"snapshotID", snapshotID,
			"error", err,
		)
		return fmt.Errorf("failed to generate wiki: %w", err)
	}

	s.log.Info("Wiki generation completed",
		"sourceID", sourceID,
		"snapshotID", snapshotID,
	)

	return nil
}

// RegenerateWiki はWikiを再生成します
func (s *WikiService) RegenerateWiki(ctx context.Context, sourceID, snapshotID uuid.UUID) error {
	s.log.Info("Starting wiki regeneration",
		"sourceID", sourceID,
		"snapshotID", snapshotID,
	)

	// 既存の要約を削除してから再生成
	// TODO: 既存要約の削除機能を実装する必要がある場合は追加
	// 現在の実装では、既に要約が存在する場合はスキップされる

	return s.GenerateWiki(ctx, sourceID, snapshotID)
}
