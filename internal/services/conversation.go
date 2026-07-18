package services

import (
	"context"
	"errors"
	"strings"
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
	Peers        []dto.UserDTO               `json:"peers,omitempty"`
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
			return s.GetConversation(ctx, userID, existing.ID)
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
	view, err := s.GetConversation(ctx, userID, conv.ID)
	if err != nil {
		return nil, err
	}
	s.publish(ctx, constants.WSEventConversation, &conv.ID, &userID, view)
	return view, nil
}

func (s *Services) ListConversations(ctx context.Context, userID uuid.UUID, archived bool) ([]ConversationView, error) {
	convs, memberMap, err := s.Conversations.ListForUser(ctx, userID, archived, 100)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(convs))
	for _, c := range convs {
		ids = append(ids, c.ID)
	}
	allMembers, _ := s.Conversations.ListMembersForConversations(ctx, ids)
	membersByConv := map[uuid.UUID][]models.ConversationMember{}
	userIDs := make([]uuid.UUID, 0, len(allMembers))
	seenUsers := map[uuid.UUID]struct{}{}
	for _, m := range allMembers {
		membersByConv[m.ConversationID] = append(membersByConv[m.ConversationID], m)
		if _, ok := seenUsers[m.UserID]; !ok {
			seenUsers[m.UserID] = struct{}{}
			userIDs = append(userIDs, m.UserID)
		}
	}
	users, _ := s.Users.FindByIDs(ctx, userIDs)
	userMap := map[uuid.UUID]models.User{}
	for i := range users {
		userMap[users[i].ID] = users[i]
	}

	out := make([]ConversationView, 0, len(convs))
	for _, c := range convs {
		members := membersByConv[c.ID]
		view := ConversationView{
			Conversation: c,
			Membership:   memberMap[c.ID],
			Members:      members,
			Peers:        peersFromMembers(members, userID, userMap),
		}
		applyDisplayTitle(&view, userID)
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
	userIDs := make([]uuid.UUID, 0, len(members))
	for _, mem := range members {
		userIDs = append(userIDs, mem.UserID)
	}
	users, _ := s.Users.FindByIDs(ctx, userIDs)
	userMap := map[uuid.UUID]models.User{}
	for i := range users {
		userMap[users[i].ID] = users[i]
	}
	view := &ConversationView{
		Conversation: *c,
		Membership:   *m,
		Members:      members,
		Peers:        peersFromMembers(members, userID, userMap),
	}
	applyDisplayTitle(view, userID)
	if lm, err := s.Messages.Latest(ctx, conversationID); err == nil {
		view.LastMessage = lm
	}
	return view, nil
}

func peersFromMembers(members []models.ConversationMember, me uuid.UUID, users map[uuid.UUID]models.User) []dto.UserDTO {
	out := make([]dto.UserDTO, 0, len(members))
	for _, m := range members {
		if m.UserID == me {
			continue
		}
		if u, ok := users[m.UserID]; ok {
			out = append(out, toUserDTO(&u))
		}
	}
	return out
}

// applyDisplayTitle fills empty / default titles using peer display names for the API response.
func applyDisplayTitle(view *ConversationView, me uuid.UUID) {
	title := strings.TrimSpace(view.Conversation.Title)
	switch view.Conversation.Type {
	case constants.ConversationDirect:
		if len(view.Peers) > 0 {
			peer := view.Peers[0]
			name := strings.TrimSpace(peer.DisplayName)
			if name == "" {
				name = peer.Username
			}
			if name != "" {
				view.Conversation.Title = name
			}
			if view.Conversation.AvatarURL == "" && peer.AvatarURL != "" {
				view.Conversation.AvatarURL = peer.AvatarURL
			}
		} else if title == "" {
			view.Conversation.Title = "Direct chat"
		}
	case constants.ConversationGroup, constants.ConversationBroadcast:
		if title == "" || strings.EqualFold(title, "New group") {
			names := make([]string, 0, len(view.Peers))
			for _, p := range view.Peers {
				n := strings.TrimSpace(p.DisplayName)
				if n == "" {
					n = p.Username
				}
				if n != "" {
					names = append(names, n)
				}
			}
			if len(names) > 0 {
				view.Conversation.Title = strings.Join(names, ", ")
			} else if title == "" {
				view.Conversation.Title = "Group"
			}
		}
	default:
		_ = me
		if title == "" {
			view.Conversation.Title = "Chat"
		}
	}
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
