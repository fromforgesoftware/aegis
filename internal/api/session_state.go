package api

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

// ResourceTypeSessionState is the JSON:API type for /api/session-states.
const ResourceTypeSessionState resource.Type = "sessionStates"

// SessionStateDTO is the wire shape for live session topology; the id is the
// session id.
type SessionStateDTO struct {
	resource.RestDTO

	RAccountID      string    `jsonapi:"attr,accountId,omitempty"`
	RCurrentRealmID string    `jsonapi:"attr,currentRealmId,omitempty"`
	RCurrentShard   string    `jsonapi:"attr,currentShard,omitempty"`
	RRegion         string    `jsonapi:"attr,region,omitempty"`
	RIP             string    `jsonapi:"attr,ip,omitempty"`
	RUserAgent      string    `jsonapi:"attr,userAgent,omitempty"`
	RLastActive     time.Time `jsonapi:"attr,lastActive,omitempty"`
}

func SessionStateToDTO(s domain.SessionState) *SessionStateDTO {
	if s == nil {
		return nil
	}
	dto := &SessionStateDTO{
		RestDTO:         resource.ToRestDTO(s),
		RAccountID:      s.AccountID(),
		RCurrentRealmID: s.CurrentRealmID(),
		RCurrentShard:   s.CurrentShard(),
		RRegion:         s.Region(),
		RIP:             s.IP(),
		RUserAgent:      s.UserAgent(),
		RLastActive:     s.LastActive(),
	}
	dto.RType = ResourceTypeSessionState
	return dto
}

func SessionStateFromDTO(dto *SessionStateDTO) domain.SessionState {
	if dto == nil {
		return nil
	}
	opts := []domain.SessionStateOption{
		domain.WithSessionStateCurrentRealmID(dto.RCurrentRealmID),
		domain.WithSessionStateCurrentShard(dto.RCurrentShard),
		domain.WithSessionStateIP(dto.RIP),
		domain.WithSessionStateUserAgent(dto.RUserAgent),
	}
	if dto.RRegion != "" {
		opts = append(opts, domain.WithSessionStateRegion(dto.RRegion))
	}
	return domain.NewSessionState(dto.RID, dto.RAccountID, opts...)
}
