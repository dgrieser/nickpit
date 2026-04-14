package scm

import "time"

type PullLikeInfo struct {
	Title       string
	Description string
	BaseRef     string
	HeadRef     string
	URL         string
}

type Commit struct {
	SHA     string
	Message string
	Author  string
	Date    time.Time
}
