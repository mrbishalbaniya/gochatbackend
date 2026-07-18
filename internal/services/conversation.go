package services

import (
	"context"
	"errors"
	"time"

	"github.com/pulse/chat-service/internal/constants"
	"github.com/pulse/chat-service/internal/dto"
	"github.com/pulse/chat-service/internal/models"
	"github.com/pulse/chat-service/internal/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ConversationView struct {
	Conversation models.Conversation         `json:"conversation"`
	Membership   models.ConversationMember   `json:"membership"`
	LastMessage  *models.Message             `json:"lastMessage,omitempty"`
	Members      []models.ConversationMember `json:"members,omitempty"`
}

func (s *Services) CreateConversation(ctx context.Context, userID uuid.UUID, req dto.CreateConversationRequest) (*ConversationView, error) {
	typ := req.Type
	switch typ {
	case constants.ConversationDirect, constants.ConversationGroup, constants.ConversationBroadcast,
		constants.ConversationSelf, constants.ConversationTemporary, constants.ConversationPrivate:
	default:
		return nil, errors.New("invalid conversation type")
	}

	memberIDs := uniqueUUIDs(req.MemberIDs)
	if typ == constants.ConversationDirect {
		if len(memberIDs) != 1 {
			return nil, errors.New("direct chat requires exactly one other member")
		}
		other := memberIDs[0]
		blocked, err := s.Blocks.IsBlocked(ctx, userID, other)
		if err != nil {
			return nil, err
		}
		if blocked {
			return nil, errors.New("cannot message blocked user")
		}
		if existing, err := s.Conversations.FindDirect(ctx, userID, other); err == nil {
			m, _ := s.Conversations.GetMember(ctx, existing.ID, userID)
			return &ConversationView{Conversation: *existing, Membership: *m}, nil
		}
	}
	if typ == constants.ConversationSelf {
		memberIDs = nil
	}
	if typ == constants.ConversationGroup && req.Title == "" {
		req.Title = "New group"
	}

	now := time.Now().UTC()
	conv := &models.Conversation{
		Type: typ, Title: utils.SanitizeText(req.Title, 200), Description: utils.SanitizeText(req.Description, 1000),
		CreatedBy: userID, IsTemporary: req.IsTemporary || typ == constants.ConversationTemporary,
	}
	if conv.IsTemporary {
		exp := now.Add(24 * time.Hour)
		conv.ExpiresAt = &exp
	}

	members := []models.ConversationMember{{
		UserID: userID, Role: constants.MemberRoleOwner, JoinedAt: now,
	}}
	for _, id := range memberIDs {
		if id == userID {
			continue
		}
		if _, err := s.Users.FindByID(ctx, id); err != nil {
			return nil, errors.New("member not found: " + id.String())
		}
		members = append(members, models.ConversationMember{
			UserID: id, Role: constants.MemberRoleMember, JoinedAt: now,
		})
	}

	if err := s.Conversations.Create(ctx, conv, members); err != nil {
		return nil, err
	}
	membership, _ := s.Conversations.GetMember(ctx, conv.ID, userID)
	view := &ConversationView{Conversation: *conv, Membership: *membership, Members: members}
	s.publish(ctx, constants.WSEventConversation, &conv.ID, &userID, view)
	return view, nil
}

func (s *Services) ListConversations(ctx context.Context, userID uuid.UUID, archived bool) ([]ConversationView, error) {
	convs, memberMap, err := s.Conversations.ListForUser(ctx, userID, archived, 100)
	if err != nil {
		return nil, err
	}
	out := make([]ConversationView, 0, len(convs))
	for _, c := range convs {
		view := ConversationView{Conversation: c, Membership: memberMap[c.ID]}
		if lm, err := s.Messages.Latest(ctx, c.ID); err == nil {
			view.LastMessage = lm
		}
		out = append(out, view)
	}
	return out, nil
}

func (s *Services) GetConversation(ctx context.Context, userID, conversationID uuid.UUID) (*ConversationView, error) {
	ok, err := s.Conversations.IsMember(ctx, conversationID, userID)
	if err != nil || !ok {
		return nil, errors.New("forbidden")
	}
	c, err := s.Conversations.FindByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	m, err := s.Conversations.GetMember(ctx, conversationID, userID)
	if err != nil {
		return nil, err
	}
	members, _ := s.Conversations.ListMembers(ctx, conversationID)
	view := &ConversationView{Conversation: *c, Membership: *m, Members: members}
	if lm, err := s.Messages.Latest(ctx, c.ID); err == nil {
		view.LastMessage = lm
	}
	return view, nil
}

func (s *Services) SetArchived(ctx context.Context, userID, conversationID uuid.UUID, archived bool) error {
	m, err := s.requireMember(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	m.IsArchived = archived
	return s.Conversations.UpdateMember(ctx, m)
}

func (s *Services) SetMuted(ctx context.Context, userID, conversationID uuid.UUID, mute bool, until *time.Time) error {
	m, err := s.requireMember(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	m.IsMuted = mute
	m.MuteUntil = until
	return s.Conversations.UpdateMember(ctx, m)
}

func (s *Services) SetPinnedConversation(ctx context.Context, userID, conversationID uuid.UUID, pinned bool) error {
	m, err := s.requireMember(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	m.IsPinned = pinned
	return s.Conversations.UpdateMember(ctx, m)
}

func (s *Services) AddMembers(ctx context.Context, actorID, conversationID uuid.UUID, memberIDs []uuid.UUID) error {
	m, err := s.requireMember(ctx, conversationID, actorID)
	if err != nil {
		return err
	}
	if m.Role != constants.MemberRoleOwner && m.Role != constants.MemberRoleAdmin {
		return errors.New("forbidden")
	}
	c, err := s.Conversations.FindByID(ctx, conversationID)
	if err != nil {
		return err
	}
	if c.Type != constants.ConversationGroup && c.Type != constants.ConversationBroadcast {
		return errors.New("cannot add members to this conversation type")
	}
	now := time.Now().UTC()
	for _, id := range uniqueUUIDs(memberIDs) {
		_ = s.Conversations.AddMember(ctx, &models.ConversationMember{
			UserID: id, ConversationID: conversationID, Role: constants.MemberRoleMember, JoinedAt: now,
		})
	}
	s.publish(ctx, constants.WSEventConversation, &conversationID, &actorID, map[string]interface{}{"action": "members_added", "memberIds": memberIDs})
	return nil
}

func (s *Services) requireMember(ctx context.Context, conversationID, userID uuid.UUID) (*models.ConversationMember, error) {
	m, err := s.Conversations.GetMember(ctx, conversationID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("forbidden")
		}
		return nil, err
	}
	return m, nil
}

func uniqueUUIDs(ids []uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]struct{}{}
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
