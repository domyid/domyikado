package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PushReport struct {
	ProjectName string        `bson:"projectname" json:"projectname"`
	Project     Project       `bson:"project" json:"project"`
	User        Userdomyikado `bson:"user,omitempty" json:"user,omitempty"`
	Username    string        `bson:"username" json:"username"`
	Email       string        `bson:"email,omitempty" json:"email,omitempty"`
	Repo        string        `bson:"repo" json:"repo"`
	Ref         string        `bson:"ref" json:"ref"`
	Message     string        `bson:"message" json:"message"`
	Modified    string        `bson:"modified,omitempty" json:"modified,omitempty"`
	RemoteAddr  string        `bson:"remoteaddr,omitempty" json:"remoteaddr,omitempty"`
}

type MasterEnrool struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Kode      string             `bson:"kode,omitempty" json:"kode,omitempty"`
	Nama      string             `bson:"nama,omitempty" json:"nama,omitempty"`
	Deskripsi string             `bson:"deskripsi,omitempty" json:"deskripsi,omitempty"`
}

type Project struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Enroll       string             `bson:"enroll,omitempty" json:"enroll,omitempty"` //kelas atau proyek atau bimbingan
	MasterEnrool MasterEnrool       `bson:"masterenroll,omitempty" json:"masterenroll,omitempty"`
	Secret       string             `bson:"secret" json:"secret"`
	GithubToken  string             `bson:"githubtoken" json:"githubtoken"`
	Name         string             `bson:"name" json:"name"`
	Description  string             `bson:"description" json:"description"`
	Owner        Userdomyikado      `bson:"owner" json:"owner"`
	WAGroupID    string             `bson:"wagroupid,omitempty" json:"wagroupid,omitempty"`
	RepoOrg      string             `bson:"repoorg,omitempty" json:"repoorg,omitempty"`
	RepoLogName  string             `bson:"repologname,omitempty" json:"repologname,omitempty"`
	Members      []Userdomyikado    `bson:"members,omitempty" json:"members,omitempty"`
	Closed       bool               `bson:"closed,omitempty" json:"closed,omitempty"`
	Pembimbing   []Userdomyikado    `bson:"pembimbing,omitempty" json:"pembimbing,omitempty"`
}

type Userdomyikado struct {
	ID                   primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Name                 string             `bson:"name,omitempty" json:"name,omitempty"`
	PhoneNumber          string             `bson:"phonenumber,omitempty" json:"phonenumber,omitempty"`
	Email                string             `bson:"email,omitempty" json:"email,omitempty"`
	GithubUsername       string             `bson:"githubusername,omitempty" json:"githubusername,omitempty"`
	GitlabUsername       string             `bson:"gitlabusername,omitempty" json:"gitlabusername,omitempty"`
	GitHostUsername      string             `bson:"githostusername,omitempty" json:"githostusername,omitempty"`
	Poin                 float64            `bson:"poin,omitempty" json:"poin,omitempty"`
	GoogleProfilePicture string             `bson:"googleprofilepicture,omitempty" json:"picture,omitempty"`
	Team                 string             `json:"team,omitempty" bson:"team,omitempty"`
	Scope                string             `json:"scope,omitempty" bson:"scope,omitempty"`
	Section              string             `json:"section,omitempty" bson:"section,omitempty"`
	Chapter              string             `json:"chapter,omitempty" bson:"chapter,omitempty"`
	LinkedDevice         string             `json:"linkeddevice,omitempty" bson:"linkeddevice,omitempty"`
	JumlahAntrian        int                `json:"jumlahantrian,omitempty" bson:"jumlahantrian,omitempty"`
	SponsorName          string             `json:"sponsorname,omitempty" bson:"sponsorname,omitempty"`
	SponsorPhoneNumber   string             `json:"sponsorphonenumber,omitempty" bson:"sponsorphonenumber,omitempty"`
	StravaProfilePicture string             `json:"stravaprofilepicture,omitempty" bson:"stravaprofilepicture,omitempty"`
	AthleteId            string             `json:"athleteid,omitempty" bson:"athleteid,omitempty"`
	NPM                  string             `json:"npm,omitempty" bson:"npm,omitempty"`
	Wonpaywallet         string             `json:"wonpaywallet,omitempty" bson:"wonpaywallet,omitempty"`
	RVNwallet            string             `json:"rvnwallet,omitempty" bson:"rvnwallet,omitempty"`
	WeeklyScore          []ActivityScore    `json:"weeklyscore,omitempty" bson:"weeklyscore,omitempty"` // aktifitas mingguan
	IsDosen              bool               `json:"isdosen,omitempty" bson:"isdosen,omitempty"`
}

