package models

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

type LeaderboardEntry struct {
	Rank     int    `json:"rank"`
	Username string `json:"username"`
	Rating   int    `json:"rating"`
}
