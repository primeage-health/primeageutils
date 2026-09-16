// Package outbox holds the transactional outbox's shape: the event as a service
// states it, the row as Mongo stores it, and the mapping between the two.
//
// The stream is one logical stream spread across per-tenant databases —
// auth.primeage.life writes user and session events into each tenant's,
// backend.primeage.life writes agency and person events into the admin domain's
// — so a relay tailing both sees one set of field names only while both sides
// store the same shape. Declared twice, that is exactly what drifts.
//
// Only the envelope is here. Each service still owns its own vocabulary of
// aggregate and event names, because each knows what it writes.
package outbox

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Event is SPEC §5.13's outbox row, whose payload carries §5.12's audit shape.
//
// It is written in the same transaction as the state change it describes, so the
// two can never diverge. Nothing consumes it yet: the audit writer and the relay
// are deferred, and these rows accumulate unpublished until they exist. That is
// deliberate — the rows *are* the audit events, so a later writer can replay the
// whole backlog and the trail retroactively covers everything since the first
// write.
//
// Which is why the payload has to be complete now. `Before` is unrecoverable
// after the fact: once the document is overwritten, its pre-image is gone. A thin
// payload today means an audit trail that can never answer what changed.
type Event struct {
	EventID       string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       AuditPayload
	CreatedAt     time.Time
	// PublishedAt stays zero until a relay fans the event out.
	PublishedAt time.Time
}

// IsPublished reports whether a relay has already fanned this event out.
func (e Event) IsPublished() bool {
	return !e.PublishedAt.IsZero()
}

// AuditPayload is SPEC §5.12's audit-event shape, captured at write time.
//
// Before and After are hand-built maps, never a reflected dump of a struct. That
// is a security property, not a style choice: a dump would silently carry any
// field later added to the entity — a refresh token, a code hash — into a
// permanent, append-only record. Each service builds its own projections; none
// of them lives here.
type AuditPayload struct {
	HouseholdID    string
	ActorID        string
	Action         string
	EntityType     string
	EntityID       string
	Before         map[string]any
	After          map[string]any
	OccurredAt     time.Time
	RequestContext RequestContext
}

// RequestContext is what the audit trail knows about the call that caused a
// change.
//
// Only fields the service actually has are present. Client IP and user agent
// would need the REST layer to inject them through the request context; they are
// left out rather than shipped as empty strings that read as "unknown" when they
// mean "never captured".
type RequestContext struct {
	Domain   string
	ActorID  string
	DeviceID string
}

// Object is the stored outbox row.
//
// The bson tags live here rather than in a service's adapter because the stored
// shape is the platform's contract with the relay, not one service's private
// encoding.
type Object struct {
	ID            primitive.ObjectID `bson:"_id"`
	EventID       string             `bson:"event_id"`
	AggregateType string             `bson:"aggregate_type"`
	AggregateID   string             `bson:"aggregate_id"`
	EventType     string             `bson:"event_type"`
	Payload       AuditPayloadObject `bson:"payload"`
	CreatedAt     time.Time          `bson:"created_at"`
	PublishedAt   time.Time          `bson:"published_at"`
}

// AuditPayloadObject is SPEC §5.12's shape as stored.
type AuditPayloadObject struct {
	HouseholdID    string               `bson:"household_id"`
	ActorID        string               `bson:"actor_id"`
	Action         string               `bson:"action"`
	EntityType     string               `bson:"entity_type"`
	EntityID       string               `bson:"entity_id"`
	Before         map[string]any       `bson:"before"`
	After          map[string]any       `bson:"after"`
	OccurredAt     time.Time            `bson:"occurred_at"`
	RequestContext RequestContextObject `bson:"request_context"`
}

// RequestContextObject is what was known about the originating call, as stored.
type RequestContextObject struct {
	Domain   string `bson:"domain"`
	ActorID  string `bson:"actor_id"`
	DeviceID string `bson:"device_id"`
}

// ObjectFromEvent maps an event to a new stored document.
func ObjectFromEvent(e Event) Object {
	return Object{
		ID:            primitive.NewObjectID(),
		EventID:       e.EventID,
		AggregateType: e.AggregateType,
		AggregateID:   e.AggregateID,
		EventType:     e.EventType,
		Payload: AuditPayloadObject{
			HouseholdID: e.Payload.HouseholdID,
			ActorID:     e.Payload.ActorID,
			Action:      e.Payload.Action,
			EntityType:  e.Payload.EntityType,
			EntityID:    e.Payload.EntityID,
			Before:      e.Payload.Before,
			After:       e.Payload.After,
			OccurredAt:  e.Payload.OccurredAt,
			RequestContext: RequestContextObject{
				Domain:   e.Payload.RequestContext.Domain,
				ActorID:  e.Payload.RequestContext.ActorID,
				DeviceID: e.Payload.RequestContext.DeviceID,
			},
		},
		CreatedAt:   e.CreatedAt,
		PublishedAt: e.PublishedAt,
	}
}

// EventFromObject maps a stored document back. Only a relay needs this today,
// but the mapping lives with its twin so the two cannot drift.
func EventFromObject(o Object) Event {
	return Event{
		EventID:       o.EventID,
		AggregateType: o.AggregateType,
		AggregateID:   o.AggregateID,
		EventType:     o.EventType,
		Payload: AuditPayload{
			HouseholdID: o.Payload.HouseholdID,
			ActorID:     o.Payload.ActorID,
			Action:      o.Payload.Action,
			EntityType:  o.Payload.EntityType,
			EntityID:    o.Payload.EntityID,
			Before:      o.Payload.Before,
			After:       o.Payload.After,
			OccurredAt:  o.Payload.OccurredAt,
			RequestContext: RequestContext{
				Domain:   o.Payload.RequestContext.Domain,
				ActorID:  o.Payload.RequestContext.ActorID,
				DeviceID: o.Payload.RequestContext.DeviceID,
			},
		},
		CreatedAt:   o.CreatedAt,
		PublishedAt: o.PublishedAt,
	}
}