// skor asessment proyek1 dan lainnya aktifitas mingguan. ini pengganti kartu bimbingan
type ActivityScore struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CreatedAt       time.Time          `bson:"createdAt"`                                          //kalo lebih dari seminggu auto hapus
	BimbinganKe     int                `bson:"bimbinganke,omitempty" json:"bimbinganke,omitempty"` //bimbingan ke berapa
	PhoneNumber     string             `bson:"phonenumber,omitempty" json:"phonenumber,omitempty"`
	Enroll          MasterEnrool       `bson:"enroll,omitempty" json:"enroll,omitempty"` //kelas atau proyek atau bimbingan
	Sponsordata     int                `bson:"sponsordata,omitempty" json:"sponsordata,omitempty"`
	Sponsor         int                `bson:"sponsor,omitempty" json:"sponsor,omitempty"` // lengkap 100, nomor 50, nama 50
	StravaKM        float32            `bson:"stravakm,omitempty" json:"stravakm,omitempty"`
	Strava          int                `bson:"strava,omitempty" json:"strava,omitempty"` //perminggu dibagi 6KM dikali 100
	IQresult        int                `bson:"iqresult,omitempty" json:"iqresult,omitempty"`
	IQ              int                `bson:"iq,omitempty" json:"iq,omitempty"`
	Pomokitsesi     int                `bson:"pomokitsesi,omitempty" json:"pomokitsesi,omitempty"`
	Pomokit         int                `bson:"pomokit,omitempty" json:"pomokit,omitempty"`                 //20 per cycle
	MBC             float32            `bson:"mbc,omitempty" json:"mbc,omitempty"`                         //jumlah total mbc
	MBCPoints       float64            `bson:"mbcPoints,omitempty" json:"mbcPoints,omitempty"`             //points for MBC contributions
	RVN             float32            `bson:"rvn,omitempty" json:"rvn,omitempty"`                         //jumlah total rvn
	RavencoinPoints float64            `bson:"ravencoinPoints,omitempty" json:"ravencoinPoints,omitempty"` //points for Ravencoin contributions
	BlockChain      int                `bson:"blockchain,omitempty" json:"blockchain,omitempty"`           // dibagi rata2 kelas dikali 100
	Rupiah          int                `bson:"rupiah,omitempty" json:"rupiah,omitempty"`                   //total nilai rupiah yang disetorkan
	QRIS            int                `bson:"qris,omitempty" json:"qris,omitempty"`                       // dibagi rata2 kelas dikali 100
	QRISPoints      float64            `bson:"qrisPoints,omitempty" json:"qrisPoints,omitempty"`           //points for QRIS contributions
	Trackerdata     int                `bson:"trackerdata,omitempty" json:"trackerdata,omitempty"`         //jumlah total visitor
	Tracker         float64            `bson:"tracker,omitempty" json:"tracker,omitempty"`                 //rata2 10 unique visitor sehari 100
	BukuKatalog     string             `bson:"bukukatalog,omitempty" json:"bukukatalog,omitempty"`         //url katalog buku
	BukPed          int                `bson:"bukped,omitempty" json:"bukped,omitempty"`                   //upload 25;approve 50;resi 75;deposit 100
	JurnalWeb       string             `bson:"jurnalweb,omitempty" json:"jurnalweb,omitempty"`             //Alamat web jurnal
	Jurnal          int                `bson:"jurnal,omitempty" json:"jurnal,omitempty"`                   //score jurnal
	GTMetrixResult  string             `bson:"gtmetrixresult,omitempty" json:"gtmetrixresult,omitempty"`   //detaul score gtmetrix
	GTMetrix        int                `bson:"gtmetrix,omitempty" json:"gtmetrix,omitempty"`               //A 100;B 75;C 50;D 25; E 0
	WebHookpush     int                `bson:"webhookpush,omitempty" json:"webhookpush,omitempty"`         // jumlah push ke webhook
	WebHook         int                `bson:"webhook,omitempty" json:"webhook,omitempty"`                 //maksimal 100 dari push github diambil dari seminggu terakhir
	PresensiHari    int                `bson:"presensihari,omitempty" json:"presensihari,omitempty"`       //jumlah ari presensi
	Presensi        int                `bson:"presensi,omitempty" json:"presensi,omitempty"`               //5*lengkap masuk dan pulang = 100
	TotalScore      int                `bson:"total,omitempty" json:"total,omitempty"`
	Approved        bool               `bson:"approved" json:"approved"`
	Asesor          Userdomyikado      `bson:"asesor,omitempty" json:"asesor,omitempty"`
	Validasi        int                `bson:"validasi,omitempty" json:"validasi,omitempty"` // rate bintang validasi
	Komentar        string             `bson:"komentar,omitempty" json:"komentar,omitempty"` //komentar dari asesor
}

