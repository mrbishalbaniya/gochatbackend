package services

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/pulse/chat-service/internal/config"
	"github.com/pulse/chat-service/internal/call/constants"
	"github.com/pulse/chat-service/internal/call/dto"
	"github.com/pulse/chat-service/internal/call/events"
	"github.com/pulse/chat-service/internal/call/models"
	"github.com/pulse/chat-service/internal/call/turn"
	"github.com/pulse/chat-service/internal/call/webrtc"
	chatmodels "github.com/pulse/chat-service/internal/models"
	"github.com/redis/go-redis/v9"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Publisher interface {
	Publish(ctx context.Context, evt events.Event) error
}

type Services struct {
	Cfg       *config.Config
	DB        *gorm.DB
	RDB       *redis.Client
	Publisher Publisher
}

func New(cfg *config.Config, db *gorm.DB, rdb *redis.Client, pub Publisher) *Services {
	return &Services{Cfg: cfg, DB: db, RDB: rdb, Publisher: pub}
}

func (s *Services) publish(ctx context.Context, typ string, callID, userID *uuid.UUID, payload interface{}) {
	if s.Publisher == nil {
		return
	}
	evt, err := events.New(typ, callID, userID, payload)
	if err != nil {
		return
	}
	_ = s.Publisher.Publish(ctx, evt)
}

func (s *Services) logEvent(ctx context.Context, callID uuid.UUID, userID *uuid.UUID, typ string, payload interface{}) {
	raw, _ := json.Marshal(payload)
	_ = s.DB.WithContext(ctx).Create(&models.CallEvent{
		CallID: callID, UserID: userID, Type: typ, Payload: datatypes.JSON(raw),
	}).Error
	_ = s.DB.WithContext(ctx).Create(&models.CallLog{
		CallID: callID, UserID: userID, Event: typ, Payload: datatypes.JSON(raw),
	}).Error
}

func (s *Services) EnsureUser(ctx context.Context, externalID uuid.UUID, name string) error {
	var u models.User
	err := s.DB.WithContext(ctx).Where("external_id = ?", externalID).First(&u).Error
	if err == gorm.ErrRecordNotFound {
		return s.DB.WithContext(ctx).Create(&models.User{ExternalID: externalID, DisplayName: name}).Error
	}
	return err
}

func (s *Services) IsBlocked(ctx context.Context, a, b uuid.UUID) (bool, error) {
	var n int64
	err := s.DB.WithContext(ctx).Model(&models.BlockedUser{}).
		Where("(blocker_id = ? AND blocked_id = ?) OR (blocker_id = ? AND blocked_id = ?)", a, b, b, a).
		Count(&n).Error
	return n > 0, err
}

type CallView struct {
	Call         models.Call          `json:"call"`
	Participants []models.Participant `json:"participants"`
}

