package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// BimbinganWeekly represents weekly activity scores for a student
type BimbinganWeekly struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	PhoneNumber   string             `bson:"phonenumber,omitempty" json:"phonenumber,omitempty"`
	WeekNumber    int                `bson:"weeknumber" json:"weeknumber"`
	WeekLabel     string             `bson:"weeklabel" json:"weeklabel"`
	ActivityScore ActivityScore      `bson:"activityscore" json:"activityscore"`
	Approved      bool               `bson:"approved" json:"approved"`
	Asesor        Userdomyikado      `bson:"asesor,omitempty" json:"asesor,omitempty"`
	Validasi      int                `bson:"validasi,omitempty" json:"validasi,omitempty"` // rate bintang validasi
	Komentar      string             `bson:"komentar,omitempty" json:"komentar,omitempty"` // komentar dari asesor
	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}

// BimbinganWeeklyStatus tracks the current active week for bimbingan
type BimbinganWeeklyStatus struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CurrentWeek int                `bson:"currentweek" json:"currentweek"`
	WeekLabel   string             `bson:"weeklabel" json:"weeklabel"`
	StartDate   time.Time          `bson:"startdate" json:"startdate"`
	EndDate     time.Time          `bson:"enddate" json:"enddate"`
	LastUpdated time.Time          `bson:"lastupdated" json:"lastupdated"`
	UpdatedBy   string             `bson:"updatedby,omitempty" json:"updatedby,omitempty"`
}

// ChangeWeekRequest is the request structure for changing the current active week
type ChangeWeekRequest struct {
	WeekNumber int    `json:"weeknumber"`
	WeekLabel  string `json:"weeklabel"`
	UpdatedBy  string `json:"updatedby,omitempty"`
}

type BimbinganPengajuan struct {
	ID                   primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Name                 string             `bson:"name" json:"name"`
	NPM                  string             `bson:"npm" json:"npm"`
	NomorKelompok        string             `bson:"nomorkelompok" json:"nomorkelompok"`
	DosenPenguji         string             `bson:"dosenpenguji" json:"dosenpenguji"`
	DosenPengujiPhone    string             `bson:"dosenpengujiphone" json:"dosenpengujiphone"`
	DosenPembimbing      string             `bson:"dosenpembimbing" json:"dosenpembimbing"`
	DosenPembimbingPhone string             `bson:"dosenpembimbingphone" json:"dosenpembimbingphone"`
	PhoneNumber          string             `bson:"phonenumber" json:"phonenumber"`
	Timestamp            time.Time          `bson:"timestamp" json:"timestamp"`
	Status               string             `bson:"status" json:"status"` // "pending", "approved", "rejected"
}

// EventCode struct untuk menyimpan kode referral
type EventCode struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Code        string             `bson:"code" json:"code"`
	CreatedBy   string             `bson:"createdby" json:"createdby"`
	CreatedAt   time.Time          `bson:"createdat" json:"createdat"`
	IsUsed      bool               `bson:"isused" json:"isused"`
	UsedBy      string             `bson:"usedby,omitempty" json:"usedby,omitempty"`
	UsedAt      time.Time          `bson:"usedat,omitempty" json:"usedat,omitempty"`
	BimbinganID primitive.ObjectID `bson:"bimbinganid,omitempty" json:"bimbinganid,omitempty"`
}

// EventCodeTime struct untuk kode dengan waktu kadaluarsa
type EventCodeTime struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Code        string             `bson:"code" json:"code"`
	CreatedBy   string             `bson:"createdby" json:"createdby"`
	CreatedAt   time.Time          `bson:"createdat" json:"createdat"`
	ExpiresAt   time.Time          `bson:"expiresat" json:"expiresat"`
	DurationSec int                `bson:"durationsec" json:"durationsec"`
	IsActive    bool               `bson:"isactive" json:"isactive"`
}

// EventUserCodeTime struct untuk tracking user yang sudah claim kode time
type EventUserCodeTime struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CodeID      primitive.ObjectID `bson:"codeid" json:"codeid"`
	Code        string             `bson:"code" json:"code"`
	UserPhone   string             `bson:"userphone" json:"userphone"`
	ClaimedAt   time.Time          `bson:"claimedat" json:"claimedat"`
	BimbinganID primitive.ObjectID `bson:"bimbinganid,omitempty" json:"bimbinganid,omitempty"`
}

// TimeCodeGenerateRequest struct untuk request generate time code
type TimeCodeGenerateRequest struct {
	DurationSeconds int `json:"duration_seconds" bson:"duration_seconds"`
}

// TimeCodeClaimRequest struct untuk request claim time code
type TimeCodeClaimRequest struct {
	Code string `json:"code" bson:"code"`
}

// Event struct untuk menyimpan event yang dibuat owner
type Event struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description" json:"description"`
	Points      int                `bson:"points" json:"points"`
	CreatedBy   string             `bson:"createdby" json:"createdby"`
	CreatedAt   time.Time          `bson:"createdat" json:"createdat"`
	IsActive    bool               `bson:"isactive" json:"isactive"`
}

// EventClaim struct untuk tracking user yang claim event
type EventClaim struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	EventID     primitive.ObjectID `bson:"eventid" json:"eventid"`
	UserPhone   string             `bson:"userphone" json:"userphone"`
	ClaimedAt   time.Time          `bson:"claimedat" json:"claimedat"`
	Deadline    time.Time          `bson:"deadline" json:"deadline"`
	Status      string             `bson:"status" json:"status"` // "claimed", "submitted", "approved", "expired"
	TaskLink    string             `bson:"tasklink,omitempty" json:"tasklink,omitempty"`
	SubmittedAt time.Time          `bson:"submittedat,omitempty" json:"submittedat,omitempty"`
	ApprovedAt  time.Time          `bson:"approvedat,omitempty" json:"approvedat,omitempty"`
	ApprovedBy  string             `bson:"approvedby,omitempty" json:"approvedby,omitempty"`
}

// EventCreateRequest struct untuk request create event
type EventCreateRequest struct {
	Name        string `json:"name" bson:"name"`
	Description string `json:"description" bson:"description"`
	Points      int    `json:"points" bson:"points"`
}

// EventClaimRequest struct untuk request claim event
type EventClaimRequest struct {
	EventID         string `json:"event_id" bson:"event_id"`
	DeadlineSeconds int    `json:"deadline_seconds" bson:"deadline_seconds"`
}

// EventSubmitRequest struct untuk submit task link
type EventSubmitRequest struct {
	ClaimID  string `json:"claim_id" bson:"claim_id"`
	TaskLink string `json:"task_link" bson:"task_link"`
}

// EventApproveRequest struct untuk approve event claim
type EventApproveRequest struct {
	ClaimID string `json:"claim_id" bson:"claim_id"`
}

// TimeCodeResponse struct untuk response generate time code
type TimeCodeResponse struct {
	Code      string `json:"code"`
	ExpiresAt string `json:"expires_at"`
	Duration  int    `json:"duration"`
}
