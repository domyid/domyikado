package model

import "time"

type BukpedBook struct {
	ID          string        `json:"_id" bson:"_id"`
	Secret      string        `json:"secret" bson:"secret"`
	Name        string        `json:"name" bson:"name"`
	Title       string        `json:"title" bson:"title"`
	Description string        `json:"description" bson:"description"`
	Owner       BukpedMember  `json:"owner" bson:"owner"`
	Editor      BukpedMember  `json:"editor" bson:"editor"`
	Manager     BukpedMember  `json:"manager" bson:"manager"`
	IsApproved  bool          `json:"isapproved" bson:"isapproved"`
	CoverBuku   string        `json:"coverbuku" bson:"coverbuku"`
	DraftBuku   string        `json:"draftbuku" bson:"draftbuku"`
	DraftPDFBuku string       `json:"draftpdfbuku" bson:"draftpdfbuku"`
	SampulPDFBuku string      `json:"sampulpdfbuku" bson:"sampulpdfbuku"`
	URLKatalog  string        `json:"urlkatalog" bson:"urlkatalog"`
	PathKatalog string        `json:"pathkatalog" bson:"pathkatalog"`
	SPI         string        `json:"spi" bson:"spi"`
	ISBN        string        `json:"isbn" bson:"isbn"`
	Terbit      string        `json:"terbit" bson:"terbit,omitempty"`
	Ukuran      string        `json:"ukuran" bson:"ukuran,omitempty"`
	JumlahHalaman string      `json:"jumlahhalaman" bson:"jumlahhalaman,omitempty"`
	Tebal       string        `json:"tebal" bson:"tebal,omitempty"`
	NoResiISBN  string        `json:"noresiisbn" bson:"noresiisbn,omitempty"`
	Members     []BukpedMember `json:"members" bson:"members"`
	CreatedAt   time.Time     `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at" bson:"updated_at"`
	Points      float64       `json:"points" bson:"points"` 
}

type BukpedMember struct {
	ID          string `json:"_id" bson:"_id"`
	Name        string `json:"name" bson:"name"`
	PhoneNumber string `json:"phonenumber" bson:"phonenumber"`
	Email       string `json:"email" bson:"email,omitempty"`
	NIK         string `json:"nik" bson:"nik,omitempty"`
	Pekerjaan   string `json:"pekerjaan" bson:"pekerjaan,omitempty"`
	AlamatRumah string `json:"alamatrumah" bson:"alamatrumah,omitempty"`
	AlamatKantor string `json:"alamatkantor" bson:"alamatkantor,omitempty"`
	Picture     string `json:"picture" bson:"picture,omitempty"`
	ProfPic     string `json:"profpic" bson:"profpic,omitempty"`
	Bio         string `json:"bio" bson:"bio,omitempty"`
	URLBio      string `json:"urlbio" bson:"urlbio,omitempty"`
	PathBio     string `json:"pathbio" bson:"pathbio,omitempty"`
	IsManager   bool   `json:"ismanager,omitempty" bson:"ismanager,omitempty"`
}

type BukpedUserInfo struct {
	PhoneNumber string  `json:"phone_number" bson:"phone_number"`
	Name        string  `json:"name" bson:"name"`
	TotalBooks  int     `json:"total_books" bson:"total_books"`
	TotalPoints int     `json:"total_points" bson:"total_points"`
}