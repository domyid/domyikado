package model

import (
    "time"
    "go.mongodb.org/mongo-driver/bson/primitive"
)

type GTMetrixInfo struct {
    ID                 primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
    Name               string             `json:"name" bson:"name"`
    PhoneNumber        string             `json:"phonenumber" bson:"phonenumber"`
    Cycle              int                `json:"cycle" bson:"cycle"`
    Hostname           string             `json:"hostname" bson:"hostname"`
    IP                 string             `json:"ip" bson:"ip"`
    Screenshots        int                `json:"screenshots" bson:"screenshots"`
    Pekerjaan          string             `json:"pekerjaan" bson:"pekerjaan"`
    Token              string             `json:"token" bson:"token"`
    URLPekerjaan       string             `json:"urlpekerjaan" bson:"urlpekerjaan"`
    WaGroupID          string             `json:"wagroupid" bson:"wagroupid"`
    GTMetrixURLTarget  string             `json:"gtmetrix_url_target" bson:"gtmetrix_url_target"`
    GTMetrixGrade      string             `json:"gtmetrix_grade" bson:"gtmetrix_grade"`
    GTMetrixPerformance string            `json:"gtmetrix_performance" bson:"gtmetrix_performance"`
    GTMetrixStructure  string             `json:"gtmetrix_structure" bson:"gtmetrix_structure"`
    LCP                string             `json:"lcp" bson:"lcp"`
    TBT                string             `json:"tbt" bson:"tbt"`
    CLS                string             `json:"cls" bson:"cls"`
    CreatedAt          time.Time          `json:"createdAt" bson:"createdAt"`
    Points             float64            `json:"points,omitempty" bson:"points,omitempty"`
}