type Task struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	ProjectID string             `bson:"projectid" json:"projectid"`
	Name      string             `bson:"name" json:"name"`
	PIC       Userdomyikado      `bson:"pic" json:"pic"`
	Done      bool               `bson:"done,omitempty" json:"done,omitempty"`
}

type ReportData struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	UserID      primitive.ObjectID `bson:"userid,omitempty" json:"userid,omitempty"`
	Name        string             `bson:"name,omitempty" json:"name,omitempty"`
	PhoneNumber string             `bson:"phonenumber,omitempty" json:"phonenumber,omitempty"`
	Email       string             `bson:"email,omitempty" json:"email,omitempty"`
	ProjectID   primitive.ObjectID `bson:"projectid,omitempty" json:"projectid,omitempty"`
	ProjectName string             `bson:"projectname,omitempty" json:"projectname,omitempty"`
	Poin        float64            `bson:"poin,omitempty" json:"poin,omitempty"`
	Activity    string             `bson:"activity,omitempty" json:"activity,omitempty"`
	Detail      string             `bson:"detail,omitempty" json:"detail,omitempty"`
	Info        string             `bson:"info,omitempty" json:"info,omitempty"`
	URL         string             `bson:"url,omitempty" json:"url,omitempty"`
}

type LaporanHistory struct {
	ID        primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty" query:"id" url:"_id,omitempty" reqHeader:"_id"`
	Petugas   string             `json:"petugas,omitempty" bson:"petugas,omitempty"`
	NoPetugas string             `json:"nopetugas,omitempty" bson:"nopetugas,omitempty"`
	Kode      string             `json:"kode,omitempty" bson:"kode,omitempty"`
	Nama      string             `json:"nama,omitempty" bson:"nama,omitempty"`
	Phone     string             `json:"phone,omitempty" bson:"phone,omitempty"`
	Solusi    string             `json:"solusi,omitempty" bson:"solusi,omitempty"`
	Komentar  string             `json:"komentar,omitempty" bson:"komentar,omitempty"`
	Rating    float64            `json:"rating,omitempty" bson:"rating,omitempty"`
}

type LoginRequest struct {
	PhoneNumber string `json:"phonenumber"`
	Password    string `json:"password"`
}

type Stp struct {
	PhoneNumber  string    `bson:"phonenumber,omitempty" json:"phonenumber,omitempty"`
	PasswordHash string    `bson:"password,omitempty" json:"password,omitempty"`
	CreatedAt    time.Time `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
}

type VerifyRequest struct {
	PhoneNumber string `json:"phonenumber"`
	Password    string `json:"password"`
}

type Group struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	GroupName string             `json:"groupname"`
	Owner     string             `json:"owner"`
}

type Member struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	GroupID     Group              `bson:"groupid" json:"groupid"`
	PhoneNumber string             `bson:"phonenumber" json:"phonenumber"`
	Role        bool               `json:"role"`
}
