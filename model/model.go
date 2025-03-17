package model

type Response struct {
	Response string `json:"response"`
	Info     string `json:"info,omitempty"`
	Status   string `json:"status,omitempty"`
	Location string `json:"location,omitempty"`
}

type Profile struct {
	Token       string `bson:"token"`
	Phonenumber string `bson:"phonenumber"`
	Secret      string `bson:"secret"`
	URL         string `bson:"url"`
	QRKeyword   string `bson:"qrkeyword"`
	PublicKey   string `bson:"publickey"`
}

type LogInfo struct {
	PhoneNumber string `json:"phonenumber,omitempty" bson:"phonenumber,omitempty"`
	Alias       string `json:"alias,omitempty" bson:"alias,omitempty"`
	RepoOrg     string `json:"repoorg,omitempty" bson:"repoorg,omitempty"`
	RepoName    string `json:"reponame,omitempty" bson:"reponame,omitempty"`
	Commit      string `json:"commit,omitempty" bson:"commit,omitempty"`
	Remaining   int    `json:"remaining,omitempty" bson:"remaining,omitempty"`
	FileName    string `json:"filename,omitempty" bson:"filename,omitempty"`
	Base64Str   string `json:"base64str,omitempty" bson:"base64str,omitempty"`
	FileHash    string `json:"filehash,omitempty" bson:"filehash,omitempty"`
	Error       string `json:"error,omitempty" bson:"error,omitempty"`
}

type Config struct {
	PhoneNumber            string `json:"phonenumber,omitempty" bson:"phonenumber,omitempty"`
	LeaflyURL              string `json:"leaflyurl,omitempty" bson:"leaflyurl,omitempty"`
	LeaflyURLLMSDesaGambar string `json:"leaflyurllmsdesagambar,omitempty" bson:"leaflyurllmsdesagambar,omitempty"`
	LeaflyURLLMSDesaFile   string `json:"leaflyurllmsdesafile,omitempty" bson:"leaflyurllmsdesafile,omitempty"`
	LeaflySecret           string `json:"leaflysecret,omitempty" bson:"leaflysecret,omitempty"`
	DomyikadoPresensiURL   string `json:"domyikadopresensiurl,omitempty" bson:"domyikadopresensiurl,omitempty"`
	DomyikadoSecret        string `json:"domyikadosecret,omitempty" bson:"domyikadosecret,omitempty"`
	ApproveBimbinganURL    string `json:"approvebimbinganurl,omitempty" bson:"approvebimbinganurl,omitempty"`
	PomokitUrl             string `json:"pomokiturl,omitempty" bson:"pomokiturl,omitempty"`
	StravaUrl              string `json:"stravaurl,omitempty" bson:"stravaurl,omitempty"`
}
