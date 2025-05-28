package model

import "time"

type UserInfo struct {
	IPv4          string    `json:"ipv4"`
	Hostname      string    `json:"hostname"`
	Url           string    `json:"url"`
	Browser       string    `json:"browser"`
	Tanggal_Ambil time.Time `json:"tanggal_ambil"`
}

type HostnameTanggal struct {
	Hostname      string    `json:"hostname" bson:"hostname"`
	Tanggal_Ambil time.Time `json:"tanggal_ambil" bson:"tanggal_ambil"`
}

type PhoneDomain struct {
	PhoneNumber      string
	Project_Hostname string
}
