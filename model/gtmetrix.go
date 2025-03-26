package model

import "time"

type GTMetrixInfo struct {
	Name             string
	PhoneNumber      string
	Grade            string
	Points           float64
	WaGroupID        string
	CreatedAt        time.Time
	PerformanceScore string
	StructureScore   string
	LCP              string
	TBT              string
	CLS              string
}