func (s *Services) StartCall(ctx context.Context, initiator uuid.UUID, req dto.StartCallRequest) (*CallView, error) {
	if req.CallType != constants.CallTypeAudio && req.CallType != constants.CallTypeVideo {
		return nil, errors.New("invalid call type")
	}
	ids := unique(req.ParticipantIDs)
	if len(ids) == 0 {
		return nil, errors.New("participants required")
	}
	for _, id := range ids {
		if id == initiator {
			continue
		}
		blocked, err := s.IsBlocked(ctx, initiator, id)
		if err != nil {
			return nil, err
		}
		if blocked {
			return nil, errors.New("cannot call blocked user")
		}
	}
	_ = s.EnsureUser(ctx, initiator, "")

	call := &models.Call{
		ConversationID:  req.ConversationID,
		InitiatorID:     initiator,
		CallType:        req.CallType,
		Status:          constants.CallStatusRinging,
		MaxParticipants: s.Cfg.MaxParticipants,
		IsGroup:         req.IsGroup || len(ids) > 1,
	}
	now := time.Now().UTC()
	parts := []models.Participant{{
		UserID: initiator, Role: constants.RoleHost, Status: constants.ParticipantJoined,
		JoinedAt: &now, CameraOn: req.CallType == constants.CallTypeVideo,
	}}
	for _, id := range ids {
		if id == initiator {
			continue
		}
		parts = append(parts, models.Participant{
			UserID: id, Role: constants.RoleParticipant, Status: constants.ParticipantRinging,
		})
	}
	if len(parts) > s.Cfg.MaxParticipants {
		return nil, errors.New("too many participants")
	}

	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(call).Error; err != nil {
			return err
		}
		for i := range parts {
			parts[i].CallID = call.ID
			if err := tx.Create(&parts[i]).Error; err != nil {
				return err
			}
			if parts[i].UserID != initiator {
				_ = tx.Create(&models.CallInvitation{
					CallID: call.ID, InviterID: initiator, InviteeID: parts[i].UserID, Status: "pending",
				}).Error
				_ = tx.Create(&models.CallNotification{
					UserID: parts[i].UserID, CallID: call.ID, Type: "incoming_call",
					Title: "Incoming call", Body: string(req.CallType), Status: "pending",
				}).Error
				payload, _ := json.Marshal(map[string]interface{}{
					"callId": call.ID.String(), "callType": req.CallType, "url": "/inbox",
				})
				_ = tx.Create(&chatmodels.NotificationQueue{
					UserID: parts[i].UserID, Type: "incoming_call", Title: "Incoming call",
					Body: req.CallType + " call", Payload: datatypes.JSON(payload), Status: "pending",
				}).Error
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	view := &CallView{Call: *call, Participants: parts}
	s.logEvent(ctx, call.ID, &initiator, "call_started", view)
	s.publish(ctx, constants.WSCallInvite, &call.ID, &initiator, view)
	return view, nil
}

func (s *Services) GetCall(ctx context.Context, userID, callID uuid.UUID) (*CallView, error) {
	if err := s.requireParticipant(ctx, callID, userID); err != nil {
		return nil, err
	}
	var call models.Call
	if err := s.DB.WithContext(ctx).First(&call, "id = ?", callID).Error; err != nil {
		return nil, err
	}
	var parts []models.Participant
	_ = s.DB.WithContext(ctx).Where("call_id = ?", callID).Find(&parts).Error
	return &CallView{Call: call, Participants: parts}, nil
}

func (s *Services) AcceptCall(ctx context.Context, userID, callID uuid.UUID) (*CallView, error) {
	p, err := s.getParticipant(ctx, callID, userID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	p.Status = constants.ParticipantJoined
	p.JoinedAt = &now
	if err := s.DB.WithContext(ctx).Save(p).Error; err != nil {
		return nil, err
	}
	_ = s.DB.WithContext(ctx).Model(&models.Call{}).Where("id = ? AND status = ?", callID, constants.CallStatusRinging).
		Updates(map[string]interface{}{"status": constants.CallStatusActive, "started_at": now}).Error
	view, _ := s.GetCall(ctx, userID, callID)
	s.publish(ctx, constants.WSAccept, &callID, &userID, view)
	s.publish(ctx, constants.WSParticipantUpdate, &callID, &userID, p)
	return view, nil
}

func (s *Services) RejectCall(ctx context.Context, userID, callID uuid.UUID) error {
	p, err := s.getParticipant(ctx, callID, userID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	p.Status = constants.ParticipantDeclined
	p.LeftAt = &now
	_ = s.DB.WithContext(ctx).Save(p).Error
	var call models.Call
	_ = s.DB.WithContext(ctx).First(&call, "id = ?", callID).Error
	if !call.IsGroup {
		_ = s.DB.WithContext(ctx).Model(&call).Updates(map[string]interface{}{
			"status": constants.CallStatusRejected, "ended_at": now,
		}).Error
	}
	s.publish(ctx, constants.WSReject, &callID, &userID, map[string]interface{}{"userId": userID})
	return nil
}

func (s *Services) EndCall(ctx context.Context, userID, callID uuid.UUID) (*CallView, error) {
	if err := s.requireParticipant(ctx, callID, userID); err != nil {
		return nil, err
	}
	var call models.Call
	if err := s.DB.WithContext(ctx).First(&call, "id = ?", callID).Error; err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	call.Status = constants.CallStatusEnded
	call.EndedAt = &now
	if call.StartedAt != nil {
		call.DurationSec = int(now.Sub(*call.StartedAt).Seconds())
	}
	_ = s.DB.WithContext(ctx).Save(&call).Error
	_ = s.DB.WithContext(ctx).Model(&models.Participant{}).Where("call_id = ? AND left_at IS NULL", callID).
		Updates(map[string]interface{}{"status": constants.ParticipantLeft, "left_at": now}).Error

	// Missed for ringing participants
	var ringing []models.Participant
	_ = s.DB.WithContext(ctx).Where("call_id = ? AND status = ?", callID, constants.ParticipantRinging).Find(&ringing).Error
	for _, p := range ringing {
		_ = s.DB.WithContext(ctx).Create(&models.MissedCall{CallID: callID, UserID: p.UserID, FromUser: call.InitiatorID}).Error
		_ = s.DB.WithContext(ctx).Model(&models.Call{}).Where("id = ? AND started_at IS NULL", callID).
			Update("status", constants.CallStatusMissed).Error
	}

	view := &CallView{Call: call}
	_ = s.DB.WithContext(ctx).Where("call_id = ?", callID).Find(&view.Participants).Error
	s.publish(ctx, constants.WSCallEnd, &callID, &userID, view)
	s.logEvent(ctx, callID, &userID, "call_ended", view)
	return view, nil
}

func (s *Services) UpdateMediaState(ctx context.Context, userID, callID uuid.UUID, muted, camera, screen *bool, raiseHand *bool) (*models.Participant, error) {
	p, err := s.getParticipant(ctx, callID, userID)
	if err != nil {
		return nil, err
	}
	if muted != nil {
		p.Muted = *muted
	}
	if camera != nil {
		p.CameraOn = *camera
	}
	if screen != nil {
		p.ScreenShare = *screen
		if *screen {
			_ = s.DB.WithContext(ctx).Create(&models.ScreenSharing{
				CallID: callID, UserID: userID, ShareType: "screen", StartedAt: time.Now().UTC(), Active: true,
			}).Error
		} else {
			_ = s.DB.WithContext(ctx).Model(&models.ScreenSharing{}).
				Where("call_id = ? AND user_id = ? AND active = true", callID, userID).
				Updates(map[string]interface{}{"active": false, "ended_at": time.Now().UTC()}).Error
		}
	}
	if raiseHand != nil {
		p.RaisedHand = *raiseHand
	}
	if err := s.DB.WithContext(ctx).Save(p).Error; err != nil {
		return nil, err
	}
	s.publish(ctx, constants.WSParticipantUpdate, &callID, &userID, p)
	return p, nil
}

func (s *Services) SaveOffer(ctx context.Context, from, to, callID uuid.UUID, sdp string) error {
	if !webrtc.ValidateSDP(sdp) {
		return errors.New("invalid sdp")
	}
	if err := s.requireParticipant(ctx, callID, from); err != nil {
		return err
	}
	o := &models.CallOffer{CallID: callID, FromUser: from, ToUser: to, SDP: sdp}
	if err := s.DB.WithContext(ctx).Create(o).Error; err != nil {
		return err
	}
	s.publish(ctx, constants.WSOffer, &callID, &from, map[string]interface{}{
		"from": from, "to": to, "sdp": sdp,
	})
	return nil
}

func (s *Services) SaveAnswer(ctx context.Context, from, to, callID uuid.UUID, sdp string) error {
	if !webrtc.ValidateSDP(sdp) {
		return errors.New("invalid sdp")
	}
	if err := s.requireParticipant(ctx, callID, from); err != nil {
		return err
	}
	a := &models.CallAnswer{CallID: callID, FromUser: from, ToUser: to, SDP: sdp}
	if err := s.DB.WithContext(ctx).Create(a).Error; err != nil {
		return err
	}
	s.publish(ctx, constants.WSAnswer, &callID, &from, map[string]interface{}{
		"from": from, "to": to, "sdp": sdp,
	})
	return nil
}

func (s *Services) SaveICE(ctx context.Context, userID, callID uuid.UUID, candidate, sdpMid string, mline int, to *uuid.UUID) error {
	if err := s.requireParticipant(ctx, callID, userID); err != nil {
		return err
	}
	ice := &models.ICECandidate{CallID: callID, UserID: userID, Candidate: candidate, SDPMid: sdpMid, SDPMLineIndex: mline}
	_ = s.DB.WithContext(ctx).Create(ice).Error
	payload := map[string]interface{}{
		"from": userID, "candidate": candidate, "sdpMid": sdpMid, "sdpMLineIndex": mline,
	}
	if to != nil {
		payload["to"] = *to
	}
	s.publish(ctx, constants.WSIceCandidate, &callID, &userID, payload)
	return nil
}

func (s *Services) ReportQuality(ctx context.Context, userID, callID uuid.UUID, q dto.QualityReport) error {
	if err := s.requireParticipant(ctx, callID, userID); err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Create(&models.CallQuality{
		CallID: callID, UserID: userID, LatencyMs: q.LatencyMs, JitterMs: q.JitterMs,
		PacketLoss: q.PacketLoss, BandwidthKbps: q.BandwidthKbps, ConnectionType: q.ConnectionType,
		SignalStrength: q.SignalStrength,
	}).Error
}

func (s *Services) ListMissed(ctx context.Context, userID uuid.UUID) ([]models.MissedCall, error) {
	var items []models.MissedCall
	err := s.DB.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Limit(50).Find(&items).Error
	return items, err
}

func (s *Services) ListHistory(ctx context.Context, userID uuid.UUID) ([]models.Call, error) {
	var calls []models.Call
	err := s.DB.WithContext(ctx).
		Joins("JOIN pulse_call_participants p ON p.call_id = pulse_calls.id AND p.user_id = ? AND p.deleted_at IS NULL", userID).
		Order("pulse_calls.created_at DESC").Limit(50).Find(&calls).Error
	return calls, err
}

func (s *Services) ICEConfig() dto.ICEServersResponse {
	return turn.ICEServers(s.Cfg)
}

func (s *Services) GetOrCreateSettings(ctx context.Context, userID uuid.UUID) (*models.CallSettings, error) {
	var st models.CallSettings
	err := s.DB.WithContext(ctx).Where("user_id = ?", userID).First(&st).Error
	if err == gorm.ErrRecordNotFound {
		st = models.CallSettings{
			UserID: userID, DefaultCamera: "user", NoiseSuppression: true,
			EchoCancellation: true, AutoGainControl: true, MirrorLocalVideo: true, MaxBitrateKbps: 1500,
		}
		if err := s.DB.WithContext(ctx).Create(&st).Error; err != nil {
			return nil, err
		}
		return &st, nil
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *Services) UpdateSettings(ctx context.Context, userID uuid.UUID, req dto.SettingsRequest) (*models.CallSettings, error) {
	st, err := s.GetOrCreateSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	if req.DefaultCamera != nil {
		st.DefaultCamera = *req.DefaultCamera
	}
	if req.NoiseSuppression != nil {
		st.NoiseSuppression = *req.NoiseSuppression
	}
	if req.EchoCancellation != nil {
		st.EchoCancellation = *req.EchoCancellation
	}
	if req.AutoGainControl != nil {
		st.AutoGainControl = *req.AutoGainControl
	}
	if req.MirrorLocalVideo != nil {
		st.MirrorLocalVideo = *req.MirrorLocalVideo
	}
	if req.MaxBitrateKbps != nil {
		st.MaxBitrateKbps = *req.MaxBitrateKbps
	}
	if err := s.DB.WithContext(ctx).Save(st).Error; err != nil {
		return nil, err
	}
	return st, nil
}

func (s *Services) Invite(ctx context.Context, hostID, callID uuid.UUID, userIDs []uuid.UUID) error {
	p, err := s.getParticipant(ctx, callID, hostID)
	if err != nil {
		return err
	}
	if p.Role != constants.RoleHost {
		return errors.New("only host can invite")
	}
	var count int64
	_ = s.DB.WithContext(ctx).Model(&models.Participant{}).Where("call_id = ? AND left_at IS NULL", callID).Count(&count)
	for _, id := range unique(userIDs) {
		if int(count)+1 > s.Cfg.MaxParticipants {
			return errors.New("participant limit reached")
		}
		_ = s.DB.WithContext(ctx).Create(&models.Participant{
			CallID: callID, UserID: id, Role: constants.RoleParticipant, Status: constants.ParticipantRinging,
		}).Error
		count++
		s.publish(ctx, constants.WSCallInvite, &callID, &hostID, map[string]interface{}{"inviteeId": id, "callId": callID})
	}
	return nil
}

func (s *Services) requireParticipant(ctx context.Context, callID, userID uuid.UUID) error {
	_, err := s.getParticipant(ctx, callID, userID)
	return err
}

func (s *Services) getParticipant(ctx context.Context, callID, userID uuid.UUID) (*models.Participant, error) {
	var p models.Participant
	err := s.DB.WithContext(ctx).Where("call_id = ? AND user_id = ?", callID, userID).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("forbidden")
		}
		return nil, err
	}
	return &p, nil
}

func (s *Services) GetParticipant(ctx context.Context, callID, userID uuid.UUID) (*models.Participant, error) {
	return s.getParticipant(ctx, callID, userID)
}

func unique(ids []uuid.UUID) []uuid.UUID {
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
