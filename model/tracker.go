package model

import "go.mongodb.org/mongo-driver/bson/primitive"

type UserInfo struct {
	IPv4          string             `json:"ipv4"`
	Hostname      string             `json:"hostname"`
	Browser       string             `json:"browser"`
	Tanggal_Ambil primitive.DateTime `json:"tanggal_ambil"`
}
