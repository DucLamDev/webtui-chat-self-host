package application

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

const maxSummaryMessages = 500

type TalkAISummaryMessage struct {
	SenderName string `json:"sender_name"`
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
}

type TalkAIRepository interface {
	ListMessagesForSummary(
		ctx context.Context,
		workspaceID string,
		channelID string,
		since *time.Time,
		limit int,
	) ([]TalkAISummaryMessage, error)
}

type TalkAIProvider interface {
	Summarize(
		ctx context.Context,
		provider string,
		config json.RawMessage,
		messages []TalkAISummaryMessage,
		language string,
	) (TalkSummaryResult, error)
}

type TalkSummaryResult struct {
	Summary     string   `json:"summary"`
	Decisions   []string `json:"decisions"`
	ActionItems []string `json:"action_items"`
	Model       string   `json:"model"`
}

type TalkSummaryDTO struct {
	TalkSummaryResult
	MessageCount int    `json:"message_count"`
	GeneratedAt  string `json:"generated_at"`
}

func (s *Service) SummarizeChannel(
	ctx context.Context,
	actorUserID string,
	workspaceID string,
	channelID string,
	sinceValue string,
	language string,
) (TalkSummaryDTO, error) {
	if err := s.ensureCollaborationMember(
		ctx,
		actorUserID,
		workspaceID,
		channelID,
	); err != nil {
		return TalkSummaryDTO{}, err
	}
	integration, err := s.talkGovernanceRepository().GetTalkIntegration(
		ctx,
		strings.TrimSpace(workspaceID),
	)
	if err != nil {
		return TalkSummaryDTO{}, err
	}
	if !integration.AIEnabled {
		return TalkSummaryDTO{}, apperrors.Conflict(
			"TALK_AI_DISABLED",
			"AI is disabled for this workspace.",
		)
	}
	if s.talkAI == nil {
		return TalkSummaryDTO{}, apperrors.Conflict(
			"TALK_AI_UNAVAILABLE",
			"No local AI adapter is configured.",
		)
	}
	var since *time.Time
	if strings.TrimSpace(sinceValue) != "" {
		parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(sinceValue))
		if parseErr != nil {
			return TalkSummaryDTO{}, apperrors.BadRequest(
				"VALIDATION_ERROR",
				"since must be an RFC3339 timestamp.",
			)
		}
		utc := parsed.UTC()
		since = &utc
	}
	repository, ok := s.collab.(TalkAIRepository)
	if !ok {
		return TalkSummaryDTO{}, talkCapabilityUnavailable()
	}
	messages, err := repository.ListMessagesForSummary(
		ctx,
		strings.TrimSpace(workspaceID),
		strings.TrimSpace(channelID),
		since,
		maxSummaryMessages,
	)
	if err != nil {
		return TalkSummaryDTO{}, err
	}
	if len(messages) == 0 {
		return TalkSummaryDTO{}, apperrors.BadRequest(
			"NO_MESSAGES",
			"There are no messages to summarize.",
		)
	}
	language = strings.TrimSpace(language)
	if language == "" {
		language = "vi"
	}
	result, err := s.talkAI.Summarize(
		ctx,
		integration.AIProvider,
		integration.Config,
		messages,
		language,
	)
	if err != nil {
		return TalkSummaryDTO{}, apperrors.Internal(
			"Local AI could not summarize this conversation.",
		)
	}
	return TalkSummaryDTO{
		TalkSummaryResult: result,
		MessageCount:      len(messages),
		GeneratedAt:       time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}
