package model

import "time"

type UserInfo struct {
	Hostname          string    `json:"hostname" bson:"hostname"`
	Url               string    `json:"url" bson:"url"`
	Browser           string    `json:"browser" bson:"browser"`
	Browser_Language  string    `json:"browser_language" bson:"browser_language"`
	Screen_Resolution string    `json:"screen_resolution" bson:"screen_resolution"`
	Timezone          string    `json:"timezone" bson:"timezone"`
	OnTouchStart      string    `json:"ontouchstart" bson:"ontouchstart"`
	Tanggal_Ambil     time.Time `json:"tanggal_ambil" bson:"tanggal_ambil"`
	ISP               ISP       `json:"isp" bson:"isp"`
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
	IP           string  `json:"ip" bson:"ip"`
	City         string  `json:"city,omitempty" bson:"city,omitempty"`
	Region       string  `json:"region,omitempty" bson:"region,omitempty"`
	Country_Name string  `json:"country_name" bson:"country_name"`
	Postal       string  `json:"postal,omitempty" bson:"postal,omitempty"`
	Latitude     float64 `json:"latitude,omitempty" bson:"latitude,omitempty"`
	Longitude    float64 `json:"longitude,omitempty" bson:"longitude,omitempty"`
	Timezone     string  `json:"timezone,omitempty" bson:"timezone,omitempty"`
	Asn          string  `json:"asn,omitempty" bson:"asn,omitempty"`
	Org          string  `json:"org,omitempty" bson:"org,omitempty"`
}
