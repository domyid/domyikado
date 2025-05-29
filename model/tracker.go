package model

import "time"

type UserInfo struct {
	Hostname          string    `json:"hostname"`
	Url               string    `json:"url"`
	Browser           string    `json:"browser"`
	Browser_Language  string    `json:"browser_language"`
	Screen_Resolution string    `json:"screen_resolution"`
	Timezone          string    `json:"timezone"`
	Ontouchstart      string    `json:"ontouchstart"`
	Tanggal_Ambil     time.Time `json:"tanggal_ambil"`
	ISP               time.Time `json:"isp"`
}

type HostnameTanggal struct {
	Hostname      string    `json:"hostname" bson:"hostname"`
	Tanggal_Ambil time.Time `json:"tanggal_ambil" bson:"tanggal_ambil"`
}

type PhoneDomain struct {
	PhoneNumber      string
	Project_Hostname string
}

type ISP struct {
	IP           string    `json:"ip" bson:"ip"`
	City         time.Time `json:"city" bson:"city"`
	Region       time.Time `json:"region" bson:"region"`
	Country_Name time.Time `json:"country_name" bson:"country_name"`
	Postal       time.Time `json:"postal" bson:"postal"`
	Latitude     time.Time `json:"latitude" bson:"latitude"`
	Longitude    time.Time `json:"longitude" bson:"longitude"`
	Timezone     time.Time `json:"timezone" bson:"timezone"`
	Org          time.Time `json:"org" bson:"org"`
}
