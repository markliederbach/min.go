package client

import (
	"net/http"
	"time"
)

type RapidFixtureStatus string

const (
	RapidFixtureStatusNotStarted RapidFixtureStatus = "NS"
	RapidFixtureStatusKickoff    RapidFixtureStatus = "1H"
	RapidFixtureStatusHalftime   RapidFixtureStatus = "HT"
	RapidFixtureStatusSecond     RapidFixtureStatus = "2H"
	RapidFixtureStatusInProgress RapidFixtureStatus = "1H-HT-2H"
	RapidFixtureStatusFulltime   RapidFixtureStatus = "FT-AET-PEN"
	RapidFixtureStatusAllRecent  RapidFixtureStatus = "1H-HT-2H-FT-AET-PEN"
)

const (
	ThreadsContainerStatusExpired    = "EXPIRED"
	ThreadsContainerStatusError      = "ERROR"
	ThreadsContainerStatusFinished   = "FINISHED"
	ThreadsContainerStatusInProgress = "IN_PROGRESS"
	ThreadsContainerStatusPublished  = "PUBLISHED"
)

type RapidClientOptions struct {
	APIKey     string
	HostHeader string
	BaseUrl    string
	LeagueId   string
	TeamId     string
}

type RapidClientImpl struct {
	Options    RapidClientOptions
	HttpClient *http.Client
}

type GetTeamsResponse struct {
	Results  int         `json:"results"`
	Paging   Paging      `json:"paging"`
	Response []TeamVenue `json:"response"`
}

type GetFixturesResponse struct {
	Results  int           `json:"results"`
	Paging   Paging        `json:"paging"`
	Response []FixtureInfo `json:"response"`
}

type GetEventsResponse struct {
	Results  int         `json:"results"`
	Paging   Paging      `json:"paging"`
	Response []EventInfo `json:"response"`
}

type EventInfo struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Time      EventTime `gorm:"embedded;embeddedPrefix:time_" json:"time"`
	Team      Team      `gorm:"embedded;embeddedPrefix:team_" json:"team"`
	Player    Player    `gorm:"embedded;embeddedPrefix:player_" json:"player"`
	Assist    Player    `gorm:"embedded;embeddedPrefix:assist_" json:"assist"`
	FixtureID int
	Type      string `json:"type"`
	Detail    string `json:"detail"`
	Comments  string `json:"comments"`
}

type EventTime struct {
	Elapsed int `json:"elapsed"`
	Extra   int `json:"extra"`
}

type Player struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Paging struct {
	Current int `json:"current"`
	Total   int `json:"total"`
}

type TeamVenue struct {
	Team  Team  `json:"team"`
	Venue Venue `json:"venue"`
}

type Team struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Winner bool   `json:"winner"`
}

type Venue struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	City     string `json:"city"`
	Capacity int    `json:"capacity"`
	Surface  string `json:"surface"`
	Image    string `json:"image"`
}

type FixtureInfo struct {
	Fixture Fixture `gorm:"embedded" json:"fixture"`
	League  League  `gorm:"embedded;embeddedPrefix:league_" json:"league"`
	Teams   Teams   `gorm:"embedded;embeddedPrefix:teams_" json:"teams"`
	Goals   Goals   `gorm:"embedded;embeddedPrefix:goals_" json:"goals"`
	Score   Score   `gorm:"embedded;embeddedPrefix:score_" json:"score"`
}

type Fixture struct {
	ID        int       `json:"id"`
	Referee   string    `json:"referee"`
	Timezone  string    `json:"timezone"`
	Date      time.Time `json:"date"`
	Timestamp int       `json:"timestamp"`
	Periods   Periods   `gorm:"embedded;embeddedPrefix:periods_" json:"periods"`
	Venue     Venue     `gorm:"embedded;embeddedPrefix:venue_" json:"venue"`
	Status    Status    `gorm:"embedded;;embeddedPrefix:status_" json:"status"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type Periods struct {
	First  int `json:"first"`
	Second int `json:"second"`
}

type Status struct {
	Long    string `json:"long"`
	Short   string `json:"short"`
	Elapsed int    `json:"elapsed"`
}

type League struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Country string `json:"country"`
	Logo    string `json:"logo"`
	Flag    string `json:"flag"`
	Season  int    `json:"season"`
	Round   string `json:"round"`
}

type Teams struct {
	Home Team `gorm:"embedded;embeddedPrefix:home_" json:"home"`
	Away Team `gorm:"embedded;embeddedPrefix:away_" json:"away"`
}

type Goals struct {
	Home int `gorm:"embedded;embeddedPrefix:home_" json:"home"`
	Away int `gorm:"embedded;embeddedPrefix:away_" json:"away"`
}

type Score struct {
	Halftime  Goal `gorm:"embedded;embeddedPrefix:halftime_" json:"halftime"`
	Fulltime  Goal `gorm:"embedded;embeddedPrefix:fulltime_" json:"fulltime"`
	Extratime Goal `gorm:"embedded;embeddedPrefix:extratime_" json:"extratime"`
	Penalty   Goal `gorm:"embedded;embeddedPrefix:penalty_" json:"penalty"`
}

type Goal struct {
	Home int `json:"home"`
	Away int `json:"away"`
}

type ThreadsClientImpl struct {
	Options    ThreadsClientOptions
	HttpClient *http.Client
	userId     string
}

type ThreadsClientOptions struct {
	APIKey            string
	BaseUrl           string
	Username          string
	SkipUserIdPrefill bool
}

type GetUserIdResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type CreateTextPostContainerResponse struct {
	ID string `json:"id"`
}

type CreateTextPostResponse struct {
	ID string `json:"id"`
}

type GetMediaContainerStatusResponse struct {
	Status       string `json:"status"`
	ID           string `json:"id"`
	ErrorMessage string `json:"error_message"`
}

type PublishMediaContainerResponse struct {
	ID string `json:"id"`
}

type RefreshTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type DatabaseFixture struct {
	ID           int       `gorm:"primaryKey" json:"id"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	Date         time.Time `json:"date"`
	Timezone     string    `json:"timezone"`
	Status       string    `json:"status"`
	Elapsed      int       `json:"elapsed"`
	LeagueID     int       `json:"league_id"`
	LeagueName   string    `json:"league_name"`
	HomeTeamID   int       `json:"home_team_id"`
	AwayTeamID   int       `json:"away_team_id"`
	HomeTeamName string    `json:"home_team_name"`
	AwayTeamName string    `json:"away_team_name"`
	HomeScore    int       `json:"home_score"`
	AwayScore    int       `json:"away_score"`
	NotifyState  string    `json:"notify_state"`
}
