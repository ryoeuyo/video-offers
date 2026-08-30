package video

// YouTubeThumbnailURL — публичный CDN-превью без oEmbed.
func YouTubeThumbnailURL(videoID string) string {
	if !youtubeIDRe.MatchString(videoID) {
		return ""
	}
	return "https://i.ytimg.com/vi/" + videoID + "/hqdefault.jpg"
}
