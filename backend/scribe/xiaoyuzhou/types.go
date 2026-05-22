// SPDX-License-Identifier: GPL-3.0-or-later
package xiaoyuzhou

// Episode mirrors the /v1/episode/get payload (subset we need).
type Episode struct {
	EID             string        `json:"eid"`
	PID             string        `json:"pid"`
	Title           string        `json:"title"`
	Description     string        `json:"description"`
	Duration        int           `json:"duration"` // seconds
	IsPrivateMedia  bool          `json:"isPrivateMedia"`
	Enclosure       Enclosure     `json:"enclosure"`
	Media           Media         `json:"media"`
	Image           Picture       `json:"image"`
	Podcast         PodcastBrief  `json:"podcast"`
}

type Enclosure struct {
	URL string `json:"url"`
}

type Media struct {
	ID       string     `json:"id"`
	Size     int64      `json:"size"`
	MimeType string     `json:"mimeType"`
	Source   MediaSource `json:"source"`
}

type MediaSource struct {
	URL string `json:"url"`
}

type Picture struct {
	PicURL       string `json:"picUrl"`
	MiddlePicURL string `json:"middlePicUrl"`
}

type PodcastBrief struct {
	PID    string `json:"pid"`
	Title  string `json:"title"`
	Author string `json:"author"`
}

type episodeGetResponse struct {
	Data Episode `json:"data"`
}

type privateMediaResponse struct {
	Data struct {
		URL string `json:"url"`
	} `json:"data"`
}

type episodeListResponse struct {
	Data []Episode `json:"data"`
}
